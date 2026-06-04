// Package view parses and renders the first static subset of view {} markup.
package view

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	gowhtml "github.com/gowdk/gowdk/runtime/html"
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
	out.WriteByte('<')
	out.WriteString(node.Name)
	postRoute, err := node.postRoute(ctx)
	if err != nil {
		return err
	}
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			if attr.Name != "g:post" {
				return fmt.Errorf("unsupported directive attribute %q in static build", attr.Name)
			}
			continue
		}
		if postRoute != "" && (attr.Name == "method" || attr.Name == "action") {
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
	if postRoute != "" {
		out.WriteString(` method="post" action="`)
		out.WriteString(gowhtml.Escape(postRoute))
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

func (node Element) postRoute(ctx *renderContext) (string, error) {
	postAction, err := node.postActionName()
	if err != nil {
		return "", err
	}
	if postAction == "" {
		return "", nil
	}
	route, ok := ctx.actions[postAction]
	if !ok {
		return "", fmt.Errorf("unknown action %q for g:post", postAction)
	}
	return route, nil
}

func (node Element) postActionName() (string, error) {
	postAction := ""
	for _, attr := range node.Attrs {
		if attr.Name != "g:post" {
			continue
		}
		if node.Name != "form" {
			return "", fmt.Errorf("g:post is only supported on <form>")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", fmt.Errorf("g:post requires an action name")
		}
		if postAction != "" {
			return "", fmt.Errorf("form declares multiple g:post directives")
		}
		postAction = strings.TrimSpace(attr.Value)
	}
	if postAction == "" {
		return "", nil
	}
	return postAction, nil
}

// ComponentCall invokes a parsed component with static string props.
type ComponentCall struct {
	Name  string
	Attrs []Attr
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

	childCtx := renderContext{
		components: ctx.components,
		values:     values,
		actions:    ctx.actions,
		stack:      cloneStack(ctx.stack),
	}
	childCtx.stack[node.Name] = true
	body, err := render(component.Body, childCtx)
	if err != nil {
		return err
	}
	out.WriteString(body)
	return nil
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

func render(source string, ctx renderContext) (string, error) {
	nodes, err := Parse(source)
	if err != nil {
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

func collectActionFormFields(nodes []Node, fields map[string]map[string]ActionFormField) error {
	for _, node := range nodes {
		element, ok := node.(Element)
		if !ok {
			continue
		}
		action, err := element.postActionName()
		if err != nil {
			return err
		}
		if action != "" {
			if fields[action] == nil {
				fields[action] = map[string]ActionFormField{}
			}
			if err := collectNamedControls(element.Children, fields[action]); err != nil {
				return err
			}
			continue
		}
		if err := collectActionFormFields(element.Children, fields); err != nil {
			return err
		}
	}
	return nil
}

func collectNamedControls(nodes []Node, fields map[string]ActionFormField) error {
	for _, node := range nodes {
		element, ok := node.(Element)
		if !ok {
			continue
		}
		if field, ok, err := controlField(element); err != nil {
			return err
		} else if ok {
			previous := fields[field.Name]
			field.Required = field.Required || previous.Required
			fields[field.Name] = field
		}
		if err := collectNamedControls(element.Children, fields); err != nil {
			return err
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
	for _, attr := range element.Attrs {
		if attr.Name == "required" {
			field.Required = true
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
	return field, true, nil
}

type renderContext struct {
	components map[string]Component
	values     map[string]string
	actions    map[string]string
	stack      map[string]bool
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
			return Element{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
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
			return ComponentCall{}, parser.errorf("component <%s> must be self-closing in this build slice", name)
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
	if !parser.consume(`"`) {
		return Attr{}, parser.errorf("attribute %q must use a quoted string value", name)
	}
	start := parser.index
	for !parser.done() && parser.peek() != '"' {
		parser.advance()
	}
	if parser.done() {
		return Attr{}, parser.errorf("unterminated attribute %q", name)
	}
	value := string(parser.source[start:parser.index])
	parser.advance()
	return Attr{Name: name, Value: value}, nil
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
	if parser.consume(`"`) {
		start := parser.index
		for !parser.done() && parser.peek() != '"' {
			parser.advance()
		}
		if parser.done() {
			return Attr{}, parser.errorf("unterminated directive attribute %q", name)
		}
		value := strings.TrimSpace(string(parser.source[start:parser.index]))
		parser.advance()
		return Attr{Name: name, Value: value}, nil
	}
	return Attr{}, parser.errorf("directive attribute %q must use {name}", name)
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
		resolved, ok := ctx.values[name]
		if !ok {
			return "", fmt.Errorf("unknown interpolation %q", name)
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
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
