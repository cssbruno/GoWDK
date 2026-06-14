package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// ValidateContractReferences converts linked contract-reference metadata into
// compiler diagnostics for CLI validation paths.
func ValidateContractReferences(refs []gwdkir.ContractReference) error {
	var diagnostics []ValidationError
	for _, ref := range refs {
		switch ref.Status {
		case gwdkir.ContractBindingMissing:
			diagnostics = append(diagnostics, contractReferenceDiagnostic(ref, "contract_reference_missing"))
		case gwdkir.ContractBindingInvalid:
			diagnostics = append(diagnostics, contractReferenceDiagnostic(ref, "contract_reference_invalid"))
		case gwdkir.ContractBindingBound:
			if !contractReferenceAllowsWeb(ref) {
				diagnostics = append(diagnostics, contractReferenceRoleDiagnostic(ref))
			}
		}
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return normalizeValidationErrors(diagnostics)
}

func contractReferenceAllowsWeb(ref gwdkir.ContractReference) bool {
	if len(ref.Roles) == 0 {
		return true
	}
	for _, role := range ref.Roles {
		if role == "web" {
			return true
		}
	}
	return false
}

func validateContractReferenceRoutes(refs []gwdkir.ContractReference) []ValidationError {
	var diagnostics []ValidationError
	for _, ref := range refs {
		method := source.BackendRouteMethod(ref.Method)
		if strings.TrimSpace(ref.Method) != "" && method != contractReferenceRouteMethod(ref.Kind) {
			diagnostics = append(diagnostics, contractReferenceRouteDiagnostic(ref, fmt.Sprintf("%s %s route method %q is invalid; %s contract routes require %s", ref.Kind, ref.Name, ref.Method, ref.Kind, contractReferenceRouteMethod(ref.Kind))))
		}
		if strings.TrimSpace(ref.Path) != "" {
			if err := source.ValidateBackendRoutePath(ref.Path); err != nil {
				diagnostics = append(diagnostics, contractReferenceRouteDiagnostic(ref, fmt.Sprintf("%s %s route path is invalid: %v", ref.Kind, ref.Name, err)))
			}
		}
	}
	return diagnostics
}

func contractReferenceRouteMethod(kind gwdkir.ContractKind) string {
	if kind == gwdkir.ContractQuery {
		return "GET"
	}
	return "POST"
}

func contractReferenceRouteDiagnostic(ref gwdkir.ContractReference, message string) ValidationError {
	diagnostic := contractReferenceDiagnostic(ref, "contract_route_invalid")
	diagnostic.Message = message
	return diagnostic
}

func contractReferenceRoleDiagnostic(ref gwdkir.ContractReference) ValidationError {
	return ValidationError{
		Code:          "contract_reference_role_not_allowed",
		PageID:        contractReferencePageID(ref),
		ComponentName: contractReferenceComponentName(ref),
		Source:        ref.Source,
		Span:          ref.Span,
		Message:       fmt.Sprintf("%s %s is registered for roles %s, but generated web routes execute with role web", ref.Kind, ref.Name, strings.Join(ref.Roles, ", ")),
	}
}

func contractReferenceDiagnostic(ref gwdkir.ContractReference, code string) ValidationError {
	message := ref.Message
	if message == "" {
		message = fmt.Sprintf("%s %s is not bound to a valid Go registration", ref.Kind, ref.Name)
	}
	return ValidationError{
		Code:          code,
		PageID:        contractReferencePageID(ref),
		ComponentName: contractReferenceComponentName(ref),
		Source:        ref.Source,
		Span:          ref.Span,
		Message:       message,
	}
}

func contractReferencePageID(ref gwdkir.ContractReference) string {
	if ref.OwnerKind == gwdkir.SourcePage {
		return ref.OwnerID
	}
	return ""
}

func contractReferenceComponentName(ref gwdkir.ContractReference) string {
	if ref.OwnerKind == gwdkir.SourceComponent {
		return ref.OwnerID
	}
	return ""
}
