// Package view parses and renders the first static subset of view {} markup.
package view

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

// Node is a static view markup node.
type Node interface {
	render(*renderContext, *strings.Builder) error
}

// Text is escaped text content.
type Text struct {
	Value string
}

func (node Text) render(ctx *renderContext, out *strings.Builder) error {
	return renderText(ctx, out, node.Value)
}

// Element is a lowercase HTML element.
type Element struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

func (node Element) render(ctx *renderContext, out *strings.Builder) error {
	if node.Name == "slot" {
		if ctx.slotHTML != "" {
			out.WriteString(ctx.slotHTML)
			return nil
		}
		for _, child := range node.Children {
			if err := child.render(ctx, out); err != nil {
				return err
			}
		}
		return nil
	}
	out.WriteByte('<')
	out.WriteString(node.Name)
	directives, err := node.postDirectives(ctx)
	if err != nil {
		return err
	}
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if directives.Route != "" && (attr.Name == "method" || attr.Name == "action") {
			return fmt.Errorf("form with g:post must not declare %q", attr.Name)
		}
		value, err := interpolate(ctx, attr.Value)
		if err != nil {
			return err
		}
		out.WriteByte(' ')
		out.WriteString(attr.Name)
		if attr.Value != "" || !attr.Boolean {
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(value))
			out.WriteByte('"')
		}
	}
	if directives.Route != "" {
		out.WriteString(` method="post" action="`)
		out.WriteString(gowhtml.Escape(directives.Route))
		out.WriteByte('"')
	}
	if directives.Target != "" {
		out.WriteString(` data-gowdk-target="`)
		out.WriteString(gowhtml.Escape(directives.Target))
		out.WriteByte('"')
	}
	if directives.Swap != "" {
		out.WriteString(` data-gowdk-swap="`)
		out.WriteString(gowhtml.Escape(directives.Swap))
		out.WriteByte('"')
	}
	out.WriteByte('>')
	for _, child := range node.Children {
		if err := child.render(ctx, out); err != nil {
			return err
		}
	}
	out.WriteString("</")
	out.WriteString(node.Name)
	out.WriteByte('>')
	return nil
}

type postDirectives struct {
	Action string
	Route  string
	Target string
	Swap   string
}

func (node Element) postDirectives(ctx *renderContext) (postDirectives, error) {
	directives, err := node.directiveValues()
	if err != nil {
		return postDirectives{}, err
	}
	if directives.Action == "" {
		if directives.Target != "" || directives.Swap != "" {
			return postDirectives{}, fmt.Errorf("g:target and g:swap require g:post")
		}
		return postDirectives{}, nil
	}
	route, ok := ctx.actions[directives.Action]
	if !ok {
		return postDirectives{}, fmt.Errorf("unknown action %q for g:post", directives.Action)
	}
	directives.Route = route
	return directives, nil
}

func (node Element) postActionName() (string, error) {
	directives, err := node.directiveValues()
	if err != nil {
		return "", err
	}
	return directives.Action, nil
}

func (node Element) directiveValues() (postDirectives, error) {
	var directives postDirectives
	for _, attr := range node.Attrs {
		if !strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Name != "g:post" && attr.Name != "g:target" && attr.Name != "g:swap" {
			return postDirectives{}, fmt.Errorf("unsupported directive attribute %q in static build", attr.Name)
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return postDirectives{}, fmt.Errorf("%s requires a value", attr.Name)
		}
		if node.Name != "form" {
			return postDirectives{}, fmt.Errorf("%s is only supported on <form>", attr.Name)
		}
		switch attr.Name {
		case "g:post":
			if directives.Action != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:post directives")
			}
			directives.Action = strings.TrimSpace(attr.Value)
		case "g:target":
			if directives.Target != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:target directives")
			}
			target := strings.TrimSpace(attr.Value)
			if strings.ContainsAny(target, "{}") {
				return postDirectives{}, fmt.Errorf("g:target %q must be static", target)
			}
			if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n") {
				return postDirectives{}, fmt.Errorf("g:target %q must be a static id selector", target)
			}
			directives.Target = target
		case "g:swap":
			if directives.Swap != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:swap directives")
			}
			swap := strings.TrimSpace(attr.Value)
			if !isSupportedSwapMode(swap) {
				return postDirectives{}, fmt.Errorf("unsupported g:swap mode %q", swap)
			}
			directives.Swap = swap
		}
	}
	if directives.Swap != "" && directives.Target == "" {
		return postDirectives{}, fmt.Errorf("g:swap requires g:target")
	}
	return directives, nil
}

