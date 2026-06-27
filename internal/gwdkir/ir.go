// Package gwdkir defines the stable internal representation shared by GOWDK
// compiler passes after .gwdk AST analysis.
package gwdkir

import (
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
)

// Program is the normalized compiler IR produced from analyzed .gwdk sources.
type Program struct {
	Packages              []Package
	Pages                 []Page
	Components            []Component
	Layouts               []Layout
	Routes                []Route
	Endpoints             []Endpoint
	Templates             []Template
	ContractRefs          []ContractReference
	RealtimeSubscriptions []RealtimeSubscription
	QueryInvalidations    []QueryInvalidation
	AuditSpecs            []AuditSpec
	ClientBehaviors       []ClientBehavior
	Assets                []Asset
	SourceMap             SourceMap
	Diagnostics           []Diagnostic
}

// Diagnostic records an author-facing problem found while assembling IR.
type Diagnostic struct {
	Code    string
	Source  string
	Span    source.SourceSpan
	Message string
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
	Span    source.SourceSpan
}

type SourceKind string

const (
	SourcePage      SourceKind = "page"
	SourceComponent SourceKind = "component"
	SourceLayout    SourceKind = "layout"
	SourceAudit     SourceKind = "audit"
)

// Import records a Go package import used by analyzed source.
type Import struct {
	Alias string
	Path  string
	Span  source.SourceSpan
}

// Use records an explicit GOWDK package import.
type Use struct {
	Alias   string
	Package string
	Span    source.SourceSpan
}

// Store records one shared state declaration.
type Store struct {
	Name string
	Type GoRef
	Init GoRef
	// Persist is the optional `persist "<scope>"` modifier scope literal
	// ("" when not persisted). Validated in the compiler, not the parser.
	Persist string
	// PersistSet reports whether a `persist` clause was present, so an explicit
	// empty scope (`persist ""`) is diagnosed instead of treated as unpersisted.
	PersistSet bool
	Span       source.SourceSpan
}

// Page is the normalized IR for one page source.
type Page struct {
	Source      string
	Package     string
	ID          string
	Route       string
	RouteParams []source.RouteParam
	Render      gowdk.RenderMode
	Cache       string
	Revalidate  string
	ErrorPage   string
	Metadata    PageMetadata
	Layouts     []string
	Guards      []string
	CSS         []string
	JS          []string
	InlineJS    []source.InlineScript
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
	Robots      string
	NoIndex     bool
	Preload     []HeadResource
	Prefetch    []HeadResource
	Structured  []StructuredData
}

type StructuredData struct {
	Kind string
}

type HeadResource struct {
	Href string
	As   string
}

type Blocks struct {
	Paths        bool
	PathsBody    string
	PathsRecords []LiteralRecord `json:"-"`
	Build        bool
	BuildBody    string
	BuildRecords []LiteralRecord `json:"-"`
	BuildCall    *BuildCall      `json:"-"`
	Server       bool
	ServerBody   string
	Client       bool
	ClientBody   string
	GoBlocks     []GoBlock
	View         bool
	ViewBody     string
	ViewNodes    []viewmodel.Node `json:"-"`
	Style        bool
	StyleBody    string
	Actions      []Action
	APIs         []API
	Fragments    []FragmentEndpoint
	Spans        BlockSpans
}

// LiteralRecord is a parsed literal record from paths {} or build {}.
type LiteralRecord struct {
	Fields      map[string]string
	Expressions map[string]string `json:"-"`
	FieldOrder  []string          `json:"-"`
	Span        source.SourceSpan
}

// BuildCall is a parsed imported or same-package build data function call.
type BuildCall struct {
	Alias    string
	Function string
	Span     source.SourceSpan
}

// GoBlock records one optional inline Go authoring block preserved for the
// extraction pipeline.
type GoBlock struct {
	Target string
	Body   string
	Span   source.SourceSpan
}

