package manifest

import (
	"regexp"
	"sort"

	"github.com/cssbruno/gowdk"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Manifest is the compiler's normalized view of discovered .gwdk files.
type Manifest struct {
	Pages      []Page
	Components []Component
	Layouts    []Layout
}

// SourcePosition is a 1-based source location in a parsed .gwdk file.
type SourcePosition struct {
	Line   int
	Column int
}

// SourceSpan is a 1-based source range. End is exclusive.
type SourceSpan struct {
	Start SourcePosition
	End   SourcePosition
}

// NamedSpan records the source range for a named declaration or reference.
type NamedSpan struct {
	Name string
	Span SourceSpan
}

// PageSpans records source ranges for page annotations and declarations.
type PageSpans struct {
	Page        SourceSpan
	Route       SourceSpan
	Render      SourceSpan
	Layouts     []NamedSpan
	Guard       []NamedSpan
	CSS         []NamedSpan
	RouteParams []NamedSpan
}

// BlockSpans records source ranges for page, component, or layout blocks.
type BlockSpans struct {
	Paths   SourceSpan
	Build   SourceSpan
	Load    SourceSpan
	View    SourceSpan
	Actions []NamedSpan
	APIs    []NamedSpan
}

// Page describes a .gwdk page after parsing and normalization.
type Page struct {
	Source  string
	ID      string
	Route   string
	Render  gowdk.RenderMode
	Layouts []string
	Guard   []string
	CSS     []string
	Paths   bool
	Blocks  Blocks
	Spans   PageSpans
}

// Blocks records the source blocks declared by a page.
type Blocks struct {
	PathsBody string
	Build     bool
	BuildBody string
	Load      bool
	LoadBody  string
	View      bool
	ViewBody  string
	Actions   []Action
	APIs      []API
	Spans     BlockSpans
}

// Component describes a .cmp.gwdk component after parsing and normalization.
type Component struct {
	Source string
	Name   string
	Props  []Prop
	Blocks Blocks
	Span   SourceSpan
}

// Layout describes a .layout.gwdk layout after parsing and normalization.
type Layout struct {
	Source string
	ID     string
	Blocks Blocks
	Span   SourceSpan
}

// Prop describes one component prop declaration.
type Prop struct {
	Name string
	Type string
	Span SourceSpan
}

// Action describes an act block.
type Action struct {
	Name           string
	Body           string
	InputName      string
	InputType      string
	ValidatesInput bool
	Redirect       string
	Fragments      []Fragment
	Span           SourceSpan
	InputSpan      SourceSpan
	ValidationSpan SourceSpan
	RedirectSpan   SourceSpan
}

// Fragment describes a server fragment declared inside an action.
type Fragment struct {
	Target string
	Body   string
	Span   SourceSpan
}

// API describes an api block.
type API struct {
	Name        string
	Method      string
	Route       string
	Span        SourceSpan
	RouteSpan   SourceSpan
	RouteParams []NamedSpan
}

// RenderMode returns the effective render mode for a page.
func (page Page) RenderMode(defaultMode gowdk.RenderMode) gowdk.RenderMode {
	if page.Render != "" {
		return page.Render
	}
	if defaultMode == "" {
		return gowdk.Static
	}
	return defaultMode
}

// DynamicParams returns route parameters declared with /path/{param} syntax.
func (page Page) DynamicParams() []string {
	matches := routeParamPattern.FindAllStringSubmatch(page.Route, -1)
	if len(matches) == 0 {
		return nil
	}

	params := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		param := match[1]
		if !seen[param] {
			seen[param] = true
			params = append(params, param)
		}
	}
	sort.Strings(params)
	return params
}
