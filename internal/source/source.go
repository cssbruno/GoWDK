// Package source holds the neutral leaf value types shared across the GOWDK
// compiler packages: source spans, route params, inline scripts, and backend
// binding metadata. These types carry no behavior and depend on nothing else in
// the module, so every layer (parser, AST, IR, generated output) can reference
// them without creating import cycles.
//
// These were originally part of internal/manifest, which forced packages that
// only needed a SourceSpan to depend on the whole manifest model (and made
// internal/gwdkir depend on manifest). They were extracted into this neutral
// home so the IR could reference shared leaf types directly; the manifest model
// itself has since been removed (the manifest→IR migration, #145/#173).
package source

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// SourcePosition is a 1-based source location in a parsed .gwdk file.
//
// Offset is the 0-based byte offset of the position into the source buffer. It
// is the exact substrate for AST-backed formatting, precise LSP edits, and
// exact diagnostic ranges, none of which should re-derive offsets from
// line/column. Offset is best-effort: some parser spans are still line-derived
// and leave Offset at its zero value while Line/Column are set. Use
// PositionAt/OffsetOf to convert against a source buffer when an exact offset is
// required. Set-ness of a position is determined by Line/Column being positive,
// not by Offset, because byte offset 0 is a valid first-byte position.
type SourcePosition struct {
	Line   int
	Column int
	Offset int
}

// PositionAt returns the 1-based line/column (column counted in runes, matching
// the parser's rune-column spans) for a 0-based byte offset into src, with
// Offset set to that byte offset. The offset is clamped to the buffer bounds.
func PositionAt(src []byte, offset int) SourcePosition {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	line, column := 1, 1
	for index := 0; index < offset; {
		r, size := utf8.DecodeRune(src[index:])
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
		index += size
	}
	return SourcePosition{Line: line, Column: column, Offset: offset}
}

