package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// ValidateBackendBindingPolicy enforces build-mode rules for declared backend
// endpoints after same-package Go handler binding metadata has been produced.
func ValidateBackendBindingPolicy(config gowdk.Config, app manifest.Manifest) error {
	if config.Build.Mode != gowdk.Production || config.Build.AllowMissingBackend {
		return nil
	}
	if len(app.BackendBindings) == 0 && manifestDeclaresBackendEndpoints(app) {
		app = BindBackendHandlers(app)
	}

	var diagnostics []ValidationError
	for _, binding := range app.BackendBindings {
		switch binding.Status {
		case manifest.BackendBindingMissing, manifest.BackendBindingUnsupportedSignature:
			diagnostics = append(diagnostics, backendBindingRequiredDiagnostic(binding))
		}
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}

func manifestDeclaresBackendEndpoints(app manifest.Manifest) bool {
	for _, page := range app.Pages {
		if len(page.Blocks.Actions) > 0 || len(page.Blocks.APIs) > 0 || page.Blocks.Load {
			return true
		}
	}
	return false
}

func backendBindingRequiredDiagnostic(binding manifest.BackendBinding) ValidationError {
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
		Message: message,
	}
}