func isSupportedSwapMode(value string) bool {
	switch value {
	case "innerHTML", "outerHTML":
		return true
	default:
		return false
	}
}

// ComponentCall invokes a parsed component with static string props.
type ComponentCall struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

func (node ComponentCall) render(ctx *renderContext, out *strings.Builder) error {
	component, ok := ctx.components[node.Name]
	if !ok {
		return fmt.Errorf("missing component %q", node.Name)
	}
	if ctx.stack[node.Name] {
		return fmt.Errorf("recursive component %q", node.Name)
	}

	values := map[string]string{}
	for _, attr := range node.Attrs {
		if attr.Boolean {
			return fmt.Errorf("component %s prop %q requires a string value", node.Name, attr.Name)
		}
		value, err := interpolate(ctx, attr.Value)
		if err != nil {
			return err
		}
		values[attr.Name] = value
	}
	for _, prop := range component.Props {
		if _, ok := values[prop]; !ok {
			return fmt.Errorf("component %s missing required prop %q", node.Name, prop)
		}
	}
	for prop := range values {
		if !component.HasProp(prop) {
			return fmt.Errorf("component %s does not declare prop %q", node.Name, prop)
		}
	}
	slotHTML, err := renderNodes(node.Children, ctx)
	if err != nil {
		return err
	}

	childCtx := renderContext{
		components: ctx.components,
		values:     values,
		actions:    ctx.actions,
		stack:      cloneStack(ctx.stack),
		slotHTML:   slotHTML,
	}
	childCtx.stack[node.Name] = true
	body, err := render(component.Body, childCtx)
	if err != nil {
		return err
	}
	out.WriteString(body)
	return nil
}

func renderNodes(nodes []Node, ctx *renderContext) (string, error) {
	if len(nodes) == 0 {
		return "", nil
	}
	var out strings.Builder
	for _, node := range nodes {
		if err := node.render(ctx, &out); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

// Attr is a static HTML attribute.
type Attr struct {
	Name    string
	Value   string
	Boolean bool
}

// Component is a static component template known to the view renderer.
type Component struct {
	Name  string
	Props []string
	Body  string
}

// HasProp reports whether a component declares a prop.
func (component Component) HasProp(name string) bool {
	for _, prop := range component.Props {
		if prop == name {
			return true
		}
	}
	return false
}

// Parse parses a static markup fragment.
func Parse(source string) ([]Node, error) {
	parser := parser{source: []rune(source)}
	nodes, err := parser.nodes("")
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if !parser.done() {
		return nil, parser.errorf("unexpected content")
	}
	return nodes, nil
}

// RenderStatic renders a static markup fragment with escaped text and attrs.
func RenderStatic(source string) (string, error) {
	return RenderWithComponents(source, nil)
}

// RenderWithComponents renders a static markup fragment with component support.
func RenderWithComponents(source string, components map[string]Component) (string, error) {
	return RenderWithData(source, components, nil)
}

// RenderWithData renders a static markup fragment with component support and
// string interpolation data.
func RenderWithData(source string, components map[string]Component, data map[string]string) (string, error) {
	return RenderWithOptions(source, components, data, Options{})
}

// Options configures static view rendering.
type Options struct {
	Actions map[string]string
}

// ActionFormField describes one direct static form field for a g:post action.
type ActionFormField struct {
	Name     string
	Required bool
}

// Dependencies records static dependencies visible in the first view subset.
type Dependencies struct {
	StaticAssets    []string
	CSSClasses      []string
	StyleAttributes []string
}

// RenderWithOptions renders a static markup fragment with component support,
// interpolation data, and page-scoped action routes.
func RenderWithOptions(source string, components map[string]Component, data map[string]string, options Options) (string, error) {
	return render(source, renderContext{
		components: components,
		values:     cloneValues(data),
		actions:    cloneValues(options.Actions),
		stack:      map[string]bool{},
	})
}

// ActionFormFields returns direct static HTML control names grouped by g:post
// action name. Component-hidden controls are intentionally not inferred in this
// first decoder slice.
func ActionFormFields(source string) (map[string][]string, error) {
	schema, err := ActionFormSchema(source)
	if err != nil {
		return nil, err
	}
	fields := map[string][]string{}
	for action, actionFields := range schema {
		for _, field := range actionFields {
			fields[action] = append(fields[action], field.Name)
		}
	}
	return fields, nil
}

// ViewDependencies returns direct static asset and style references from a
// static markup fragment. Interpolated and external URLs are not reported.
func ViewDependencies(source string) (Dependencies, error) {
	nodes, err := Parse(source)
	if err != nil {
		return Dependencies{}, err
	}
	assets := map[string]bool{}
	classes := map[string]bool{}
	styles := map[string]bool{}
	collectViewDependencies(nodes, assets, classes, styles)
	return Dependencies{
		StaticAssets:    sortedKeys(assets),
		CSSClasses:      sortedKeys(classes),
		StyleAttributes: sortedKeys(styles),
	}, nil
}

// ActionFormSchema returns direct static HTML controls grouped by g:post action
// name. Duplicate field names are merged, and Required is true if any matching
// direct control is required.
func ActionFormSchema(source string) (map[string][]ActionFormField, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	fields := map[string]map[string]ActionFormField{}
	if err := collectActionFormFields(nodes, fields); err != nil {
		return nil, err
	}
	schema := map[string][]ActionFormField{}
	for action := range fields {
		names := make([]string, 0, len(fields[action]))
		for name := range fields[action] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			schema[action] = append(schema[action], fields[action][name])
		}
	}
	return schema, nil
}

// ComponentReferences returns unique component names directly referenced by a
// static markup fragment.
func ComponentReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	collectComponentReferences(nodes, names)
	if len(names) == 0 {
		return nil, nil
	}
	refs := make([]string, 0, len(names))
	for name := range names {
		refs = append(refs, name)
	}
	sort.Strings(refs)
	return refs, nil
}

