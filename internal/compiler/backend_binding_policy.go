package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// ValidateBackendBindingPolicyIR enforces build-mode rules for declared backend
// endpoints after same-package Go handler binding metadata has been produced.
// When the program carries no binding metadata but declares backend endpoints
// (callers that build straight from an unbound program), handlers are
// re-discovered from disk before the policy check.
func ValidateBackendBindingPolicyIR(config gowdk.Config, ir gwdkir.Program) error {
	if config.Build.Mode != gowdk.Production || config.Build.AllowMissingBackend {
		return nil
	}
	bindings := BackendBindingsFromIR(ir)
	if len(bindings) == 0 && programDeclaresBackendEndpoints(ir) {
		bindings = computeBackendBindings(ir)
	}

	var diagnostics []ValidationError
	for _, binding := range bindings {
		// A fragment never requires a Go handler: a fragment with no bound
		// handler renders static .gwdk output. Only an existing-but-unsupported
		// fragment signature is a hard production error; a missing fragment
		// handler (including an unexported near-miss) stays a non-fatal warning.
		if binding.Kind == fragmentHandlerKind && binding.Status == source.BackendBindingMissing {
			continue
		}
		switch binding.Status {
		case source.BackendBindingMissing, source.BackendBindingUnsupportedSignature:
			diagnostics = append(diagnostics, backendBindingRequiredDiagnostic(binding))
		}
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return normalizeValidationErrors(diagnostics)
}

func programDeclaresBackendEndpoints(ir gwdkir.Program) bool {
	for _, page := range ir.Pages {
		if len(page.Blocks.Actions) > 0 || len(page.Blocks.APIs) > 0 || page.Blocks.Load {
			return true
		}
	}
	return false
}

func backendBindingRequiredDiagnostic(binding source.BackendBinding) ValidationError {
	kind := binding.Kind
	if kind == "" {
		kind = "backend"
	}
	status := string(binding.Status)
	if status == "" {
		status = "missing"
	}
	message := fmt.Sprintf(
		"production build requires a bound %s handler %s for %s %s; current binding status is %s",
		kind,
		binding.FunctionName,
		binding.Method,
		binding.Route,
		status,
	)
	if binding.Message != "" {
		message += ": " + binding.Message
	}
	message += ". Fix the handler or set Build.AllowMissingBackend / pass --allow-missing-backend to generate 501 stubs."
	return ValidationError{
		Code:    "backend_binding_required",
		PageID:  binding.PageID,
		Source:  binding.Source,
		Span:    binding.Span,
		Message: message,
	}
}