// OffsetOf returns the 0-based byte offset into src for a 1-based line/column
// position (column counted in runes). An unset position (non-positive line or
// column) maps to 0, and a position past the end of src is clamped to len(src).
// It is the inverse of PositionAt for in-bounds, rune-aligned positions.
func OffsetOf(src []byte, pos SourcePosition) int {
	if pos.Line <= 0 || pos.Column <= 0 {
		return 0
	}
	line, column := 1, 1
	for index := 0; index < len(src); {
		if line == pos.Line && column == pos.Column {
			return index
		}
		r, size := utf8.DecodeRune(src[index:])
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
		index += size
	}
	return len(src)
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

// RelatedSpan is a secondary source location attached to a diagnostic, such as
// the first declaration that a conflict diagnostic also points at. Source is the
// owning file label (matching a diagnostic's primary Source) and may be empty
// for a same-file relation. Message is a short note shown alongside the location
// (for example "first declared here").
type RelatedSpan struct {
	Source  string
	Span    SourceSpan
	Message string
}

// RouteParam describes one dynamic route parameter and its declared scalar
// type. Empty Type means string for compatibility with legacy {name} syntax.
type RouteParam struct {
	Name string
	Type string
	Span SourceSpan
}

// SSRReplacement maps a generated placeholder back to a request route param.
type SSRReplacement struct {
	Param       string
	Placeholder string
	URL         bool
}

// SSRLoadReplacement maps a generated placeholder back to a request-time load
// field path.
type SSRLoadReplacement struct {
	Path        string
	Placeholder string
	URL         bool
}

// SSRListSpec describes one server-rendered g:for list for a request-time
// page. The runtime list renderer resolves SourcePath against load data (or, for
// nested lists, against a parent row element), then substitutes Fields and
// Children into RowTemplate once per element. It mirrors
// viewrender.SSRListReplacement and ssr.ListSpec.
type SSRListSpec struct {
	Placeholder string
	SourcePath  string
	RowTemplate string
	Fields      []SSRListField
	Lists       []SSRListSpec
	Conds       []SSRCondSpec
}

// SSRCondSpec describes one server-rendered g:if conditional for a
// request-time page. Its branch renders only when SourcePath resolves to a
// truthy value (negated when Negate is set). It mirrors
// viewrender.SSRCondReplacement and ssr.CondSpec.
type SSRCondSpec struct {
	Placeholder string
	SourcePath  string
	Negate      bool
	Expr        string
	Template    string
	Fields      []SSRListField
	Lists       []SSRListSpec
	Conds       []SSRCondSpec
}

// SSRListField is one per-render scalar substitution inside a server region.
type SSRListField struct {
	Placeholder string
	Path        string
	Index       bool
	URL         bool
}

// SSRQueryRegion is the standalone render recipe for one request-time g:query
// region: the region element's template (with placeholders) plus the subset of
// the page's g:for/g:if specs and scalar load substitutions that fall inside it.
// The app generator lowers each into a runtime region renderer keyed by
// QueryType so a g:command can render exactly the regions it invalidated inline
// in its response (single-flight) instead of forcing the client to refetch the
// whole page. Only regions renderable without route context the command request
// lacks are emitted.
type SSRQueryRegion struct {
	QueryType        string
	Template         string
	ListSpecs        []SSRListSpec
	CondSpecs        []SSRCondSpec
	LoadReplacements []SSRLoadReplacement
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

// BackendRouteMethod returns the normalized HTTP method spelling used by
// generated backend route metadata.
func BackendRouteMethod(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// BackendRoutePath returns the normalized path key used by generated backend
// routers after ValidateBackendRoutePath has accepted the source value.
func BackendRoutePath(value string) string {
	return path.Clean("/" + value)
}

// ValidateBackendRoutePath rejects paths that would be unsafe or ambiguous when
// registered as generated backend routes.
func ValidateBackendRoutePath(value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("endpoint path %q must not contain surrounding whitespace", value)
	}
	if value == "" {
		return fmt.Errorf("endpoint path must not be empty")
	}
	if value[0] != '/' {
		return fmt.Errorf("endpoint path %q must be a local absolute path", value)
	}
	if len(value) > 1 && (value[1] == '/' || value[1] == '\\') {
		return fmt.Errorf("endpoint path %q must not be protocol-relative", value)
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("endpoint path %q must not contain backslashes", value)
	}
	if strings.ContainsAny(value, "?#{}") {
		return fmt.Errorf("endpoint path %q must be a concrete path without query, fragment, or params", value)
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("endpoint path %q must not contain control characters", value)
		}
	}
	cleaned := BackendRoutePath(value)
	if cleaned != value {
		return fmt.Errorf("endpoint path %q must be a clean absolute path without dot segments, duplicate slashes, or trailing slash", value)
	}
	return nil
}

// ValidateBackendRoutePattern rejects unsafe generated route patterns while
// allowing whole-segment route params such as /patients/{id:int}.
func ValidateBackendRoutePattern(value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("endpoint path %q must not contain surrounding whitespace", value)
	}
	if value == "" {
		return fmt.Errorf("endpoint path must not be empty")
	}
	if value[0] != '/' {
		return fmt.Errorf("endpoint path %q must be a local absolute path", value)
	}
	if len(value) > 1 && (value[1] == '/' || value[1] == '\\') {
		return fmt.Errorf("endpoint path %q must not be protocol-relative", value)
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("endpoint path %q must not contain backslashes", value)
	}
	if strings.ContainsAny(value, "?#") {
		return fmt.Errorf("endpoint path %q must not contain query strings or fragments", value)
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("endpoint path %q must not contain control characters", value)
		}
	}
	cleaned := BackendRoutePath(value)
	if cleaned != value {
		return fmt.Errorf("endpoint path %q must be a clean absolute path without dot segments, duplicate slashes, or trailing slash", value)
	}
	if value == "/" {
		return nil
	}

	seen := map[string]bool{}
	segments := strings.Split(strings.TrimPrefix(value, "/"), "/")
	for index, segment := range segments {
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		param, ok := backendRouteParamSegment(segment)
		if !ok {
			return fmt.Errorf("endpoint path %q has invalid route parameter segment %q; use {name} or {name:type} as the whole segment", value, segment)
		}
		if strings.HasSuffix(param.name, "?") {
			return fmt.Errorf("endpoint path %q uses optional route parameter %q; optional route parameters are not supported", value, segment)
		}
		if param.rest && param.name == "" {
			return fmt.Errorf("endpoint path %q has rest route parameter segment %q without a name; declare it as {name...}", value, segment)
		}
		if !param.rest && strings.Contains(param.name, ".") {
			return fmt.Errorf("endpoint path %q has invalid route parameter segment %q; rest route parameters use exactly three dots, such as {name...}", value, segment)
		}
		if !isBackendRouteParamName(param.name) {
			return fmt.Errorf("endpoint path %q has invalid route parameter name %q", value, param.name)
		}
		if param.rest && param.hasType {
			return fmt.Errorf("endpoint path %q declares typed rest route parameter %q; rest route parameters are always strings", value, segment)
		}
		if !isBackendRouteParamType(param.typ) {
			return fmt.Errorf("endpoint path %q has invalid route parameter type %q for %q; supported types are string, int, int64, uint, uint64, bool, float64", value, param.typ, param.name)
		}
		if param.rest && index != len(segments)-1 {
			return fmt.Errorf("endpoint path %q declares rest route parameter {%s...} before the end of the route; rest parameters must be the last segment", value, param.name)
		}
		if seen[param.name] {
			return fmt.Errorf("endpoint path %q repeats route parameter %q", value, param.name)
		}
		seen[param.name] = true
	}
	return nil
}

