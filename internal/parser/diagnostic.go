package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

const (
	DiagnosticMalformedGOWDKUse            = "malformed_gowdk_use"
	DiagnosticMalformedLegacyMetadata      = "malformed_legacy_metadata"
	DiagnosticOldActionBlockSyntax         = "old_action_block_syntax"
	DiagnosticOldAPIBlockSyntax            = "old_api_block_syntax"
	DiagnosticPackageMustBeFirst           = "package_must_be_first"
	DiagnosticUnsupportedLiteralRecord     = "unsupported_literal_record_syntax"
	DiagnosticUnsupportedTopLevelBlock     = "unsupported_top_level_block"
	DiagnosticUnsupportedLayoutMetadata    = "unsupported_layout_metadata"
	DiagnosticInvalidComponentProp         = "invalid_component_prop"
	DiagnosticUnsupportedComponentPropType = "unsupported_component_prop_type"
)

// DiagnosticError carries parser diagnostic metadata without forcing callers to
// recover line numbers and codes by parsing Error strings.
type DiagnosticError struct {
	Code    string
	Span    source.SourceSpan
	Message string
}

func (err *DiagnosticError) Error() string {
	if err == nil {
		return ""
	}
	if err.Span.Start.Line > 0 {
		return fmt.Sprintf("line %d: %s", err.Span.Start.Line, err.Message)
	}
	return err.Message
}

// DiagnosticErrors carries every parser diagnostic recovered from one file.
type DiagnosticErrors []error

func (errs DiagnosticErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

func (errs DiagnosticErrors) Unwrap() []error {
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return out
}

// ParserDiagnostic extracts a typed parser diagnostic from err when available.
func ParserDiagnostic(err error) (*DiagnosticError, bool) {
	var diagnostic *DiagnosticError
	if errors.As(err, &diagnostic) && diagnostic != nil {
		return diagnostic, true
	}
	return nil, false
}

// ParserDiagnostics extracts every typed parser diagnostic from err. Untyped
// parse failures are returned as nil entries by callers through their fallback
// diagnostic path.
func ParserDiagnostics(err error) []*DiagnosticError {
	if err == nil {
		return nil
	}
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		var diagnostics []*DiagnosticError
		for _, child := range joined.Unwrap() {
			diagnostics = append(diagnostics, ParserDiagnostics(child)...)
		}
		return diagnostics
	}
	if diagnostic, ok := ParserDiagnostic(err); ok {
		return []*DiagnosticError{diagnostic}
	}
	return nil
}

func diagnosticErrors(errs []error) error {
	out := make(DiagnosticErrors, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	switch len(out) {
	case 0:
		return nil
	case 1:
		return out[0]
	default:
		return out
	}
}

func diagnosticError(code string, span source.SourceSpan, message string) error {
	return &DiagnosticError{Code: code, Span: span, Message: message}
}

func lineDiagnosticError(code string, lineNumber int, rawLine string, format string, args ...any) error {
	return diagnosticError(code, sourceLineSpan(lineNumber, rawLine), fmt.Sprintf(format, args...))
}

func withLine(lineNumber int, err error) error {
	if _, ok := ParserDiagnostic(err); ok {
		return err
	}
	return fmt.Errorf("line %d: %w", lineNumber, err)
}
