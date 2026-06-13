package compiler

import (
	"github.com/cssbruno/gowdk/internal/source"
)

// BackendBindingDiagnostics returns non-fatal diagnostics that explain why a
// declared backend handler did not bind to a Go function, so the reason is
// visible during gowdk check and build instead of only in inspect go-bindings
// or a strict production build.
//
// It intentionally warns only when there is positive evidence of a near-miss
// the author almost certainly meant to bind:
//
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
		case binding.Status == source.BackendBindingUnsupportedSignature:
			diagnostics = append(diagnostics, backendBindingDiagnostic("unsupported_backend_signature", binding))
		case binding.Status == source.BackendBindingMissing && binding.UnexportedCandidate:
			diagnostics = append(diagnostics, backendBindingDiagnostic("unexported_backend_handler", binding))
		}
	}
	return diagnostics
}

func backendBindingDiagnostic(code string, binding source.BackendBinding) ValidationError {
	return ValidationError{
		Code:     code,
		PageID:   binding.PageID,
		Source:   binding.Source,
		Message:  binding.Message,
		Severity: SeverityWarning,
	}
}
