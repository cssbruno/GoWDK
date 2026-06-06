// Package gwdkast defines the typed syntax tree for .gwdk source files.
package gwdkast

import (
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

// File is the typed AST for the currently supported .gwdk syntax subset.
type File struct {
	Package     *Package
	Annotations []Annotation
	Imports     []Import
	Uses        []Use
	Stores      []Store
	PropsType   *GoTypeRef
	State       *StateContract
	WASM        *WASMContract
	Blocks      []Block
	Actions     []Endpoint
	APIs        []Endpoint
}

// Package is the top-level Go package declaration.
type Package struct {
	Name string
	Span manifest.SourceSpan
}

// Annotation is one top-level @annotation.
type Annotation struct {
	Name  string
	Value string
	Span  manifest.SourceSpan
}

// Import is one top-level Go import declaration.
type Import struct {
	Alias string
	Path  string
	Span  manifest.SourceSpan
}

// Use is one top-level GOWDK package import declaration.
type Use struct {
	Alias   string
	Package string
	Span    manifest.SourceSpan
}

// GoTypeRef references a Go type through a .gwdk import alias.
type GoTypeRef struct {
	Alias string
	Name  string
	Span  manifest.SourceSpan
}

// GoFuncRef references a Go function through a .gwdk import alias.
type GoFuncRef struct {
	Alias string
	Name  string
	Span  manifest.SourceSpan
}

// Store is one top-level page-scoped store declaration.
type Store struct {
	Name string
	Type GoTypeRef
	Init GoFuncRef
	Span manifest.SourceSpan
}

// StateContract describes a component state type and initializer.
type StateContract struct {
	Type GoTypeRef
	Init GoFuncRef
	Span manifest.SourceSpan
}

// WASMContract points an explicit browser-side Go package at a component.
type WASMContract struct {
	Package string
	Span    manifest.SourceSpan
}

// Block is one parsed top-level block.
type Block struct {
	Kind    string
	Name    string
	Body    string
	Span    manifest.SourceSpan
	View    []view.Node
	Records []LiteralRecord
	Call    *BuildCall
	Props   []Prop
	Emits   []Emit
	Actions []ActionStatement
	APIs    []APIStatement
}

// Endpoint is one exact action or API endpoint declaration.
type Endpoint struct {
	Kind   string
	Name   string
	Method string
	Route  string
	Span   manifest.SourceSpan
}

// LiteralRecord is a first-slice paths/build return record.
type LiteralRecord struct {
	Fields map[string]string
	Span   manifest.SourceSpan
}

// BuildCall is a first-slice imported build data function call.
type BuildCall struct {
	Alias    string
	Function string
	Span     manifest.SourceSpan
}

// Prop is one scalar prop declaration inside props {}.
type Prop struct {
	Name string
	Type string
	Span manifest.SourceSpan
}

// Emit is one component event declaration inside emits {}.
type Emit struct {
	Name   string
	Params []EmitParam
	Span   manifest.SourceSpan
}

// EmitParam is one typed event payload field.
type EmitParam struct {
	Name string
	Type string
	Span manifest.SourceSpan
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
	Span      manifest.SourceSpan
}

// APIStatement is one supported statement inside legacy api {} parsing.
type APIStatement struct {
	Method string
	Route  string
	Span   manifest.SourceSpan
}