type backendRouteParam struct {
	name    string
	typ     string
	rest    bool
	hasType bool
}

func backendRouteParamSegment(segment string) (backendRouteParam, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return backendRouteParam{}, false
	}
	if strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
		return backendRouteParam{}, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
	name, paramType, found := strings.Cut(value, ":")
	if !found {
		paramType = "string"
	}
	rest := strings.HasSuffix(name, "...")
	if rest {
		name = strings.TrimSuffix(name, "...")
	}
	return backendRouteParam{name: name, typ: paramType, rest: rest, hasType: found}, true
}

func isBackendRouteParamType(value string) bool {
	switch value {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64":
		return true
	default:
		return false
	}
}

func isBackendRouteParamName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !isASCIIIdentifierStart(r) {
				return false
			}
			continue
		}
		if !isASCIIIdentifierPart(r) {
			return false
		}
	}
	return true
}

func isASCIIIdentifierStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIIdentifierPart(r rune) bool {
	return isASCIIIdentifierStart(r) || (r >= '0' && r <= '9')
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
	Type      string // canonical value from LookupBackendInputFieldType
}

// BackendInputFieldTypeSupported reports whether goType is a form-decoder type
// supported by compiler validation and generated action decoders.
func BackendInputFieldTypeSupported(goType string) bool {
	_, ok := LookupBackendInputFieldType(goType)
	return ok
}

// BackendInputFieldSignedInteger reports whether goType uses the signed integer
// form decoder.
func BackendInputFieldSignedInteger(goType string) bool {
	fieldType, ok := LookupBackendInputFieldType(goType)
	return ok && fieldType.Kind == BackendInputFieldKindSignedInt
}

// BackendInputFieldUnsignedInteger reports whether goType uses the unsigned
// integer form decoder.
func BackendInputFieldUnsignedInteger(goType string) bool {
	fieldType, ok := LookupBackendInputFieldType(goType)
	return ok && fieldType.Kind == BackendInputFieldKindUnsignedInt
}

// BackendInputFieldIntegerBitSize returns the explicit bit size passed to form
// integer decoders. Zero means the platform-sized int or uint type.
func BackendInputFieldIntegerBitSize(goType string) int {
	fieldType, ok := LookupBackendInputFieldType(goType)
	if !ok {
		return 0
	}
	return fieldType.BitSize
}

// BackendBinding describes the Go handler selected for a backend block (a page
// load, act, api, or fragment block, or a standalone Go endpoint).
type BackendBinding struct {
	Kind         string
	PageID       string
	Source       string
	Span         SourceSpan
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
	// UnexportedCandidate is set when a handler is missing but a same-named
	// unexported Go function exists in the inspected package, so tooling can
	// explain that the function is present but not exported.
	UnexportedCandidate bool
	// Ambiguous is set when the same handler name is declared in more than one
	// Go source (sibling same-package code and an inline go {} block), so tooling
	// reports the conflict instead of silently preferring one.
	Ambiguous bool
}

// ErrorPagePath returns a clean generated-output-relative error page path.
func ErrorPagePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("error requires a value")
	}
	if strings.ContainsAny(value, "\\?#") {
		return "", fmt.Errorf("error must be a local generated HTML path without query, fragment, or backslash")
	}
	for _, part := range strings.Split(strings.TrimPrefix(value, "/"), "/") {
		if part == ".." {
			return "", fmt.Errorf("error path must stay inside generated output")
		}
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	if cleaned == "/" || cleaned == "/." {
		return "", fmt.Errorf("error requires a generated HTML file path")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if !strings.HasSuffix(strings.ToLower(cleaned), ".html") {
		return "", fmt.Errorf("error path must end in .html")
	}
	return cleaned, nil
}
