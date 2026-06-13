// Package gwdkast defines the typed syntax tree for .gwdk source files.
package gwdkast

import (
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
)

// File is the typed AST for the currently supported .gwdk syntax subset.
type File struct {
	Package    *Package
	Metadata   []MetadataDecl
	Page       *PageDecl
	Route      *RouteDecl
	Cache      *CacheDecl
	Revalidate *RevalidateDecl
	ErrorPage  *ErrorPageDecl
	Layouts    []LayoutRef
	Guards     []GuardRef
	CSS        []AssetRef
	JS         []AssetRef
	Assets     []AssetRef
	Component  *ComponentDecl
	Layout     *LayoutDecl
	Imports    []Import
	Uses       []Use
	Stores     []Store
	PropsType  *GoTypeRef
	State      *StateContract
	WASM       *WASMContract
	Blocks     []Block
	Actions    []Endpoint
	APIs       []Endpoint
	Fragments  []FragmentEndpoint
}

// Package is the top-level Go package declaration.
type Package struct {
	Name string
	Span source.SourceSpan
}

// MetadataDecl is one top-level keyword metadata declaration.
type MetadataDecl struct {
	Name  string
	Value string
	Span  source.SourceSpan
}

// PageDecl is an page declaration.
type PageDecl struct {
	ID   string
	Span source.SourceSpan
}

// ComponentDecl is an component declaration.
type ComponentDecl struct {
	Name string
	Span source.SourceSpan
}

// LayoutDecl is an layout declaration in a layout file.
type LayoutDecl struct {
	ID   string
	Span source.SourceSpan
}

// RouteDecl is an route declaration.
type RouteDecl struct {
	Path   string
	Params []RouteParam
	Span   source.SourceSpan
}

// RouteParam is one dynamic route segment declared by route.
type RouteParam struct {
	Name string
	Type string
	Span source.SourceSpan
}

// CacheDecl is an cache route response policy declaration.
type CacheDecl struct {
	Policy string
	Span   source.SourceSpan
}

// RevalidateDecl is an revalidate stale-while-revalidate declaration.
type RevalidateDecl struct {
	Seconds string
	Span    source.SourceSpan
}

// ErrorPageDecl is a route-local generated error page path.
type ErrorPageDecl struct {
	Path string
	Span source.SourceSpan
}

// LayoutRef is one layout reference on a page.
type LayoutRef struct {
	ID   string
	Span source.SourceSpan
}

// GuardRef is one guard reference on a page.
type GuardRef struct {
	Name string
	Span source.SourceSpan
}

// AssetRef is one source-selected asset reference.
type AssetRef struct {
	Kind   string
	Path   string
	Inline string
	Scope  AssetScope
	Span   source.SourceSpan
}

// AssetScope records deterministic owner metadata for scoped CSS and future
// content-hashed component assets.
type AssetScope struct {
	OwnerKind string
	OwnerID   string
	Package   string
	ScopeID   string
	HashKey   string
}

// Import is one top-level Go import declaration.
type Import struct {
	Alias string
	Path  string
	Span  source.SourceSpan
}

// Use is one top-level GOWDK package import declaration.
type Use struct {
	Alias   string
	Package string
	Span    source.SourceSpan
}

// GoTypeRef references a Go type through a .gwdk import alias.
type GoTypeRef struct {
	Alias string
	Name  string
	Span  source.SourceSpan
}

// GoFuncRef references a Go function through a .gwdk import alias.
type GoFuncRef struct {
	Alias string
	Name  string
	Span  source.SourceSpan
}

// Store is one top-level page-scoped store declaration.
type Store struct {
	Name string
	Type GoTypeRef
	Init GoFuncRef
	// Persist is the optional `persist "<scope>"` modifier. It is empty when the
	// store is not persisted, otherwise the raw (unquoted) scope literal. The
	// scope value is validated later so an invalid literal still parses into a
	// store and yields a precise diagnostic rather than a generic parse error.
	Persist string
	// PersistSet reports whether a `persist` clause was present, distinguishing an
	// absent clause from an explicit empty scope (`persist ""`). Without it the
	// latter is indistinguishable from no persistence and would silently parse as
	// unpersisted instead of yielding page_store_persist_scope_invalid.
	PersistSet bool
	Span       source.SourceSpan
}

// StateContract describes a component state type and initializer.
type StateContract struct {
	Type GoTypeRef
	Init GoFuncRef
	Span source.SourceSpan
}

// WASMContract points an explicit browser-side Go package at a component.
type WASMContract struct {
	Package string
	Span    source.SourceSpan
}

// Block is one parsed top-level block.
type Block struct {
	Kind      string
	Name      string
	Body      string
	Span      source.SourceSpan
	BodyStart source.SourcePosition
	View      []view.Node
	StyleBody string
	Records   []LiteralRecord
	Call      *BuildCall
	Props     []Prop
	Exports   []Export
	Emits     []Emit
	Actions   []ActionStatement
	APIs      []APIStatement
}

// Endpoint is one exact action or API endpoint declaration.
type Endpoint struct {
	Kind          string
	Name          string
	Method        string
	Route         string
	ErrorPage     string
	Span          source.SourceSpan
	ErrorPageSpan source.SourceSpan
}

// FragmentEndpoint is one generated server fragment route declaration.
type FragmentEndpoint struct {
	Name       string
	Method     string
	Route      string
	Target     string
	Body       string
	Span       source.SourceSpan
	RouteSpan  source.SourceSpan
	TargetSpan source.SourceSpan
}

// LiteralRecord is a first-slice paths/build return record.
type LiteralRecord struct {
	Fields map[string]string
	Span   source.SourceSpan
}

// BuildCall is a first-slice imported build data function call.
type BuildCall struct {
	Alias    string
	Function string
	Span     source.SourceSpan
}

// Prop is one scalar prop declaration inside props {}.
type Prop struct {
	Name string
	Type string
	Span source.SourceSpan
}

// Export is one typed public component export inside exports {}.
type Export struct {
	Name string
	Type string
	Span source.SourceSpan
}

// Emit is one component event declaration inside emits {}.
type Emit struct {
	Name   string
	Params []EmitParam
	Span   source.SourceSpan
}

// EmitParam is one typed event payload field.
type EmitParam struct {
	Name string
	Type string
	Span source.SourceSpan
}

// ActionStatement is one supported statement inside legacy act {} parsing.
type ActionStatement struct {
	Kind      string
	Name      string
	InputName string
	InputType string
	Target    string
	Redirect  string
	Body      string
	Span      source.SourceSpan
}

// APIStatement is one supported statement inside legacy api {} parsing.
type APIStatement struct {
	Method string
	Route  string
	Span   source.SourceSpan
}