type PageSpans struct {
	Package     source.SourceSpan
	Page        source.SourceSpan
	Route       source.SourceSpan
	Render      source.SourceSpan
	Cache       source.SourceSpan
	Revalidate  source.SourceSpan
	ErrorPage   source.SourceSpan
	Title       source.SourceSpan
	Description source.SourceSpan
	Canonical   source.SourceSpan
	Image       source.SourceSpan
	Robots      source.SourceSpan
	NoIndex     source.SourceSpan
	Preload     []source.NamedSpan
	Prefetch    []source.NamedSpan
	Structured  []source.NamedSpan
	Layouts     []source.NamedSpan
	Guard       []source.NamedSpan
	CSS         []source.NamedSpan
	JS          []source.NamedSpan
	InlineJS    []source.NamedSpan
	RouteParams []source.NamedSpan
}

type BlockSpans struct {
	Paths         source.SourceSpan
	Build         source.SourceSpan
	Server        source.SourceSpan
	Client        source.SourceSpan
	GoBlocks      []source.NamedSpan
	View          source.SourceSpan
	ViewBodyStart source.SourcePosition
	Actions       []source.NamedSpan
	APIs          []source.NamedSpan
	Fragments     []source.NamedSpan
	Exports       source.SourceSpan
	Emits         source.SourceSpan
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
	Span           source.SourceSpan
	RouteSpan      source.SourceSpan
	RouteParams    []source.NamedSpan
	InputSpan      source.SourceSpan
	ValidationSpan source.SourceSpan
	RedirectSpan   source.SourceSpan
	ErrorPageSpan  source.SourceSpan
}

type Fragment struct {
	Target string
	Body   string
	Span   source.SourceSpan
}

type FragmentEndpoint struct {
	Name        string
	Method      string
	Route       string
	Target      string
	Body        string
	Span        source.SourceSpan
	RouteSpan   source.SourceSpan
	TargetSpan  source.SourceSpan
	RouteParams []source.NamedSpan
}

type API struct {
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	CORS          EndpointCORS
	Span          source.SourceSpan
	RouteSpan     source.SourceSpan
	RouteParams   []source.NamedSpan
	ErrorPageSpan source.SourceSpan
}

// EndpointCORS is an endpoint-local CORS policy declaration. Empty option
// fields inherit from Build.CORS when it is enabled.
type EndpointCORS struct {
	Enabled             bool
	AllowedOrigins      []string
	AllowedMethods      []string
	AllowedHeaders      []string
	ExposedHeaders      []string
	AllowCredentials    bool
	AllowCredentialsSet bool
	MaxAgeSeconds       int
	MaxAgeSet           bool
	Span                source.SourceSpan
}

// Component is the normalized IR for one component source.
type Component struct {
	Source      string
	Package     string
	Name        string
	Imports     []Import
	Uses        []Use
	CSS         []string
	JS          []string
	InlineJS    []source.InlineScript
	Assets      []string
	Props       []Prop
	PropsType   GoRef
	State       StateContract
	WASM        WASMContract
	Exports     []Export
	Emits       []Emit
	Blocks      Blocks
	Span        source.SourceSpan
	PackageSpan source.SourceSpan
	Spans       ComponentSpans
}

type ComponentSpans struct {
	CSS      []source.NamedSpan
	JS       []source.NamedSpan
	InlineJS []source.NamedSpan
	Assets   []source.NamedSpan
}

type StateContract struct {
	Type GoRef
	Init GoRef
	Span source.SourceSpan
}

type WASMContract struct {
	Package string
	Span    source.SourceSpan
}

type Prop struct {
	Name       string
	Type       string
	Default    string
	DefaultSet bool
	Span       source.SourceSpan
}

type Export struct {
	Name string
	Type string
	Span source.SourceSpan
}

// ComponentExportActiveFlag is reserved in generated exports payloads for the
// island mount-state flag.
const ComponentExportActiveFlag = "active"

type Emit struct {
	Name   string
	Params []EmitParam
	Span   source.SourceSpan
}

