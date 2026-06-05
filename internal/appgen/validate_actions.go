package appgen

import (
	"fmt"
	"strings"
)

func validateActionRoutes(routes []ActionRoute) error {
	seen := map[string]ActionRoute{}
	for _, route := range routes {
		if strings.TrimSpace(route.ActionName) == "" {
			return fmt.Errorf("generated action route for page %q is missing action name", route.PageID)
		}
		if err := validateActionRoutePath(route.Route); err != nil {
			return fmt.Errorf("generated action %s.%s: %w", route.PageID, route.ActionName, err)
		}
		if strings.TrimSpace(route.Redirect) != "" {
			if err := validateActionRedirect(route.Redirect); err != nil {
				return fmt.Errorf("generated action %s.%s: %w", route.PageID, route.ActionName, err)
			}
		}
		if err := validateInputFields(route); err != nil {
			return err
		}
		if err := validateRequiredFields(route); err != nil {
			return err
		}
		if err := validateActionFragments(route); err != nil {
			return err
		}
		if previous, exists := seen[route.Route]; exists {
			return fmt.Errorf("generated action %s.%s route %q duplicates action %s.%s", route.PageID, route.ActionName, route.Route, previous.PageID, previous.ActionName)
		}
		seen[route.Route] = route
	}
	return nil
}

func validateActionFragments(route ActionRoute) error {
	seen := map[string]bool{}
	for _, fragment := range route.Fragments {
		target := strings.TrimSpace(fragment.Target)
		if target == "" {
			return fmt.Errorf("generated action %s.%s declares an empty fragment target", route.PageID, route.ActionName)
		}
		if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n{}") {
			return fmt.Errorf("generated action %s.%s fragment target %q must be a literal id selector", route.PageID, route.ActionName, fragment.Target)
		}
		if seen[target] {
			return fmt.Errorf("generated action %s.%s declares duplicate fragment target %q", route.PageID, route.ActionName, target)
		}
		seen[target] = true
	}
	return nil
}

func validateInputFields(route ActionRoute) error {
	seen := map[string]bool{}
	for _, field := range route.InputFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty input field", route.PageID, route.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate input field %q", route.PageID, route.ActionName, field)
		}
		if strings.ContainsAny(field, "{}") {
			return fmt.Errorf("generated action %s.%s input field %q must be literal", route.PageID, route.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateRequiredFields(route ActionRoute) error {
	expected := map[string]bool{}
	for _, field := range route.InputFields {
		expected[field] = true
	}
	seen := map[string]bool{}
	for _, field := range route.RequiredFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty required field", route.PageID, route.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate required field %q", route.PageID, route.ActionName, field)
		}
		if !expected[field] {
			return fmt.Errorf("generated action %s.%s required field %q is not an expected input field", route.PageID, route.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateActionRoutePath(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("route %q must be an absolute path", value)
	}
	if strings.ContainsAny(value, "?#{}") {
		return fmt.Errorf("route %q must be a concrete path without query, fragment, or params", value)
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