// ParamReferences returns unique param("name") route-param references directly
// visible in the current static markup subset.
func ParamReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	collectParamReferences(nodes, names)
	return sortedKeys(names), nil
}

func render(source string, ctx renderContext) (string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return "", err
	}
	if err := validateFragmentTargetReferences(nodes); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, node := range nodes {
		if err := node.render(&ctx, &out); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func validateFragmentTargetReferences(nodes []Node) error {
	ids := map[string]bool{}
	targets := map[string]bool{}
	collectIDsAndTargets(nodes, ids, targets)
	for target := range targets {
		id := strings.TrimPrefix(target, "#")
		if !ids[id] {
			return fmt.Errorf("g:target %q does not reference a static id in this view", target)
		}
	}
	return nil
}

func collectIDsAndTargets(nodes []Node, ids map[string]bool, targets map[string]bool) {
	for _, node := range nodes {
		element, ok := node.(Element)
		if !ok {
			continue
		}
		hasPost := false
		for _, attr := range element.Attrs {
			if attr.Name == "g:post" {
				hasPost = true
				break
			}
		}
		for _, attr := range element.Attrs {
			if attr.Boolean {
				continue
			}
			switch attr.Name {
			case "id":
				id := strings.TrimSpace(attr.Value)
				if id != "" && !strings.ContainsAny(id, "{}") {
					ids[id] = true
				}
			case "g:target":
				target := strings.TrimSpace(attr.Value)
				if hasPost && target != "" && !strings.ContainsAny(target, "{}") {
					targets[target] = true
				}
			}
		}
		collectIDsAndTargets(element.Children, ids, targets)
	}
}

func collectParamReferences(nodes []Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Text:
			collectParamReferencesFromString(typed.Value, names)
		case Element:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		case ComponentCall:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		}
	}
}

func collectParamReferencesFromString(value string, names map[string]bool) {
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			return
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if name, ok := routeParamExpression(expr); ok {
			names[name] = true
		}
		value = value[end+1:]
	}
}

func collectComponentReferences(nodes []Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			names[typed.Name] = true
			collectComponentReferences(typed.Children, names)
		case Element:
			collectComponentReferences(typed.Children, names)
		}
	}
}

