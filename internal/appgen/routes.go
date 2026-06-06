package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

// ActionEndpoints extracts generated action endpoint dispatch entries from a
// parsed manifest.
func ActionEndpoints(app manifest.Manifest) ([]ActionEndpoint, error) {
	var endpoints []ActionEndpoint
	bindings := backendBindingsByBlock(app.BackendBindings)
	for _, page := range app.Pages {
		fieldsByAction, err := view.ActionFormSchema(page.Blocks.ViewBody)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		for _, action := range page.Blocks.Actions {
			method := strings.TrimSpace(action.Method)
			if method == "" {
				method = "POST"
			}
			route := strings.TrimSpace(action.Route)
			if route == "" {
				route = page.Route
			}
			fragments, err := actionFragments(action)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", page.ID, action.Name, err)
			}
			endpoints = append(endpoints, ActionEndpoint{
				PageID:          page.ID,
				ActionName:      action.Name,
				Method:          method,
				Route:           route,
				InputName:       action.InputName,
				InputType:       action.InputType,
				InputFields:     actionInputFields(fieldsByAction[action.Name]),
				RequiredFields:  actionRequiredFields(fieldsByAction[action.Name]),
				ValidationRules: actionValidationRules(fieldsByAction[action.Name]),
				ValidatesInput:  action.ValidatesInput,
				Redirect:        action.Redirect,
				Fragments:       fragments,
				Binding:         bindings[backendBindingKey("action", page.ID, action.Name, method, route)],
			})
		}
	}
	if err := validateActionEndpoints(endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

// APIEndpoints extracts generated API endpoint dispatch entries from a parsed
// manifest.
func APIEndpoints(app manifest.Manifest) ([]APIEndpoint, error) {
	var endpoints []APIEndpoint
	bindings := backendBindingsByBlock(app.BackendBindings)
	for _, page := range app.Pages {
		for _, api := range page.Blocks.APIs {
			method := strings.TrimSpace(api.Method)
			if method == "" {
				method = "GET"
			}
			route := strings.TrimSpace(api.Route)
			if route == "" {
				route = page.Route
			}
			endpoints = append(endpoints, APIEndpoint{
				PageID:  page.ID,
				APIName: api.Name,
				Method:  method,
				Route:   route,
				Binding: bindings[backendBindingKey("api", page.ID, api.Name, method, route)],
			})
		}
	}
	return endpoints, nil
}

func backendBindingsByBlock(bindings []manifest.BackendBinding) map[string]manifest.BackendBinding {
	out := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		out[backendBindingKey(binding.Kind, binding.PageID, binding.BlockName, binding.Method, binding.Route)] = binding
	}
	return out
}

func backendBindingKey(kind, pageID, blockName, method, route string) string {
	return strings.Join([]string{kind, pageID, blockName, method, route}, "\x00")
}

func actionFragments(action manifest.Action) ([]ActionFragment, error) {
	if len(action.Fragments) == 0 {
		return nil, nil
	}
	fragments := make([]ActionFragment, 0, len(action.Fragments))
	for _, fragment := range action.Fragments {
		html, err := view.RenderSPA(fragment.Body)
		if err != nil {
			return nil, fmt.Errorf("fragment %s: %w", fragment.Target, err)
		}
		fragments = append(fragments, ActionFragment{Target: fragment.Target, HTML: html})
	}
	return fragments, nil
}

func actionInputFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func actionRequiredFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Required {
			names = append(names, field.Name)
		}
	}
	return names
}

func actionValidationRules(fields []view.ActionFormField) []ActionValidationRule {
	rules := make([]ActionValidationRule, 0, len(fields))
	for _, field := range fields {
		if field.MinLength == 0 && field.MaxLength == 0 && field.Pattern == "" {
			continue
		}
		rules = append(rules, ActionValidationRule{
			Field:     field.Name,
			MinLength: field.MinLength,
			MaxLength: field.MaxLength,
			Pattern:   field.Pattern,
		})
	}
	return rules
}
