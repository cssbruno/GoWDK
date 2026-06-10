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

import (
	"fmt"
	"path"
	"strings"
)

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

// BackendBinding describes the Go handler selected for a backend block (a page
// load, act, api, or fragment block, or a standalone Go endpoint).
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