type EmitParam struct {
	Name string
	Type string
	Span source.SourceSpan
}

// Layout is the normalized IR for one layout source.
type Layout struct {
	Source        string
	Package       string
	ID            string
	Layouts       []string
	ErrorPage     string
	Uses          []Use
	Blocks        Blocks
	Span          source.SourceSpan
	LayoutSpans   []source.NamedSpan
	ErrorPageSpan source.SourceSpan
	PackageSpan   source.SourceSpan
}

// GoRef points at an imported Go package symbol.
type GoRef struct {
	Alias string
	Name  string
	Span  source.SourceSpan
}

// GoRefFromLiteral parses a Go type literal such as "ui.CartState" or
// "CartState" into a GoRef. A single "pkg.Name" qualifier becomes the alias and
// name; an unqualified literal sets only the name (same-package type).
func GoRefFromLiteral(literal string) GoRef {
	literal = strings.TrimSpace(literal)
	if alias, name, ok := strings.Cut(literal, "."); ok {
		return GoRef{Alias: alias, Name: name}
	}
	return GoRef{Name: literal}
}

// Route is page/file route metadata. Endpoint behavior is represented by
// Endpoint, not by route kinds.
type Route struct {
	ID            RouteID
	Kind          RouteKind
	Method        string
	Path          string
	PageID        string
	Package       string
	Render        gowdk.RenderMode
	Cache         string
	DynamicParams []string
	RouteParams   []source.RouteParam
	Layouts       []string
	Guards        []string
	Source        string
	Span          source.SourceSpan
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
	ID            EndpointID
	Kind          EndpointKind
	Source        EndpointSource
	Package       string
	PageID        string
	Symbol        string
	Method        string
	Path          string
	Cache         string
	Guards        []string
	CSRF          bool
	ErrorPage     string
	CORS          EndpointCORS
	DynamicParams []string
	RouteParams   []source.RouteParam
	SourceFile    string
	Span          source.SourceSpan
	Binding       Binding
}

// SourceMap keeps exact source spelling and spans outside semantic IR records.
// Generators consume normalized semantic fields and use source-map entries only
// for diagnostics, inspection, formatting, and trace metadata.
type SourceMap struct {
	Endpoints []EndpointSourceMap
}

// EndpointSourceMap records the exact standalone endpoint declaration that
// lowered into a normalized Program.Endpoints entry.
type EndpointSourceMap struct {
	ID            EndpointID
	Kind          string
	SourceKind    EndpointSource
	Package       string
	Source        string
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	Span          source.SourceSpan
	RouteSpan     source.SourceSpan
	RouteParams   []source.NamedSpan
	ErrorPageSpan source.SourceSpan
}

// StandaloneEndpointDeclaration is the lossless discovery input for
// `//gowdk:` Go endpoint comments. Program assembly lowers it into a normalized
// Endpoint plus an EndpointSourceMap entry.
type StandaloneEndpointDeclaration = EndpointSourceMap

// EndpointSource returns the source-map entry for a normalized endpoint ID.
func (program Program) EndpointSource(id EndpointID) (EndpointSourceMap, bool) {
	return program.SourceMap.Endpoint(id)
}

// Endpoint returns the source-map entry for a normalized endpoint ID.
func (sourceMap SourceMap) Endpoint(id EndpointID) (EndpointSourceMap, bool) {
	for _, endpoint := range sourceMap.Endpoints {
		if endpoint.ID == id {
			return endpoint, true
		}
	}
	return EndpointSourceMap{}, false
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
	Status        source.BackendBindingStatus
	Message       string
	ImportPath    string
	PackageName   string
	FunctionName  string
	Signature     source.BackendSignatureKind
	InputType     string
	InputPointer  bool
	InputFields   []source.BackendInputField
	ResultType    string
	ResultPointer bool
	ResultFields  []source.BackendResultField
}

