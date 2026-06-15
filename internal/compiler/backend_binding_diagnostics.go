package compiler

import (
	"github.com/cssbruno/gowdk/internal/source"
)

// BackendBindingDiagnostics returns non-fatal diagnostics that explain why a
// declared backend handler did not bind to a Go function, so the reason is
// visible during gowdk check and build instead of only in inspect go-bindings
// or a strict production build.
//
// It intentionally warns only when there is positive evidence of a problem the
// author almost certainly cares about:
//
//   - ambiguous_backend_handler: the same handler is declared in both
//     same-package Go and an inline go {} block, so the chosen source is
//     ambiguous.
//   - unsupported_backend_signature: a same-named Go function exists but has a
//     signature GOWDK does not support.
//   - unexported_backend_handler: a same-named Go function exists but is not
//     exported, so binding cannot see it.
//
// A plainly missing handler (no candidate function at all) is left silent
// because the default workflow generates 501 stubs for not-yet-implemented
// handlers, and strict production builds already fail via
// backend_binding_required.
func BackendBindingDiagnostics(bindings []source.BackendBinding) []ValidationError {
	var diagnostics []ValidationError
	for _, binding := range bindings {
		switch {
		case binding.Ambiguous:
			diagnostics = append(diagnostics, backendBindingDiagnostic("ambiguous_backend_handler", binding))
		case binding.Status == source.BackendBindingUnsupportedSignature:
			diagnostics = append(diagnostics, backendBindingDiagnostic("unsupported_backend_signature", binding))
		case binding.Status == source.BackendBindingMissing && binding.UnexportedCandidate:
			diagnostics = append(diagnostics, backendBindingDiagnostic("unexported_backend_handler", binding))
		}
	}
	return normalizeValidationErrors(diagnostics)
}

func backendBindingDiagnostic(code string, binding source.BackendBinding) ValidationError {
	return ValidationError{
		Code:     code,
		PageID:   binding.PageID,
		Source:   binding.Source,
		Span:     binding.Span,
		Message:  binding.Message,
		Severity: SeverityWarning,
	}
}
