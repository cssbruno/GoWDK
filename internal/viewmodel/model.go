package viewmodel

import "github.com/cssbruno/gowdk/internal/clientlang"

// Node is a parsed view markup node.
type Node interface {
	viewNode()
}

// Text is escaped text content.
type Text struct {
	Value string
	Start int
	End   int
}

func (Text) viewNode() {}

// Attr is a literal HTML or component attribute.
type Attr struct {
	Name       string
	Value      string
	Boolean    bool
	Expression bool
	Spread     bool
	Start      int
	End        int
}

// Element is a lowercase HTML element in a parsed view tree.
type Element struct {
	Name     string
	Attrs    []Attr
	Children []Node
	Start    int
	End      int
}

func (Element) viewNode() {}

// ComponentCall invokes a parsed component with literal or expression props.
type ComponentCall struct {
	Name     string
	Attrs    []Attr
	Children []Node
	Start    int
	End      int
}

func (ComponentCall) viewNode() {}

// AwaitBlock renders local pending/resolved/error UI around a bounded client
// async expression inside a browser island.
type AwaitBlock struct {
	Expression string
	ResultName string
	ErrorName  string
	Pending    []Node
	Then       []Node
	Catch      []Node
	Start      int
	End        int
}

func (AwaitBlock) viewNode() {}

// InlineScript records browser module code declared directly inside a
// component source file.
type InlineScript struct {
	Name string
	Body string
}

// Component is a literal component template known to the view renderer and
// generated-output passes.
type Component struct {
	Name          string
	Package       string
	Uses          map[string]string
	JS            []string
	InlineJS      []InlineScript
	ScopeIDs      []string
	DefaultIsland string
	Props         []string
	PropTypes     map[string]clientlang.ValueType
	PropDefaults  map[string]string
	State         map[string]string
	StateJSON     string
	Handlers      map[string]clientlang.Handler
	HandlersJSON  string
	StateTypes    map[string]clientlang.ValueType
	Refs          map[string]clientlang.Ref
	Emits         map[string]clientlang.Emit
	Exports       map[string]clientlang.ValueType
	Computed      []clientlang.Computed
	Body          string
	Nodes         []Node
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

// HasStateField reports whether a component declares local browser state with
// the given name.
func (component Component) HasStateField(name string) bool {
	if _, ok := component.State[name]; ok {
		return true
	}
	if _, ok := component.StateTypes[name]; ok {
		return true
	}
	return false
}

// PropType returns the declared scalar type for a prop. Components constructed
// by older tests only populate Props; those props remain string-typed.
func (component Component) PropType(name string) clientlang.ValueType {
	if component.PropTypes != nil {
		if typ, ok := component.PropTypes[name]; ok && typ != clientlang.TypeUnknown {
			return typ
		}
	}
	if component.HasProp(name) {
		return clientlang.TypeString
	}
	return clientlang.TypeUnknown
}

// Identity is the package-qualified component identity used for compiler-time
// resolution and recursion checks. The public call name can be an import alias.
func (component Component) Identity() string {
	if component.Package == "" {
		return component.Name
	}
	return component.Package + "." + component.Name
}
