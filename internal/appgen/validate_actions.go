package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/runtime/validation"
)

func validateActionEndpoints(endpoints []ActionEndpoint) error {
	seen := map[string]ActionEndpoint{}
	for index, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.ActionName) == "" {
			return fmt.Errorf("generated action endpoint for page %q is missing action name", endpoint.PageID)
		}
		if err := validateActionEndpointPath(endpoint.Route); err != nil {
			return fmt.Errorf("generated action %s.%s: %w", endpoint.PageID, endpoint.ActionName, err)
		}
		if strings.TrimSpace(endpoint.Redirect) != "" {
			if err := validateActionRedirect(endpoint.Redirect); err != nil {
				return fmt.Errorf("generated action %s.%s: %w", endpoint.PageID, endpoint.ActionName, err)
			}
		}
		if endpoint.ErrorPage != "" {
			errorPage, err := source.ErrorPagePath(endpoint.ErrorPage)
			if err != nil {
				return fmt.Errorf("generated action %s.%s: %w", endpoint.PageID, endpoint.ActionName, err)
			}
			endpoint.ErrorPage = errorPage
			endpoints[index].ErrorPage = errorPage
		}
		if err := validateInputFields(endpoint); err != nil {
			return err
		}
		if err := validateRequiredFields(endpoint); err != nil {
			return err
		}
		if endpoint.ValidatesInput {
			if err := validateValidationRules(endpoint); err != nil {
				return err
			}
		}
		if err := validateActionFragments(endpoint); err != nil {
			return err
		}
		if previous, exists := seen[endpoint.Route]; exists {
			return fmt.Errorf("generated action %s.%s endpoint path %q duplicates action %s.%s", endpoint.PageID, endpoint.ActionName, endpoint.Route, previous.PageID, previous.ActionName)
		}
		seen[endpoint.Route] = endpoint
	}
	return nil
}

func validateAPIEndpoints(endpoints []APIEndpoint) error {
	for index, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.APIName) == "" {
			return fmt.Errorf("generated API endpoint for page %q is missing API name", endpoint.PageID)
		}
		if err := validateActionEndpointPath(endpoint.Route); err != nil {
			return fmt.Errorf("generated API %s.%s: %w", endpoint.PageID, endpoint.APIName, err)
		}
		if endpoint.ErrorPage != "" {
			errorPage, err := source.ErrorPagePath(endpoint.ErrorPage)
			if err != nil {
				return fmt.Errorf("generated API %s.%s: %w", endpoint.PageID, endpoint.APIName, err)
			}
			endpoint.ErrorPage = errorPage
			endpoints[index].ErrorPage = errorPage
		}
	}
	return nil
}

func validateActionFragments(endpoint ActionEndpoint) error {
	seen := map[string]bool{}
	for _, fragment := range endpoint.Fragments {
		if err := validateFragmentTargetValue(fragment.Target); err != nil {
			return fmt.Errorf("generated action %s.%s fragment target %q %w", endpoint.PageID, endpoint.ActionName, fragment.Target, err)
		}
		target := strings.TrimSpace(fragment.Target)
		if seen[target] {
			return fmt.Errorf("generated action %s.%s declares duplicate fragment target %q", endpoint.PageID, endpoint.ActionName, target)
		}
		seen[target] = true
	}
	return nil
}

func validateFragmentEndpoints(endpoints []FragmentEndpoint) error {
	seen := map[string]FragmentEndpoint{}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.FragmentName) == "" {
			return fmt.Errorf("generated fragment endpoint for page %q is missing fragment name", endpoint.PageID)
		}
		if endpoint.Method != "GET" {
			return fmt.Errorf("generated fragment %s.%s uses unsupported method %s; fragments currently require GET", endpoint.PageID, endpoint.FragmentName, endpoint.Method)
		}
		if err := validateActionEndpointPath(endpoint.Route); err != nil {
			return fmt.Errorf("generated fragment %s.%s: %w", endpoint.PageID, endpoint.FragmentName, err)
		}
		if err := validateFragmentTargetValue(endpoint.Target); err != nil {
			return fmt.Errorf("generated fragment %s.%s target %q %w", endpoint.PageID, endpoint.FragmentName, endpoint.Target, err)
		}
		key := endpoint.Method + "\x00" + endpoint.Route
		if previous, exists := seen[key]; exists {
			return fmt.Errorf("generated fragment %s.%s endpoint path %q duplicates fragment %s.%s", endpoint.PageID, endpoint.FragmentName, endpoint.Route, previous.PageID, previous.FragmentName)
		}
		seen[key] = endpoint
	}
	return nil
}

