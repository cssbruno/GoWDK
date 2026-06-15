package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
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

// ValidateRealtimeSubscriptionBindings converts linked realtime subscription
// metadata into compiler diagnostics for CLI validation paths.
func ValidateRealtimeSubscriptionBindings(subscriptions []gwdkir.RealtimeSubscription) error {
	var diagnostics []ValidationError
	for _, subscription := range subscriptions {
		switch subscription.Status {
		case gwdkir.ContractBindingMissing:
			diagnostics = append(diagnostics, realtimeSubscriptionDiagnostic(subscription, "realtime_subscription_missing"))
		case gwdkir.ContractBindingInvalid:
			diagnostics = append(diagnostics, realtimeSubscriptionDiagnostic(subscription, "realtime_subscription_invalid"))
		case gwdkir.ContractBindingBound:
			if !realtimeSubscriptionAllowsWeb(subscription) {
				diagnostics = append(diagnostics, realtimeSubscriptionRoleDiagnostic(subscription))
			}
		}
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return normalizeValidationErrors(diagnostics)
}

// ValidateQueryInvalidations converts linked query-invalidation metadata into
// compiler diagnostics for CLI validation paths.
func ValidateQueryInvalidations(config gowdk.Config, invalidations []gwdkir.QueryInvalidation) error {
	if len(invalidations) == 0 {
		return nil
	}
	var diagnostics []ValidationError
	if !config.HasFeature(gowdk.FeatureRealtime) {
		for _, invalidation := range invalidations {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "missing_realtime_addon",
				PageID:        queryInvalidationPageID(invalidation),
				ComponentName: queryInvalidationComponentName(invalidation),
				Source:        invalidation.Source,
				Span:          invalidation.Span,
				Message:       fmt.Sprintf("query invalidation for %s requires realtime.Addon() in gowdk.config.go", invalidation.Query),
			})
		}
	}
	for _, invalidation := range invalidations {
		switch invalidation.Status {
		case gwdkir.ContractBindingMissing:
			diagnostics = append(diagnostics, queryInvalidationDiagnostic(invalidation, "query_invalidation_missing"))
		case gwdkir.ContractBindingInvalid:
			diagnostics = append(diagnostics, queryInvalidationDiagnostic(invalidation, "query_invalidation_invalid"))
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

func realtimeSubscriptionAllowsWeb(subscription gwdkir.RealtimeSubscription) bool {
	if len(subscription.Roles) == 0 {
		return true
	}
	for _, role := range subscription.Roles {
		if role == "web" {
			return true
		}
	}
	return false
}

func validateRealtimeSubscriptions(config gowdk.Config, subscriptions []gwdkir.RealtimeSubscription) []ValidationError {
	if len(subscriptions) == 0 || config.HasFeature(gowdk.FeatureRealtime) {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "missing_realtime_addon",
			PageID:        contractSubscriptionPageID(subscription),
			ComponentName: contractSubscriptionComponentName(subscription),
			Source:        subscription.Source,
			Span:          subscription.Span,
			Message:       fmt.Sprintf("g:subscribe %s requires realtime.Addon() in gowdk.config.go", subscription.Event),
		})
	}
	return diagnostics
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

func realtimeSubscriptionRoleDiagnostic(subscription gwdkir.RealtimeSubscription) ValidationError {
	return ValidationError{
		Code:          "realtime_subscription_role_not_allowed",
		PageID:        contractSubscriptionPageID(subscription),
		ComponentName: contractSubscriptionComponentName(subscription),
		Source:        subscription.Source,
		Span:          subscription.Span,
		Message:       fmt.Sprintf("presentation event %s is registered for roles %s, but generated web subscriptions execute with role web", subscription.Event, strings.Join(subscription.Roles, ", ")),
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

func realtimeSubscriptionDiagnostic(subscription gwdkir.RealtimeSubscription, code string) ValidationError {
	message := subscription.Message
	if message == "" {
		message = fmt.Sprintf("presentation event %s is not bound to a valid Go registration", subscription.Event)
	}
	return ValidationError{
		Code:          code,
		PageID:        contractSubscriptionPageID(subscription),
		ComponentName: contractSubscriptionComponentName(subscription),
		Source:        subscription.Source,
		Span:          subscription.Span,
		Message:       message,
	}
}

func queryInvalidationDiagnostic(invalidation gwdkir.QueryInvalidation, code string) ValidationError {
	message := invalidation.Message
	if message == "" {
		message = fmt.Sprintf("query invalidation %s from %s is not bound to valid Go registrations", invalidation.Query, invalidation.Event)
	}
	return ValidationError{
		Code:          code,
		PageID:        queryInvalidationPageID(invalidation),
		ComponentName: queryInvalidationComponentName(invalidation),
		Source:        invalidation.Source,
		Span:          invalidation.Span,
		Message:       message,
	}
}

func contractReferencePageID(ref gwdkir.ContractReference) string {
	if ref.OwnerKind == gwdkir.SourcePage {
		return ref.OwnerID
	}
	return ""
}

func contractSubscriptionPageID(subscription gwdkir.RealtimeSubscription) string {
	if subscription.OwnerKind == gwdkir.SourcePage {
		return subscription.OwnerID
	}
	return ""
}

func queryInvalidationPageID(invalidation gwdkir.QueryInvalidation) string {
	if invalidation.OwnerKind == gwdkir.SourcePage {
		return invalidation.OwnerID
	}
	return ""
}

func contractReferenceComponentName(ref gwdkir.ContractReference) string {
	if ref.OwnerKind == gwdkir.SourceComponent {
		return ref.OwnerID
	}
	return ""
}

func contractSubscriptionComponentName(subscription gwdkir.RealtimeSubscription) string {
	if subscription.OwnerKind == gwdkir.SourceComponent {
		return subscription.OwnerID
	}
	return ""
}

func queryInvalidationComponentName(invalidation gwdkir.QueryInvalidation) string {
	if invalidation.OwnerKind == gwdkir.SourceComponent {
		return invalidation.OwnerID
	}
	return ""
}