// Template records a renderable view block.
type Template struct {
	OwnerKind SourceKind
	OwnerID   string
	Package   string
	Source    string
	Route     string
	Guards    []string
	Imports   []Import
	Uses      []Use
	Body      string
	Nodes     []viewmodel.Node `json:"-"`
	Span      source.SourceSpan
	BodyStart source.SourcePosition
}

// ContractReference records a source-level reference to a backend contract.
// Binding to Go contract metadata is a later analyzer step.
type ContractReference struct {
	Kind              ContractKind
	Name              string
	ImportAlias       string
	ImportPath        string
	Type              string
	Result            string
	Roles             []string
	Guards            []string
	InputFields       []source.BackendInputField
	ResultFields      []source.BackendInputField
	Method            string
	Path              string
	Status            ContractBindingStatus
	Handler           string
	Register          string
	Message           string
	DeclarationSource string
	DeclarationSpan   source.SourceSpan
	OwnerKind         SourceKind
	OwnerID           string
	Package           string
	Source            string
	Span              source.SourceSpan
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

// RealtimeSubscription records a query-bounded browser subscription to a
// presentation-event contract. Binding to Go contract metadata is a later
// analyzer step.
type RealtimeSubscription struct {
	Query            string
	QueryImportAlias string
	QueryImportPath  string
	QueryType        string
	Event            string
	EventImportAlias string
	EventImportPath  string
	EventType        string
	EventCategory    string
	Roles            []string
	Guards           []string
	Status           ContractBindingStatus
	Handler          string
	Register         string
	Message          string
	OwnerKind        SourceKind
	OwnerID          string
	Package          string
	Source           string
	Span             source.SourceSpan
	QuerySpan        source.SourceSpan
}

// QueryInvalidation records a bound query region that should refresh when a
// backend event type is emitted by a successful command.
type QueryInvalidation struct {
	Query            string
	QueryImportAlias string
	QueryImportPath  string
	QueryType        string
	Event            string
	EventImportPath  string
	EventType        string
	EventCategory    string
	Guards           []string
	Status           ContractBindingStatus
	Message          string
	OwnerKind        SourceKind
	OwnerID          string
	Package          string
	Source           string
	Span             source.SourceSpan
}

// AuditSpec is the normalized IR for one *.audit.gwdk source.
type AuditSpec struct {
	Source   string
	Package  string
	Policies []AuditPolicy
	Tests    []AuditTest
	Span     source.SourceSpan
}

// AuditPolicy declares a composable policy that can extend other policies and
// apply rules to selectors.
type AuditPolicy struct {
	Name    string
	Extends []string
	Applies []AuditApply
	Rules   []AuditRule
	Span    source.SourceSpan
}

// AuditApply records one selector applied to a declared policy.
type AuditApply struct {
	Selector string
	Span     source.SourceSpan
}

// AuditRule records one declared policy rule. Attrs carries structured arguments
// for rules that need more than a single value (for example a raw-HTML
// exception's owner, justification, expiry, and sanitizer contract).
type AuditRule struct {
	Kind  string
	Value string
	Code  string
	Attrs map[string]string `json:"Attrs,omitempty"`
	Span  source.SourceSpan
}

// AuditTest preserves a declared test block for Phase 4 runtime verification.
type AuditTest struct {
	Name string
	Body string
	Span source.SourceSpan
}

// ClientBehavior records a compiler-owned client block. The body is retained
// until the client language has a dedicated full AST.
type ClientBehavior struct {
	Component string
	Package   string
	Source    string
	Body      string
	Span      source.SourceSpan
}

// Asset records source-selected assets and future generated assets.
type Asset struct {
	Kind       AssetKind
	OwnerID    string
	Package    string
	Source     string
	Path       string
	Inline     string
	Name       string
	UseAlias   string
	UsePackage string
	ScopeID    string
	HashKey    string
	Span       source.SourceSpan
}

type AssetKind string

const (
	AssetCSS  AssetKind = "css"
	AssetJS   AssetKind = "js"
	AssetFile AssetKind = "asset"
	AssetWASM AssetKind = "wasm"
)
