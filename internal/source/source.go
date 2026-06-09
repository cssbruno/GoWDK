// Package source holds the neutral leaf value types shared across the GOWDK
// compiler packages: source spans, route params, inline scripts, and backend
// binding metadata. These types carry no behavior and depend on nothing else in
// the module, so every layer (parser, AST, IR, manifest, generated output) can
// reference them without creating import cycles or coupling to the manifest
// page/component model.
//
// Historically these lived in internal/manifest, which forced packages that
// only needed a SourceSpan to depend on the whole manifest model (and made
// internal/gwdkir depend on manifest). They were extracted here so the
// manifest model and the IR can both reference shared leaf types from a neutral
// home. manifest re-exports them as aliases for backward compatibility.
package source

import "fmt"

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

// RouteParam describes one dynamic route parameter and its declared scalar
// type. Empty Type means string for compatibility with legacy {name} syntax.
type RouteParam struct {
	Name string
	Type string
	Span SourceSpan
}

// InlineScript records browser module code declared directly inside a .gwdk
// source file. Path-based script declarations should remain preferred.
type InlineScript struct {
	Name string
	Body string
	Span SourceSpan
}

// InlineScriptName returns the deterministic generated filename for the
// zero-based inline browser script declaration index in one source owner.
func InlineScriptName(index int) string {
	if index <= 0 {
		return "inline-gowdk.js"
	}
	return fmt.Sprintf("inline-%d-gowdk.js", index+1)
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
