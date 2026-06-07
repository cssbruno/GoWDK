package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"sort"
	"strconv"
	"strings"
)

// Attr is a literal HTML attribute.
type Attr struct {
	Name       string
	Value      string
	Boolean    bool
	Expression bool
	Start      int
	End        int
}

// Component is a literal component template known to the view renderer.
type Component struct {
	Name         string
	Package      string
	Uses         map[string]string
	ScopeIDs     []string
	Props        []string
	State        map[string]string
	StateJSON    string
	Handlers     map[string]clientlang.Handler
	HandlersJSON string
	StateTypes   map[string]clientlang.ValueType
	Refs         map[string]clientlang.Ref
	Emits        map[string]clientlang.Emit
	Computed     []clientlang.Computed
	Body         string
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

// Parse parses a view markup fragment.
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

// RenderSPA renders a view markup fragment with escaped text and attrs.
func RenderSPA(source string) (string, error) {
	return RenderWithComponents(source, nil)
}

// RenderWithComponents renders a view markup fragment with component support.
func RenderWithComponents(source string, components map[string]Component) (string, error) {
	return RenderWithData(source, components, nil)
}

// RenderWithData renders a view markup fragment with component support and
// string interpolation data.
func RenderWithData(source string, components map[string]Component, data map[string]string) (string, error) {
	return RenderWithOptions(source, components, data, Options{})
}

// Options configures view rendering.
type Options struct {
	Actions map[string]string
	Package string
	Uses    map[string]string
}

// ActionFormField describes one direct literal form field for a g:post action.
type ActionFormField struct {
	Name             string
	Required         bool
	RequiredMessage  string
	MinLength        int
	MinLengthMessage string
	MaxLength        int
	MaxLengthMessage string
	Pattern          string
	PatternMessage   string
}

// Dependencies records source dependencies visible in the first view subset.
type Dependencies struct {
	Assets          []string
	CSSClasses      []string
	StyleAttributes []string
}

// ComponentIslandUsage records one component call that explicitly selects an
// island runtime.
type ComponentIslandUsage struct {
	Component string
	Mode      string
}

// ComponentCallUsage records one component call and its optional island mode.
type ComponentCallUsage struct {
	Component     string
	Island        string
	ReactiveProps bool
}

// ContractReference records one template-local backend contract intent.
type ContractReference struct {
	Kind  ContractReferenceKind
	Name  string
	Start int
	End   int
}

type ContractReferenceKind string

const (
	ContractReferenceCommand ContractReferenceKind = "command"
	ContractReferenceQuery   ContractReferenceKind = "query"
)

// CommandReference records one form-local backend command intent.
type CommandReference struct {
	Command string
	Start   int
	End     int
}

// QueryReference records one template-local backend query intent.
type QueryReference struct {
	Query string
	Start int
	End   int
}

// RenderWithOptions renders a view markup fragment with component support,
// interpolation data, and page-scoped action endpoints.
func RenderWithOptions(source string, components map[string]Component, data map[string]string, options Options) (string, error) {
	bindingSeq := 0
	islandSeq := 0
	return render(source, renderContext{
		components:   components,
		ownerPackage: options.Package,
		uses:         cloneValues(options.Uses),
		values:       cloneValues(data),
		actions:      cloneValues(options.Actions),
		stack:        map[string]bool{},
		stateFields:  map[string]bool{},
		readFields:   map[string]bool{},
		bindFields:   map[string]bool{},
		bindingSeq:   &bindingSeq,
		islandSeq:    &islandSeq,
	})
}

// ActionFormFields returns direct literal HTML control names grouped by g:post
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

// ViewDependencies returns direct literal asset and style references from a
// view markup fragment. Interpolated and external URLs are not reported.
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
		Assets:          sortedKeys(assets),
		CSSClasses:      sortedKeys(classes),
		StyleAttributes: sortedKeys(styles),
	}, nil
}

// ActionFormSchema returns direct literal HTML controls grouped by g:post action
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
// view markup fragment.
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