func validateContractRoutes(ir *gwdkir.Program) error {
	if ir == nil {
		return nil
	}
	for _, ref := range ir.ContractRefs {
		method := source.BackendRouteMethod(ref.Method)
		if strings.TrimSpace(ref.Method) != "" && method != contractRouteMethod(ref.Kind) {
			return fmt.Errorf("generated %s contract %s route method %q is invalid; %s contract routes require %s", ref.Kind, ref.Name, ref.Method, ref.Kind, contractRouteMethod(ref.Kind))
		}
		if strings.TrimSpace(ref.Path) != "" {
			if err := source.ValidateBackendRoutePath(ref.Path); err != nil {
				return fmt.Errorf("generated %s contract %s route path is invalid: %w", ref.Kind, ref.Name, err)
			}
		}
	}
	return nil
}

func contractRouteMethod(kind gwdkir.ContractKind) string {
	if kind == gwdkir.ContractQuery {
		return "GET"
	}
	return "POST"
}

func validateFragmentTargetValue(value string) error {
	target := strings.TrimSpace(value)
	if target == "" {
		return fmt.Errorf("must not be empty")
	}
	if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n{}") {
		return fmt.Errorf("must be a literal id selector")
	}
	return nil
}

func validateInputFields(endpoint ActionEndpoint) error {
	seen := map[string]bool{}
	for _, field := range endpoint.InputFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty input field", endpoint.PageID, endpoint.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate input field %q", endpoint.PageID, endpoint.ActionName, field)
		}
		if strings.ContainsAny(field, "{}") {
			return fmt.Errorf("generated action %s.%s input field %q must be literal", endpoint.PageID, endpoint.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateRequiredFields(endpoint ActionEndpoint) error {
	expected := map[string]bool{}
	for _, field := range endpoint.InputFields {
		expected[field] = true
	}
	seen := map[string]bool{}
	for _, field := range endpoint.RequiredFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty required field", endpoint.PageID, endpoint.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate required field %q", endpoint.PageID, endpoint.ActionName, field)
		}
		if !expected[field] {
			return fmt.Errorf("generated action %s.%s required field %q is not an expected input field", endpoint.PageID, endpoint.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateValidationRules(endpoint ActionEndpoint) error {
	expected := map[string]bool{}
	for _, field := range endpoint.InputFields {
		expected[field] = true
	}
	seen := map[string]bool{}
	for _, rule := range endpoint.ValidationRules {
		field := strings.TrimSpace(rule.Field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty validation field", endpoint.PageID, endpoint.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate validation rules for field %q", endpoint.PageID, endpoint.ActionName, field)
		}
		if !expected[field] {
			return fmt.Errorf("generated action %s.%s validation field %q is not an expected input field", endpoint.PageID, endpoint.ActionName, field)
		}
		if rule.MinLength < 0 || rule.MaxLength < 0 {
			return fmt.Errorf("generated action %s.%s validation field %q has a negative length constraint", endpoint.PageID, endpoint.ActionName, field)
		}
		if rule.MinLength == 0 && rule.MaxLength == 0 && strings.TrimSpace(rule.Pattern) == "" {
			return fmt.Errorf("generated action %s.%s validation field %q has no constraints", endpoint.PageID, endpoint.ActionName, field)
		}
		if rule.MinLength > 0 && rule.MaxLength > 0 && rule.MinLength > rule.MaxLength {
			return fmt.Errorf("generated action %s.%s validation field %q minlength exceeds maxlength", endpoint.PageID, endpoint.ActionName, field)
		}
		if strings.TrimSpace(rule.Pattern) != "" {
			if err := validation.ValidatePattern(rule.Pattern); err != nil {
				return fmt.Errorf("generated action %s.%s validation field %q has invalid pattern: %w", endpoint.PageID, endpoint.ActionName, field, err)
			}
		}
		seen[field] = true
	}
	return nil
}

func validateActionEndpointPath(value string) error {
	return source.ValidateBackendRoutePath(value)
}

func validateActionRedirect(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("redirect %q must be a local absolute path", value)
	}
	if strings.HasPrefix(value, "//") {
		return fmt.Errorf("redirect %q must not be protocol-relative", value)
	}
	// Browsers normalize "\" to "/" before navigating, so "/\evil.com" is
	// treated like the protocol-relative "//evil.com".
	if strings.Contains(value, "\\") {
		return fmt.Errorf("redirect %q must not contain backslashes", value)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("redirect %q must not contain newlines", value)
	}
	return nil
}
