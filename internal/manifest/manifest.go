package manifest

import (
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/source"
)

// Manifest is the compiler's normalized view of discovered .gwdk files.
type Manifest struct {
	Pages           []Page
	Components      []Component
	Layouts         []Layout
	Endpoints       []EndpointDeclaration
	BackendBindings []BackendBinding
}

// The shared leaf value types now live in internal/source. These aliases keep
// existing manifest.* references compiling while packages migrate to source.*.

// SourcePosition is a 1-based source location in a parsed .gwdk file.
type SourcePosition = source.SourcePosition

// SourceSpan is a 1-based source range. End is exclusive.
type SourceSpan = source.SourceSpan

// NamedSpan records the source range for a named declaration or reference.
type NamedSpan = source.NamedSpan

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
	JS          []NamedSpan
	InlineJS    []NamedSpan
	RouteParams []NamedSpan
}

// ComponentSpans records source ranges for component annotations.
type ComponentSpans struct {
	CSS      []NamedSpan
	JS       []NamedSpan
	InlineJS []NamedSpan
	Assets   []NamedSpan
}

// BlockSpans records source ranges for page, component, or layout blocks.
type BlockSpans struct {
	Paths         SourceSpan
	Build         SourceSpan
	Load          SourceSpan
	Client        SourceSpan
	GoBlocks      []NamedSpan
	View          SourceSpan
	ViewBodyStart SourcePosition
	Actions       []NamedSpan
	APIs          []NamedSpan
	Fragments     []NamedSpan
	Exports       SourceSpan
	Emits         SourceSpan
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
	JS          []string
	InlineJS    []InlineScript
	Imports     []Import
	Uses        []Use
	Stores      []Store
	Paths       bool
	Blocks      Blocks
	LoadBinding BackendBinding
	Spans       PageSpans
}

// InlineScript records browser module code declared directly inside a .gwdk
// source file. Path-based script declarations should remain preferred.
type InlineScript = source.InlineScript

// InlineScriptName returns the deterministic generated filename for the
// zero-based inline browser script declaration index in one source owner.
func InlineScriptName(index int) string {
	return source.InlineScriptName(index)
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
	return source.ErrorPagePath(value)
}

// RouteParam describes one dynamic route parameter and its declared scalar
// type. Empty Type means string for compatibility with legacy {name} syntax.
type RouteParam = source.RouteParam

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
	GoBlocks   []GoBlock
	View       bool
	ViewBody   string
	Style      bool
	StyleBody  string
	Actions    []Action
	APIs       []API
	Fragments  []FragmentEndpoint
	Spans      BlockSpans
}

// GoBlock records one optional inline Go authoring block. Target is empty
// for general package Go, or a lane/addon target such as "client", "ssr", or
// "addon.contracts".
type GoBlock struct {
	Target string
	Body   string
	Span   SourceSpan
}

// Component describes a .cmp.gwdk component after parsing and normalization.
type Component struct {
	Source      string
	Package     string
	Name        string
	Imports     []Import
	Uses        []Use
	CSS         []string
	JS          []string
	InlineJS    []InlineScript
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
type BackendBindingStatus = source.BackendBindingStatus

const (
	BackendBindingBound                = source.BackendBindingBound
	BackendBindingMissing              = source.BackendBindingMissing
	BackendBindingUnsupportedSignature = source.BackendBindingUnsupportedSignature
)

// BackendSignatureKind describes the supported Go handler shape.
type BackendSignatureKind = source.BackendSignatureKind

const (
	BackendSignatureAction0       = source.BackendSignatureAction0
	BackendSignatureActionValues  = source.BackendSignatureActionValues
	BackendSignatureActionForm    = source.BackendSignatureActionForm
	BackendSignatureActionFormPtr = source.BackendSignatureActionFormPtr
	BackendSignatureAPI           = source.BackendSignatureAPI
	BackendSignatureFragment      = source.BackendSignatureFragment
	BackendSignatureLoad          = source.BackendSignatureLoad
	BackendSignatureLoadError     = source.BackendSignatureLoadError
)

// BackendInputField describes one form field decoded into a Go action input
// struct from compile-time Go AST metadata.
type BackendInputField = source.BackendInputField

// BackendBinding describes the Go handler selected for an act or api block.
type BackendBinding = source.BackendBinding

// RenderMode returns the effective render mode for a page.
func (page Page) RenderMode(defaultMode gowdk.RenderMode) gowdk.RenderMode {
	if page.Render != "" {
		return page.Render
	}
	if page.Blocks.Load || page.HasGoBlock("ssr") {
		return gowdk.SSR
	}
	if defaultMode == "" {
		return gowdk.SPA
	}
	return defaultMode
}

// HasGoBlock reports whether the page declares a go block for target.
func (page Page) HasGoBlock(target string) bool {
	for _, block := range page.Blocks.GoBlocks {
		if block.Target == target {
			return true
		}
	}
	return false
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
	params := RouteParamsFromPath(page.Route)
	if len(params) == 0 {
		return nil
	}

	names := make([]string, 0, len(params))
	seen := map[string]bool{}
	for _, param := range params {
		if !seen[param.Name] {
			seen[param.Name] = true
			names = append(names, param.Name)
		}
	}
	sort.Strings(names)
	return names
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
	params := RouteParamsFromPath(page.Route)
	if len(params) == 0 {
		return nil
	}
	out := make([]RouteParam, 0, len(params))
	seen := map[string]bool{}
	for _, param := range params {
		if seen[param.Name] {
			continue
		}
		seen[param.Name] = true
		out = append(out, param)
	}
	return out
}

// RouteParamsFromPath scans a route path for `{name}` and `{name:type}`
// segments. Invalid brace contents are ignored here; route validation owns
// reporting malformed route syntax.
func RouteParamsFromPath(route string) []RouteParam {
	var params []RouteParam
	for index := 0; index < len(route); index++ {
		if route[index] != '{' {
			continue
		}
		end := strings.IndexByte(route[index:], '}')
		if end < 0 {
			continue
		}
		end += index
		body := route[index+1 : end]
		name, paramType, ok := splitRouteParamBody(body)
		if ok {
			params = append(params, RouteParam{Name: name, Type: paramType})
		}
		index = end
	}
	return params
}

func splitRouteParamBody(body string) (string, string, bool) {
	name := body
	paramType := "string"
	if before, after, ok := strings.Cut(body, ":"); ok {
		name = before
		paramType = after
	}
	if !isRouteIdent(name) || !isRouteIdent(paramType) {
		return "", "", false
	}
	return name, paramType, true
}

func isRouteIdent(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
