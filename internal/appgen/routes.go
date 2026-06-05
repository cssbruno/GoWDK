package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

// ActionRoutes extracts generated action routes from a parsed manifest.
func ActionRoutes(app manifest.Manifest) ([]ActionRoute, error) {
	var routes []ActionRoute
	bindings := backendBindingsByBlock(app.BackendBindings)
	for _, page := range app.Pages {
		fieldsByAction, err := view.ActionFormSchema(page.Blocks.ViewBody)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		for _, action := range page.Blocks.Actions {
			fragments, err := actionFragments(action)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", page.ID, action.Name, err)
			}
			routes = append(routes, ActionRoute{
				PageID:         page.ID,
				ActionName:     action.Name,
				Route:          page.Route,
				InputName:      action.InputName,
				InputType:      action.InputType,
				InputFields:    actionInputFields(fieldsByAction[action.Name]),
				RequiredFields: actionRequiredFields(fieldsByAction[action.Name]),
				ValidatesInput: action.ValidatesInput,
				Redirect:       action.Redirect,
				Fragments:      fragments,
				Binding:        bindings[backendBindingKey("action", page.ID, action.Name, "POST", page.Route)],
			})
		}
	}
	if err := validateActionRoutes(routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// APIRoutes extracts generated API routes from a parsed manifest.
func APIRoutes(app manifest.Manifest) ([]APIRoute, error) {
	var routes []APIRoute
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
			routes = append(routes, APIRoute{
				PageID:  page.ID,
				APIName: api.Name,
				Method:  method,
				Route:   route,
				Binding: bindings[backendBindingKey("api", page.ID, api.Name, method, route)],
			})
		}
	}
	return routes, nil
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