func collectViewDependencies(nodes []Node, assets, classes, styles map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			for _, attr := range typed.Attrs {
				switch attr.Name {
				case "class":
					for _, className := range strings.Fields(attr.Value) {
						if !strings.ContainsAny(className, "{}") {
							classes[className] = true
						}
					}
				case "style":
					style := strings.TrimSpace(attr.Value)
					if style != "" && !strings.ContainsAny(style, "{}") {
						styles[style] = true
					}
				case "src", "href", "poster":
					if isStaticAssetReference(attr.Value) {
						assets[strings.TrimSpace(attr.Value)] = true
					}
				}
			}
			collectViewDependencies(typed.Children, assets, classes, styles)
		case ComponentCall:
			collectViewDependencies(typed.Children, assets, classes, styles)
		}
	}
}

func collectActionFormFields(nodes []Node, fields map[string]map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			action, err := typed.postActionName()
			if err != nil {
				return err
			}
			if action != "" {
				if fields[action] == nil {
					fields[action] = map[string]ActionFormField{}
				}
				if err := validateActionForm(typed); err != nil {
					return err
				}
				if err := collectNamedControls(typed.Children, fields[action]); err != nil {
					return err
				}
				continue
			}
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateActionForm(element Element) error {
	for _, attr := range element.Attrs {
		if attr.Name != "enctype" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		value := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(value, "{}") {
			return fmt.Errorf("action form enctype %q must be static", value)
		}
		if strings.EqualFold(value, "multipart/form-data") {
			return fmt.Errorf("multipart action forms are not supported before upload security rules are defined")
		}
	}
	return nil
}

func collectNamedControls(nodes []Node, fields map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			if field, ok, err := controlField(typed); err != nil {
				return err
			} else if ok {
				previous := fields[field.Name]
				field.Required = field.Required || previous.Required
				fields[field.Name] = field
			}
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func controlField(element Element) (ActionFormField, bool, error) {
	switch element.Name {
	case "input", "textarea", "select":
	default:
		return ActionFormField{}, false, nil
	}
	var field ActionFormField
	inputType := ""
	for _, attr := range element.Attrs {
		if attr.Name == "required" {
			field.Required = true
			continue
		}
		if element.Name == "input" && attr.Name == "type" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				continue
			}
			inputType = strings.TrimSpace(attr.Value)
			continue
		}
		if attr.Name != "name" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		name := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(name, "{}") {
			return ActionFormField{}, false, fmt.Errorf("action form field name %q must be static", name)
		}
		field.Name = name
	}
	if field.Name == "" {
		return ActionFormField{}, false, nil
	}
	if strings.ContainsAny(inputType, "{}") {
		return ActionFormField{}, false, fmt.Errorf("action form input %q type %q must be static", field.Name, inputType)
	}
	if strings.EqualFold(inputType, "file") {
		return ActionFormField{}, false, fmt.Errorf("file input %q is not supported before upload security rules are defined", field.Name)
	}
	return field, true, nil
}

func sortedKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isStaticAssetReference(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "{}") || strings.HasPrefix(value, "#") {
		return false
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"http://", "https://", "//", "mailto:", "tel:", "data:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

type renderContext struct {
	components map[string]Component
	values     map[string]string
	actions    map[string]string
	stack      map[string]bool
	slotHTML   string
}

type parser struct {
	source []rune
	index  int
}

func (parser *parser) nodes(until string) ([]Node, error) {
	var nodes []Node
	for {
		if parser.done() {
			if until != "" {
				return nil, parser.errorf("missing closing tag </%s>", until)
			}
			return nodes, nil
		}
		if until != "" && parser.startsWith("</") {
			name, err := parser.closeTag()
			if err != nil {
				return nil, err
			}
			if name != until {
				return nil, parser.errorf("expected closing tag </%s>, got </%s>", until, name)
			}
			return nodes, nil
		}
		if parser.startsWith("</") {
			return nil, parser.errorf("unexpected closing tag")
		}
		if parser.peek() == '<' {
			node, err := parser.element()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}
		if text := parser.text(); strings.TrimSpace(text) != "" {
			nodes = append(nodes, Text{Value: text})
		}
	}
}

func (parser *parser) element() (Node, error) {
	if !parser.consume("<") {
		return nil, parser.errorf("expected element")
	}
	name, err := parser.name()
	if err != nil {
		return nil, err
	}
	if isComponentName(name) {
		return parser.componentCall(name)
	}
	if !isLowerHTMLName(name) {
		return nil, parser.errorf("unsupported element <%s>; this build slice supports lowercase HTML tags only", name)
	}

	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			return Element{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			children, err := parser.nodes(name)
			if err != nil {
				return nil, err
			}
			return Element{Name: name, Attrs: attrs, Children: children}, nil
		case parser.done():
			return nil, parser.errorf("unterminated <%s> tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) componentCall(name string) (ComponentCall, error) {
	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			return ComponentCall{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
			children, err := parser.nodes(name)
			if err != nil {
				return ComponentCall{}, err
			}
			return ComponentCall{Name: name, Attrs: attrs, Children: children}, nil
		case parser.done():
			return ComponentCall{}, parser.errorf("unterminated <%s> component tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return ComponentCall{}, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) attr() (Attr, error) {
	if attr, ok, err := parser.shorthandAttr(); ok || err != nil {
		return attr, err
	}
	name, err := parser.name()
	if err != nil {
		return Attr{}, err
	}
	if !isAttrName(name) {
		return Attr{}, parser.errorf("unsupported attribute name %q", name)
	}

	parser.skipSpace()
	if !parser.consume("=") {
		return Attr{Name: name, Boolean: true}, nil
	}
	parser.skipSpace()
	if strings.HasPrefix(name, "g:") {
		return parser.directiveAttr(name)
	}
	if parser.startsWith("{") {
		value, err := parser.expressionAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value}, nil
	}
	value, err := parser.quotedAttrValue(name)
	if err != nil {
		return Attr{}, err
	}
	return Attr{Name: name, Value: value}, nil
}

func (parser *parser) expressionAttrValue(name string) (string, error) {
	if !parser.consume("{") {
		return "", parser.errorf("attribute %q must use an expression value", name)
	}
	start := parser.index
	for !parser.done() && parser.peek() != '}' {
		parser.advance()
	}
	if parser.done() {
		return "", parser.errorf("unterminated expression attribute %q", name)
	}
	expr := strings.TrimSpace(string(parser.source[start:parser.index]))
	if expr == "" {
		return "", parser.errorf("empty expression attribute %q", name)
	}
	parser.advance()
	return "{" + expr + "}", nil
}

func (parser *parser) shorthandAttr() (Attr, bool, error) {
	if parser.done() {
		return Attr{}, false, nil
	}
	prefix := parser.peek()
	if prefix != '.' && prefix != '#' {
		return Attr{}, false, nil
	}
	parser.advance()
	start := parser.index
	for !parser.done() && isShorthandPart(parser.peek()) {
		parser.advance()
	}
	if start == parser.index {
		return Attr{}, true, parser.errorf("empty shorthand attribute")
	}
	value := string(parser.source[start:parser.index])
	switch prefix {
	case '.':
		return Attr{Name: "class", Value: value}, true, nil
	case '#':
		return Attr{Name: "id", Value: value}, true, nil
	default:
		return Attr{}, false, nil
	}
}

func normalizeHTMLAttrs(attrs []Attr) ([]Attr, error) {
	var classValues []string
	var out []Attr
	id := ""
	for _, attr := range attrs {
		switch attr.Name {
		case "class":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			for _, className := range strings.Fields(attr.Value) {
				classValues = append(classValues, className)
			}
		case "id":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			if id != "" {
				return nil, fmt.Errorf("element declares multiple id attributes")
			}
			id = attr.Value
			out = append(out, attr)
		default:
			out = append(out, attr)
		}
	}
	if len(classValues) > 0 {
		out = append([]Attr{{Name: "class", Value: strings.Join(classValues, " ")}}, out...)
	}
	return out, nil
}

func (parser *parser) directiveAttr(name string) (Attr, error) {
	if parser.consume("{") {
		start := parser.index
		for !parser.done() && parser.peek() != '}' {
			parser.advance()
		}
		if parser.done() {
			return Attr{}, parser.errorf("unterminated directive attribute %q", name)
		}
		value := strings.TrimSpace(string(parser.source[start:parser.index]))
		parser.advance()
		return Attr{Name: name, Value: value}, nil
	}
	if parser.startsWith(`"`) {
		value, err := parser.quotedAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		value = strings.TrimSpace(value)
		return Attr{Name: name, Value: value}, nil
	}
	return Attr{}, parser.errorf("directive attribute %q must use {name}", name)
}

func (parser *parser) quotedAttrValue(name string) (string, error) {
	if !parser.consume(`"`) {
		return "", parser.errorf("attribute %q must use a quoted string value", name)
	}
	start := parser.index - 1
	escaped := false
	for !parser.done() {
		switch parser.peek() {
		case '\\':
			escaped = !escaped
			parser.advance()
		case '"':
			if escaped {
				escaped = false
				parser.advance()
				continue
			}
			parser.advance()
			value, err := strconv.Unquote(string(parser.source[start:parser.index]))
			if err != nil {
				return "", parser.errorf("invalid attribute %q string: %v", name, err)
			}
			return value, nil
		default:
			escaped = false
			parser.advance()
		}
	}
	return "", parser.errorf("unterminated attribute %q", name)
}

func (parser *parser) closeTag() (string, error) {
	if !parser.consume("</") {
		return "", parser.errorf("expected closing tag")
	}
	name, err := parser.name()
	if err != nil {
		return "", err
	}
	parser.skipSpace()
	if !parser.consume(">") {
		return "", parser.errorf("expected > after closing tag")
	}
	return name, nil
}

func (parser *parser) name() (string, error) {
	if parser.done() || !isNameStart(parser.peek()) {
		return "", parser.errorf("expected name")
	}
	start := parser.index
	parser.advance()
	for !parser.done() && isNamePart(parser.peek()) {
		parser.advance()
	}
	return string(parser.source[start:parser.index]), nil
}

func (parser *parser) text() string {
	start := parser.index
	for !parser.done() && parser.peek() != '<' {
		parser.advance()
	}
	return string(parser.source[start:parser.index])
}

func (parser *parser) skipSpace() {
	for !parser.done() && unicode.IsSpace(parser.peek()) {
		parser.advance()
	}
}

func (parser *parser) consume(value string) bool {
	if !parser.startsWith(value) {
		return false
	}
	parser.index += len([]rune(value))
	return true
}

func (parser *parser) startsWith(value string) bool {
	runes := []rune(value)
	if parser.index+len(runes) > len(parser.source) {
		return false
	}
	for offset, r := range runes {
		if parser.source[parser.index+offset] != r {
			return false
		}
	}
	return true
}

func (parser *parser) done() bool {
	return parser.index >= len(parser.source)
}

func (parser *parser) peek() rune {
	return parser.source[parser.index]
}

func (parser *parser) advance() {
	parser.index++
}

func (parser *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("view parse error at offset %d: %s", parser.index, fmt.Sprintf(format, args...))
}

func isLowerHTMLName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		case i > 0 && r == '-':
		default:
			return false
		}
	}
	return true
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}

func isAttrName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case isNameStart(r):
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == ':' || r == '_'):
		default:
			return false
		}
	}
	return true
}

func isNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isNamePart(r rune) bool {
	return isNameStart(r) || unicode.IsDigit(r) || r == '-' || r == ':'
}

func isShorthandPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':'
}

func renderText(ctx *renderContext, out *strings.Builder, value string) error {
	text, err := interpolate(ctx, value)
	if err != nil {
		return err
	}
	out.WriteString(gowhtml.Escape(text))
	return nil
}

func interpolate(ctx *renderContext, value string) (string, error) {
	if !strings.Contains(value, "{") {
		return value, nil
	}
	var out strings.Builder
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			out.WriteString(value)
			return out.String(), nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated interpolation")
		}
		end += start
		out.WriteString(value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if name == "" {
			return "", fmt.Errorf("empty interpolation")
		}
		if param, ok := routeParamExpression(name); ok {
			resolved, ok := ctx.values[param]
			if !ok {
				return "", fmt.Errorf("unknown route param %q", param)
			}
			out.WriteString(resolved)
			value = value[end+1:]
			continue
		}
		resolved, ok := ctx.values[name]
		if !ok {
			return "", fmt.Errorf("unknown interpolation %q", name)
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
}

func routeParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !isIdentifier(name) {
		return "", false
	}
	return name, true
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char == '_':
		case index > 0 && char >= '0' && char <= '9':
		default:
			return false
		}
	}
	return true
}

func cloneStack(input map[string]bool) map[string]bool {
	output := map[string]bool{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneValues(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}
