// Package gwdkir defines the stable internal representation shared by GOWDK
// compiler passes after .gwdk AST analysis.
package gwdkir

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

const Version = 1

// Program is the normalized compiler IR produced from analyzed .gwdk sources.
type Program struct {
	Version         int
	Packages        []Package
	Pages           []Page
	Components      []Component
	Layouts         []Layout
	Routes          []Route
	Endpoints       []Endpoint
	Templates       []Template
	ContractRefs    []ContractReference
	ClientBehaviors []ClientBehavior
	Assets          []Asset
	Generated       GeneratedOutput
}

// Package groups analyzed GOWDK source files by declared package.
type Package struct {
	Name       string
	SourceDirs []string
	Files      []SourceFile
	Imports    []Import
	Uses       []Use
	Stores     []Store
}

// SourceFile records one analyzed source file.
type SourceFile struct {
	Path    string
	Kind    SourceKind
	Package string
	Name    string
	Span    manifest.SourceSpan
}

type SourceKind string

const (
	SourcePage      SourceKind = "page"
	SourceComponent SourceKind = "component"
	SourceLayout    SourceKind = "layout"
)

// Import records a Go package import used by analyzed source.
type Import struct {
	Alias string
	Path  string
	Span  manifest.SourceSpan
}

// Use records an explicit GOWDK package import.
type Use struct {
	Alias   string
	Package string
	Span    manifest.SourceSpan
}

// Store records one shared state declaration.
type Store struct {
	Name string
	Type GoRef
	Init GoRef
	Span manifest.SourceSpan
}

// Page is the normalized IR for one page source.
type Page struct {
	Source      string
	Package     string
	ID          string
	Route       string
	RouteParams []manifest.RouteParam
	Render      gowdk.RenderMode
	Cache       string
	Revalidate  string
	ErrorPage   string
	Metadata    PageMetadata
	Layouts     []string
	Guards      []string
	CSS         []string
	Imports     []Import
	Uses        []Use
	Stores      []Store
	Blocks      Blocks
	LoadBinding Binding
	Spans       PageSpans
}

type PageMetadata struct {
	Title       string
	Description string
	Canonical   string
	Image       string
}

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

type PageSpans struct {
	Package     manifest.SourceSpan
	Page        manifest.SourceSpan
	Route       manifest.SourceSpan
	Render      manifest.SourceSpan
	Cache       manifest.SourceSpan
	Revalidate  manifest.SourceSpan
	ErrorPage   manifest.SourceSpan
	Title       manifest.SourceSpan
	Description manifest.SourceSpan
	Canonical   manifest.SourceSpan
	Image       manifest.SourceSpan
	Layouts     []manifest.NamedSpan
	Guard       []manifest.NamedSpan
	CSS         []manifest.NamedSpan
	RouteParams []manifest.NamedSpan
}

type BlockSpans struct {
	Paths         manifest.SourceSpan
	Build         manifest.SourceSpan
	Load          manifest.SourceSpan
	Client        manifest.SourceSpan
	View          manifest.SourceSpan
	ViewBodyStart manifest.SourcePosition
	Actions       []manifest.NamedSpan
	APIs          []manifest.NamedSpan
	Fragments     []manifest.NamedSpan
	Exports       manifest.SourceSpan
	Emits         manifest.SourceSpan
}

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
	Span           manifest.SourceSpan
	RouteSpan      manifest.SourceSpan
	RouteParams    []manifest.NamedSpan
	InputSpan      manifest.SourceSpan
	ValidationSpan manifest.SourceSpan
	RedirectSpan   manifest.SourceSpan
	ErrorPageSpan  manifest.SourceSpan
}

type Fragment struct {
	Target string
	Body   string
	Span   manifest.SourceSpan
}

type FragmentEndpoint struct {
	Name        string
	Method      string
	Route       string
	Target      string
	Body        string
	Span        manifest.SourceSpan
	RouteSpan   manifest.SourceSpan
	TargetSpan  manifest.SourceSpan
	RouteParams []manifest.NamedSpan
}

type API struct {
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	Span          manifest.SourceSpan
	RouteSpan     manifest.SourceSpan
	RouteParams   []manifest.NamedSpan
	ErrorPageSpan manifest.SourceSpan
}

// Component is the normalized IR for one component source.
type Component struct {
	Source      string
	Package     string
	Name        string
	Imports     []Import
	Uses        []Use
	CSS         []string
	Assets      []string
	Props       []Prop
	PropsType   GoRef
	State       StateContract
	WASM        WASMContract
	Exports     []Export
	Emits       []Emit
	Blocks      Blocks
	Span        manifest.SourceSpan
	PackageSpan manifest.SourceSpan
	Spans       ComponentSpans
}

type ComponentSpans struct {
	CSS    []manifest.NamedSpan
	Assets []manifest.NamedSpan
}

type StateContract struct {
	Type GoRef
	Init GoRef
	Span manifest.SourceSpan
}

