package appgen

import (
	"fmt"
	"strings"
)

func validateActionEndpoints(endpoints []ActionEndpoint) error {
	seen := map[string]ActionEndpoint{}
	for _, endpoint := range endpoints {
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
		if err := validateInputFields(endpoint); err != nil {
			return err
		}
		if err := validateRequiredFields(endpoint); err != nil {
			return err
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

func validateActionFragments(endpoint ActionEndpoint) error {
	seen := map[string]bool{}
	for _, fragment := range endpoint.Fragments {
		target := strings.TrimSpace(fragment.Target)
		if target == "" {
			return fmt.Errorf("generated action %s.%s declares an empty fragment target", endpoint.PageID, endpoint.ActionName)
		}
		if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n{}") {
			return fmt.Errorf("generated action %s.%s fragment target %q must be a literal id selector", endpoint.PageID, endpoint.ActionName, fragment.Target)
		}
		if seen[target] {
			return fmt.Errorf("generated action %s.%s declares duplicate fragment target %q", endpoint.PageID, endpoint.ActionName, target)
		}
		seen[target] = true
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

func validateActionEndpointPath(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("endpoint path %q must be an absolute path", value)
	}
	if strings.ContainsAny(value, "?#{}") {
		return fmt.Errorf("endpoint path %q must be a concrete path without query, fragment, or params", value)
	}
	return nil
}

func validateActionRedirect(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("redirect %q must be a local absolute path", value)
	}
	if strings.HasPrefix(value, "//") {
		return fmt.Errorf("redirect %q must not be protocol-relative", value)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("redirect %q must not contain newlines", value)
	}
	return nil
}
