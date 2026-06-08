package view

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/clientlang"
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
	Name          string
	Package       string
	Uses          map[string]string
	ScopeIDs      []string
	DefaultIsland string
	Props         []string
	State         map[string]string
	StateJSON     string
	Handlers      map[string]clientlang.Handler
	HandlersJSON  string
	StateTypes    map[string]clientlang.ValueType
	Refs          map[string]clientlang.Ref
	Emits         map[string]clientlang.Emit
	Computed      []clientlang.Computed
	Body          string
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
	Kind   ContractReferenceKind
	Name   string
	Method string
	Path   string
	Start  int
	End    int
}

type ContractReferenceKind string

const (
	ContractReferenceCommand ContractReferenceKind = "command"
	ContractReferenceQuery   ContractReferenceKind = "query"
)

// CommandReference records one form-local backend command intent.
type CommandReference struct {
	Command string
	Method  string
	Path    string
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