type WASMContract struct {
	Package string
	Span    manifest.SourceSpan
}

type Prop struct {
	Name string
	Type string
	Span manifest.SourceSpan
}

type Export struct {
	Name string
	Type string
	Span manifest.SourceSpan
}

type Emit struct {
	Name   string
	Params []EmitParam
	Span   manifest.SourceSpan
}

type EmitParam struct {
	Name string
	Type string
	Span manifest.SourceSpan
}

// Layout is the normalized IR for one layout source.
type Layout struct {
	Source      string
	Package     string
	ID          string
	Uses        []Use
	Blocks      Blocks
	Span        manifest.SourceSpan
	PackageSpan manifest.SourceSpan
}

// GoRef points at an imported Go package symbol.
type GoRef struct {
	Alias string
	Name  string
	Span  manifest.SourceSpan
}

// Route is page/file route metadata. Endpoint behavior is represented by
// Endpoint, not by route kinds.
type Route struct {
	Kind          RouteKind
	Method        string
	Path          string
	PageID        string
	Package       string
	Render        gowdk.RenderMode
	Cache         string
	DynamicParams []string
	RouteParams   []manifest.RouteParam
	Layouts       []string
	Guards        []string
	Source        string
	Span          manifest.SourceSpan
}

type RouteKind string

const (
	RouteStatic RouteKind = "static"
	RouteSPA    RouteKind = "spa"
	RouteSSR    RouteKind = "ssr"
	RouteHybrid RouteKind = "hybrid"
)

// Endpoint is framework-neutral backend endpoint metadata.
type Endpoint struct {
	Kind          EndpointKind
	Source        EndpointSource
	Package       string
	PageID        string
	Symbol        string
	Method        string
	Path          string
	ErrorPage     string
	DynamicParams []string
	SourceFile    string
	Span          manifest.SourceSpan
	Binding       Binding
}

type EndpointKind string

const (
	EndpointAction   EndpointKind = "action"
	EndpointAPI      EndpointKind = "api"
	EndpointFragment EndpointKind = "fragment"
)

type EndpointSource string

const (
	EndpointSourceGOWDK EndpointSource = "gwdk"
	EndpointSourceGo    EndpointSource = "go"
)

// Binding describes the selected Go backend handler when one is known.
type Binding struct {
	Status       manifest.BackendBindingStatus
	Message      string
	ImportPath   string
	PackageName  string
	FunctionName string
	Signature    manifest.BackendSignatureKind
	InputType    string
	InputPointer bool
	InputFields  []manifest.BackendInputField
}

// Template records a renderable view block.
type Template struct {
	OwnerKind SourceKind
	OwnerID   string
	Package   string
	Source    string
	Route     string
	Imports   []Import
	Body      string
	Span      manifest.SourceSpan
	BodyStart manifest.SourcePosition
}

// ContractReference records a source-level reference to a backend contract.
// Binding to Go contract metadata is a later analyzer step.
type ContractReference struct {
	Kind        ContractKind
	Name        string
	ImportAlias string
	ImportPath  string
	Type        string
	Result      string
	Roles       []string
	InputFields []manifest.BackendInputField
	Method      string
	Path        string
	Status      ContractBindingStatus
	Handler     string
	Register    string
	Message     string
	OwnerKind   SourceKind
	OwnerID     string
	Package     string
	Source      string
	Span        manifest.SourceSpan
}

type ContractKind string

const (
	ContractCommand ContractKind = "command"
	ContractQuery   ContractKind = "query"
)

type ContractBindingStatus string

const (
	ContractBindingUnknown ContractBindingStatus = "unknown"
	ContractBindingBound   ContractBindingStatus = "bound"
	ContractBindingMissing ContractBindingStatus = "missing"
	ContractBindingInvalid ContractBindingStatus = "invalid"
)

// ClientBehavior records a compiler-owned client block. The body is retained
// until the client language has a dedicated full AST.
type ClientBehavior struct {
	Component string
	Package   string
	Source    string
	Body      string
	Span      manifest.SourceSpan
}

// Asset records source-selected assets and future generated assets.
type Asset struct {
	Kind       AssetKind
	OwnerID    string
	Package    string
	Source     string
	Path       string
	Name       string
	UseAlias   string
	UsePackage string
	ScopeID    string
	HashKey    string
	Span       manifest.SourceSpan
}

type AssetKind string

const (
	AssetCSS  AssetKind = "css"
	AssetFile AssetKind = "asset"
	AssetWASM AssetKind = "wasm"
)

// GeneratedOutput is the stable planning shape for generated artifacts.
type GeneratedOutput struct {
	GoPackages []GeneratedGoPackage
	Files      []GeneratedFile
}

type GeneratedGoPackage struct {
	Name    string
	Path    string
	Purpose string
	Imports []Import
}

type GeneratedFile struct {
	Path    string
	Kind    string
	OwnerID string
}