// ComponentIslandUsages returns component calls that explicitly set g:island.
func ComponentIslandUsages(source string) ([]ComponentIslandUsage, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var usages []ComponentIslandUsage
	if err := collectComponentIslandUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

// ComponentCallUsages returns component calls with optional g:island metadata.
func ComponentCallUsages(source string) ([]ComponentCallUsage, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var usages []ComponentCallUsage
	if err := collectComponentCallUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

// CommandReferences returns package-qualified command references declared by
// g:command on direct form elements in a view fragment.
func CommandReferences(source string) ([]CommandReference, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var refs []CommandReference
	if err := collectCommandReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// QueryReferences returns package-qualified query references declared by
// g:query on direct HTML elements in a view fragment.
func QueryReferences(source string) ([]QueryReference, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var refs []QueryReference
	if err := collectQueryReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// ContractReferences returns package-qualified command and query references
// declared by GOWDK view directives.
func ContractReferences(source string) ([]ContractReference, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var refs []ContractReference
	if err := collectContractReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// Canonical returns a deterministic AST-backed representation of a view body.
func Canonical(source string) (string, error) {
	nodes, err := Parse(stripLineComments(source))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	writeCanonicalNodes(&out, nodes)
	return out.String(), nil
}

func writeCanonicalNodes(out *strings.Builder, nodes []Node) {
	for _, node := range nodes {
		writeCanonicalNode(out, node)
	}
}

func writeCanonicalNode(out *strings.Builder, node Node) {
	switch typed := node.(type) {
	case Text:
		out.WriteString("text(")
		out.WriteString(strconv.Quote(strings.Join(strings.Fields(typed.Value), " ")))
		out.WriteByte(')')
	case Element:
		out.WriteString("element(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	case ComponentCall:
		out.WriteString("component(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	}
}

func writeCanonicalAttrs(out *strings.Builder, attrs []Attr) {
	normalized := make([]Attr, 0, len(attrs))
	for _, attr := range attrs {
		value := strings.TrimSpace(attr.Value)
		if attr.Name == "class" {
			classes := strings.Fields(value)
			sort.Strings(classes)
			value = strings.Join(classes, " ")
		}
		value = canonicalAttrValue(attr.Name, value, attr.Expression)
		normalized = append(normalized, Attr{Name: attr.Name, Value: value, Boolean: attr.Boolean, Expression: attr.Expression})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		if normalized[i].Value != normalized[j].Value {
			return normalized[i].Value < normalized[j].Value
		}
		return !normalized[i].Boolean && normalized[j].Boolean
	})
	out.WriteByte('{')
	for index, attr := range normalized {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(attr.Name)
		if attr.Boolean {
			out.WriteString(":bool")
			continue
		}
		if attr.Expression {
			out.WriteString(":expr")
		}
		out.WriteByte('=')
		out.WriteString(strconv.Quote(attr.Value))
	}
	out.WriteByte('}')
}

func canonicalAttrValue(name string, value string, expression bool) string {
	if strings.HasPrefix(name, "g:on:") {
		return clientlang.CanonicalStatement(value)
	}
	if expression || name == "g:if" || name == "g:else-if" || name == "g:key" ||
		strings.HasPrefix(name, "class:") || strings.HasPrefix(name, "style:") {
		expr := expressionAttrSource(value)
		if canonical, err := clientlang.CanonicalExpr(expr); err == nil {
			return canonical
		}
		return strings.Join(strings.Fields(expr), " ")
	}
	return value
}

// ParamReferences returns unique param("name") route-param references directly
// visible in the current view markup subset.
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
	if ctx.loopSeq == nil {
		seq := 0
		ctx.loopSeq = &seq
	}
	if ctx.bindingSeq == nil {
		seq := 0
		ctx.bindingSeq = &seq
	}
	if ctx.islandSeq == nil {
		seq := 0
		ctx.islandSeq = &seq
	}
	return renderNodes(nodes, &ctx)
}

func validateFragmentTargetReferences(nodes []Node) error {
	ids := map[string]bool{}
	targets := map[string]bool{}
	collectIDsAndTargets(nodes, ids, targets)
	for target := range targets {
		id := strings.TrimPrefix(target, "#")
		if !ids[id] {
			return fmt.Errorf("g:target %q does not reference a literal id in this view", target)
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

func collectComponentIslandUsages(nodes []Node, usages *[]ComponentIslandUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			if mode != "" {
				*usages = append(*usages, ComponentIslandUsage{Component: typed.Name, Mode: mode})
			}
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectComponentCallUsages(nodes []Node, usages *[]ComponentCallUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			*usages = append(*usages, ComponentCallUsage{
				Component:     typed.Name,
				Island:        mode,
				ReactiveProps: typed.hasReactiveProps(),
			})
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func (node ComponentCall) hasReactiveProps() bool {
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Expression {
			return true
		}
	}
	return false
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
					if isSPAAssetReference(attr.Value) {
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

func collectCommandReferences(nodes []Node, refs *[]CommandReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			directives, err := typed.directiveValues()
			if err != nil {
				return err
			}
			if directives.Command != "" {
				*refs = append(*refs, CommandReference{Command: directives.Command, Start: directives.CommandStart, End: directives.CommandEnd})
			}
			if err := collectCommandReferences(typed.Children, refs); err != nil {
				return err
			}
		case ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == "g:event" {
					return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", typed.Name)
				}
			}
			if err := collectCommandReferences(typed.Children, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectQueryReferences(nodes []Node, refs *[]QueryReference) error {
	contracts, err := contractReferencesFromNodes(nodes)
	if err != nil {
		return err
	}
	for _, ref := range contracts {
		if ref.Kind == ContractReferenceQuery {
			*refs = append(*refs, QueryReference{Query: ref.Name, Start: ref.Start, End: ref.End})
		}
	}
	return nil
}

func collectContractReferences(nodes []Node, refs *[]ContractReference) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			directives, err := typed.directiveValues()
			if err != nil {
				return err
			}
			if directives.Command != "" {
				*refs = append(*refs, ContractReference{
					Kind:  ContractReferenceCommand,
					Name:  directives.Command,
					Start: directives.CommandStart,
					End:   directives.CommandEnd,
				})
			}
			if directives.Query != "" {
				*refs = append(*refs, ContractReference{
					Kind:  ContractReferenceQuery,
					Name:  directives.Query,
					Start: directives.QueryStart,
					End:   directives.QueryEnd,
				})
			}
			if err := collectContractReferences(typed.Children, refs); err != nil {
				return err
			}
		case ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == "g:event" {
					return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", typed.Name)
				}
			}
			if err := collectContractReferences(typed.Children, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

func contractReferencesFromNodes(nodes []Node) ([]ContractReference, error) {
	var refs []ContractReference
	if err := collectContractReferences(nodes, &refs); err != nil {
		return nil, err
	}
	return refs, nil
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
			return fmt.Errorf("action form enctype %q must be literal", value)
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
				var err error
				field, err = mergeActionFormField(previous, field)
				if err != nil {
					return err
				}
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
	case "button", "input", "textarea", "select":
	default:
		return ActionFormField{}, false, nil
	}
	var field ActionFormField
	controlType := ""
	for _, attr := range element.Attrs {
		if attr.Name == "required" && element.Name != "button" {
			field.Required = true
			continue
		}
		switch attr.Name {
		case "minlength":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				field.MinLength, err = parseLengthConstraint("minlength", value)
				if err != nil {
					return ActionFormField{}, false, err
				}
			}
			continue
		case "maxlength":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				field.MaxLength, err = parseLengthConstraint("maxlength", value)
				if err != nil {
					return ActionFormField{}, false, err
				}
			}
			continue
		case "pattern":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				if strings.TrimSpace(value) == "" {
					return ActionFormField{}, false, fmt.Errorf("action form %s pattern must not be empty", element.Name)
				}
				field.Pattern = value
			}
			continue
		case "g:message:required", "g:message:minlength", "g:message:maxlength", "g:message:pattern":
			value, ok, err := literalValidationMessage(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				switch attr.Name {
				case "g:message:required":
					field.RequiredMessage = value
				case "g:message:minlength":
					field.MinLengthMessage = value
				case "g:message:maxlength":
					field.MaxLengthMessage = value
				case "g:message:pattern":
					field.PatternMessage = value
				}
			}
			continue
		}
		if (element.Name == "button" || element.Name == "input") && attr.Name == "type" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				continue
			}
			controlType = strings.TrimSpace(attr.Value)
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
			return ActionFormField{}, false, fmt.Errorf("action form field name %q must be literal", name)
		}
		field.Name = name
	}
	if field.Name == "" {
		return ActionFormField{}, false, nil
	}
	if strings.ContainsAny(controlType, "{}") {
		return ActionFormField{}, false, fmt.Errorf("action form %s %q type %q must be literal", element.Name, field.Name, controlType)
	}
	if isNonSubmittingControl(element.Name, controlType) {
		return ActionFormField{}, false, nil
	}
	if strings.EqualFold(controlType, "file") {
		return ActionFormField{}, false, fmt.Errorf("file input %q is not supported before upload security rules are defined", field.Name)
	}
	if err := validateValidationMessages(element.Name, field); err != nil {
		return ActionFormField{}, false, err
	}
	return field, true, nil
}

func literalConstraintValue(element Element, attr Attr) (string, bool, error) {
	if element.Name == "button" || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return "", false, nil
	}
	value := strings.TrimSpace(attr.Value)
	if attr.Expression {
		return "", false, fmt.Errorf("action form %s %s %q must be literal", element.Name, attr.Name, value)
	}
	return value, true, nil
}

func literalValidationMessage(element Element, attr Attr) (string, bool, error) {
	if element.Name == "button" || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return "", false, nil
	}
	value := strings.TrimSpace(attr.Value)
	if attr.Expression {
		return "", false, fmt.Errorf("action form %s %s %q must be literal", element.Name, attr.Name, value)
	}
	return value, true, nil
}

func validateValidationMessages(elementName string, field ActionFormField) error {
	if field.RequiredMessage != "" && !field.Required {
		return fmt.Errorf("action form %s %q declares g:message:required without required", elementName, field.Name)
	}
	if field.MinLengthMessage != "" && field.MinLength == 0 {
		return fmt.Errorf("action form %s %q declares g:message:minlength without minlength", elementName, field.Name)
	}
	if field.MaxLengthMessage != "" && field.MaxLength == 0 {
		return fmt.Errorf("action form %s %q declares g:message:maxlength without maxlength", elementName, field.Name)
	}
	if field.PatternMessage != "" && field.Pattern == "" {
		return fmt.Errorf("action form %s %q declares g:message:pattern without pattern", elementName, field.Name)
	}
	return nil
}

func parseLengthConstraint(name string, value string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("action form %s must be a non-negative integer", name)
	}
	return number, nil
}

func mergeActionFormField(previous, next ActionFormField) (ActionFormField, error) {
	if previous.Name == "" {
		return next, nil
	}
	next.Required = next.Required || previous.Required
	var err error
	next.RequiredMessage, err = mergeStringConstraint(next.Name, "required message", previous.RequiredMessage, next.RequiredMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MinLength, err = mergeIntConstraint(next.Name, "minlength", previous.MinLength, next.MinLength)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MinLengthMessage, err = mergeStringConstraint(next.Name, "minlength message", previous.MinLengthMessage, next.MinLengthMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MaxLength, err = mergeIntConstraint(next.Name, "maxlength", previous.MaxLength, next.MaxLength)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MaxLengthMessage, err = mergeStringConstraint(next.Name, "maxlength message", previous.MaxLengthMessage, next.MaxLengthMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.Pattern, err = mergeStringConstraint(next.Name, "pattern", previous.Pattern, next.Pattern)
	if err != nil {
		return ActionFormField{}, err
	}
	next.PatternMessage, err = mergeStringConstraint(next.Name, "pattern message", previous.PatternMessage, next.PatternMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	return next, nil
}

func mergeIntConstraint(fieldName, constraint string, previous, next int) (int, error) {
	if previous == 0 {
		return next, nil
	}
	if next == 0 || previous == next {
		return previous, nil
	}
	return 0, fmt.Errorf("action form field %q declares conflicting %s constraints", fieldName, constraint)
}

func mergeStringConstraint(fieldName, constraint string, previous, next string) (string, error) {
	if previous == "" {
		return next, nil
	}
	if next == "" || previous == next {
		return previous, nil
	}
	return "", fmt.Errorf("action form field %q declares conflicting %s constraints", fieldName, constraint)
}

func isNonSubmittingControl(elementName, controlType string) bool {
	typ := strings.ToLower(strings.TrimSpace(controlType))
	switch elementName {
	case "button":
		return typ == "button" || typ == "reset"
	case "input":
		return typ == "button" || typ == "reset"
	default:
		return false
	}
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

func isSPAAssetReference(value string) bool {
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
	components   map[string]Component
	ownerPackage string
	uses         map[string]string
	values       map[string]string
	tainted      map[string]bool
	actions      map[string]string
	stack        map[string]bool
	slotHTML     string
	slots        map[string]slotContent
	stateFields  map[string]bool
	readFields   map[string]bool
	bindFields   map[string]bool
	conditional  *conditionalRender
	handlers     map[string]clientlang.Handler
	stateTypes   map[string]clientlang.ValueType
	refs         map[string]clientlang.Ref
	emits        map[string]clientlang.Emit
	loopSeq      *int
	bindingSeq   *int
	islandSeq    *int
	loopItem     *loopItemRender
	templateLoop *templateLoopRender
	scopeIDs     []string
	selectBound  bool
	selectValue  string
}

type loopItemRender struct {
	Group    string
	KeyExpr  string
	KeyValue string
}

type templateLoopRender struct{}
