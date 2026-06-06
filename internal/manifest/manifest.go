package manifest

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::([A-Za-z_][A-Za-z0-9_]*))?\}`)

// Manifest is the compiler's normalized view of discovered .gwdk files.
type Manifest struct {
	Pages           []Page
	Components      []Component
	Layouts         []Layout
	Endpoints       []EndpointDeclaration
	BackendBindings []BackendBinding
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

// Import records a Go import declared by a .gwdk page.
type Import struct {
	Alias string
	Path  string
	Span  SourceSpan
}

// Use records a GOWDK source package import declared by a .gwdk file.
// Go imports still use Import; Use is for package-peer .gwdk pages,
// components, layouts, stores, and assets selected by the GOWDK compiler.
type Use struct {
	Alias   string
	Package string
	Span    SourceSpan
}

// GoTypeRef references a Go type through an import alias declared in a .gwdk
// source file.
type GoTypeRef struct {
	Alias string
	Name  string
	Span  SourceSpan
}

// GoFuncRef references a Go function through an import alias declared in a
// .gwdk source file.
type GoFuncRef struct {
	Alias string
	Name  string
	Span  SourceSpan
}

// StateContract describes a local component state type and build-time
// initializer.
type StateContract struct {
	Type GoTypeRef
	Init GoFuncRef
	Span SourceSpan
}

// Store describes one page-scoped shared state declaration.
type Store struct {
	Name string
	Type GoTypeRef
	Init GoFuncRef
	Span SourceSpan
}

// WASMContract points an explicit browser-side Go package at a component.
type WASMContract struct {
	Package string
	Span    SourceSpan
}

// PageSpans records source ranges for page annotations and declarations.
type PageSpans struct {
	Package     SourceSpan
	Page        SourceSpan
	Route       SourceSpan
	Render      SourceSpan
	Cache       SourceSpan
	Revalidate  SourceSpan
	ErrorPage   SourceSpan
	Title       SourceSpan
	Description SourceSpan
	Canonical   SourceSpan
	Image       SourceSpan
	Layouts     []NamedSpan
	Guard       []NamedSpan
	CSS         []NamedSpan
	RouteParams []NamedSpan
}

// ComponentSpans records source ranges for component annotations.
type ComponentSpans struct {
	CSS    []NamedSpan
	Assets []NamedSpan
}

// BlockSpans records source ranges for page, component, or layout blocks.
type BlockSpans struct {
	Paths     SourceSpan
	Build     SourceSpan
	Load      SourceSpan
	Client    SourceSpan
	View      SourceSpan
	Actions   []NamedSpan
	APIs      []NamedSpan
	Fragments []NamedSpan
	Exports   SourceSpan
	Emits     SourceSpan
}

// Page describes a .gwdk page after parsing and normalization.
type Page struct {
	Source      string
	Package     string
	ID          string
	Route       string
	RouteParams []RouteParam
	Render      gowdk.RenderMode
	Cache       string
	Revalidate  string
	ErrorPage   string
	Metadata    PageMetadata
	Layouts     []string
	Guard       []string
	CSS         []string
	Imports     []Import
	Uses        []Use
	Stores      []Store
	Paths       bool
	Blocks      Blocks
	LoadBinding BackendBinding
	Spans       PageSpans
}

// CachePolicy returns the concrete Cache-Control policy generated for the page.
func (page Page) CachePolicy() string {
	return CachePolicyWithRevalidate(page.Cache, page.Revalidate)
}

// CachePolicyWithRevalidate appends the page revalidation directive to an
// explicit Cache-Control policy.
func CachePolicyWithRevalidate(cache string, revalidate string) string {
	if cache == "" || revalidate == "" {
		return cache
	}
	return cache + ", stale-while-revalidate=" + revalidate
}

// ErrorPagePath returns a clean generated-output-relative error page path.
func ErrorPagePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("@error requires a value")
	}
	if strings.ContainsAny(value, "\\?#") {
		return "", fmt.Errorf("@error must be a local generated HTML path without query, fragment, or backslash")
	}
	for _, part := range strings.Split(strings.TrimPrefix(value, "/"), "/") {
		if part == ".." {
			return "", fmt.Errorf("@error path must stay inside generated output")
		}
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	if cleaned == "/" || cleaned == "/." {
		return "", fmt.Errorf("@error requires a generated HTML file path")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if !strings.HasSuffix(strings.ToLower(cleaned), ".html") {
		return "", fmt.Errorf("@error path must end in .html")
	}
	return cleaned, nil
}

// RouteParam describes one dynamic route parameter and its declared scalar
// type. Empty Type means string for compatibility with legacy {name} syntax.
type RouteParam struct {
	Name string
	Type string
	Span SourceSpan
}

// PageMetadata describes HTML document metadata declared by a page.
type PageMetadata struct {
	Title       string
	Description string
	Canonical   string
	Image       string
}

// Blocks records the source blocks declared by a page.
type Blocks struct {
	PathsBody  string
	Build      bool
	BuildBody  string
	Load       bool
	LoadBody   string
	Client     bool
	ClientBody string
	View       bool
	ViewBody   string
	Actions    []Action
	APIs       []API
	Fragments  []FragmentEndpoint
	Spans      BlockSpans
}

// Component describes a .cmp.gwdk component after parsing and normalization.
type Component struct {
	Source      string
	Package     string
	Name        string
	Imports     []Import
	Uses        []Use
	CSS         []string
	Assets      []string
	Props       []Prop
	PropsType   GoTypeRef
	State       StateContract
	WASM        WASMContract
	Exports     []Export
	Emits       []Emit
	Blocks      Blocks
	Span        SourceSpan
	PackageSpan SourceSpan
	Spans       ComponentSpans
}

// Layout describes a .layout.gwdk layout after parsing and normalization.
type Layout struct {
	Source      string
	Package     string
	ID          string
	Uses        []Use
	Blocks      Blocks
	Span        SourceSpan
	PackageSpan SourceSpan
}

// Prop describes one component prop declaration.
type Prop struct {
	Name string
	Type string
	Span SourceSpan
}

// Export describes one typed public component export.
type Export struct {
	Name string
	Type string
	Span SourceSpan
}

// Emit describes one component event emitted by a browser island.
type Emit struct {
	Name   string
	Params []EmitParam
	Span   SourceSpan
}

// EmitParam describes one scalar field in a component event payload.
type EmitParam struct {
	Name string
	Type string
	Span SourceSpan
}

// Action describes an action endpoint declaration.
type Action struct {
	Name           string
	Method         string
	Route          string
	Body           string
	InputName      string
	InputType      string
	ValidatesInput bool
	Redirect       string
	Fragments      []Fragment
	ErrorPage      string
	Span           SourceSpan
	RouteSpan      SourceSpan
	RouteParams    []NamedSpan
	InputSpan      SourceSpan
	ValidationSpan SourceSpan
	RedirectSpan   SourceSpan
	ErrorPageSpan  SourceSpan
}

// Fragment describes a server fragment declared inside an action.
type Fragment struct {
	Target string
	Body   string
	Span   SourceSpan
}

// FragmentEndpoint describes a generated server fragment endpoint declaration.
type FragmentEndpoint struct {
	Name        string
	Method      string
	Route       string
	Target      string
	Body        string
	Span        SourceSpan
	RouteSpan   SourceSpan
	TargetSpan  SourceSpan
	RouteParams []NamedSpan
}

// API describes an API endpoint declaration.
type API struct {
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	Span          SourceSpan
	RouteSpan     SourceSpan
	RouteParams   []NamedSpan
	ErrorPageSpan SourceSpan
}

// EndpointSource identifies where an endpoint declaration came from.
type EndpointSource string

const (
	EndpointSourceGOWDK EndpointSource = "gwdk"
	EndpointSourceGo    EndpointSource = "go"
)

// EndpointDeclaration describes a standalone backend endpoint declaration.
// Page-owned declarations stay on Page.Blocks so page forms/fragments can still
// attach page-local behavior.
type EndpointDeclaration struct {
	Kind          string
	SourceKind    EndpointSource
	Package       string
	Source        string
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	Span          SourceSpan
	RouteSpan     SourceSpan
	RouteParams   []NamedSpan
	ErrorPageSpan SourceSpan
}

// BackendBindingStatus describes whether a .gwdk backend block has a matching
// same-package Go handler.
type BackendBindingStatus string

const (
	BackendBindingBound                BackendBindingStatus = "bound"
	BackendBindingMissing              BackendBindingStatus = "missing"
	BackendBindingUnsupportedSignature BackendBindingStatus = "unsupported_signature"
)

// BackendSignatureKind describes the supported Go handler shape.
type BackendSignatureKind string

const (
	BackendSignatureAction0       BackendSignatureKind = "action0"
	BackendSignatureActionValues  BackendSignatureKind = "action_values"
	BackendSignatureActionForm    BackendSignatureKind = "action_form"
	BackendSignatureActionFormPtr BackendSignatureKind = "action_form_ptr"
	BackendSignatureAPI           BackendSignatureKind = "api"
	BackendSignatureFragment      BackendSignatureKind = "fragment"
	BackendSignatureLoad          BackendSignatureKind = "load"
	BackendSignatureLoadError     BackendSignatureKind = "load_error"
)

// BackendInputField describes one form field decoded into a Go action input
// struct from compile-time Go AST metadata.
type BackendInputField struct {
	FieldName string
	FormName  string
	Type      string
}

// BackendBinding describes the Go handler selected for an act or api block.
type BackendBinding struct {
	Kind         string
	PageID       string
	Source       string
	BlockName    string
	Method       string
	Route        string
	ImportPath   string
	PackageName  string
	FunctionName string
	Signature    BackendSignatureKind
	InputType    string
	InputPointer bool
	InputFields  []BackendInputField
	Status       BackendBindingStatus
	Message      string
}

// RenderMode returns the effective render mode for a page.
func (page Page) RenderMode(defaultMode gowdk.RenderMode) gowdk.RenderMode {
	if page.Render != "" {
		return page.Render
	}
	if defaultMode == "" {
		return gowdk.SPA
	}
	return defaultMode
}

// DynamicParams returns route parameters declared with /path/{param} syntax.
func (page Page) DynamicParams() []string {
	if len(page.RouteParams) > 0 {
		params := make([]string, 0, len(page.RouteParams))
		seen := map[string]bool{}
		for _, param := range page.RouteParams {
			if param.Name == "" || seen[param.Name] {
				continue
			}
			seen[param.Name] = true
			params = append(params, param.Name)
		}
		sort.Strings(params)
		return params
	}
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

// TypedRouteParams returns route params with explicit type metadata. Untyped
// params are reported as string.
func (page Page) TypedRouteParams() []RouteParam {
	if len(page.RouteParams) > 0 {
		out := make([]RouteParam, 0, len(page.RouteParams))
		for _, param := range page.RouteParams {
			if param.Name == "" {
				continue
			}
			if param.Type == "" {
				param.Type = "string"
			}
			out = append(out, param)
		}
		return out
	}
	matches := routeParamPattern.FindAllStringSubmatch(page.Route, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]RouteParam, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		paramType := match[2]
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, RouteParam{Name: name, Type: paramType})
	}
	return out
}
