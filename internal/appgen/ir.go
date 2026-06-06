package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func actionEndpointsFromIR(ir gwdkir.Program) ([]ActionEndpoint, error) {
	bindings := irBindingsByEndpoint(ir.Endpoints)
	var endpoints []ActionEndpoint
	for _, page := range ir.Pages {
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
			fragments, err := actionFragmentsFromIR(action)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", page.ID, action.Name, err)
			}
			endpoints = append(endpoints, ActionEndpoint{
				PageID:         page.ID,
				ActionName:     action.Name,
				Method:         method,
				Route:          route,
				Guards:         append([]string(nil), page.Guards...),
				InputName:      action.InputName,
				InputType:      action.InputType,
				InputFields:    actionInputFields(fieldsByAction[action.Name]),
				RequiredFields: actionRequiredFields(fieldsByAction[action.Name]),
				ValidatesInput: action.ValidatesInput,
				Redirect:       action.Redirect,
				Fragments:      fragments,
				Binding:        bindings[irEndpointKey(gwdkir.EndpointAction, page.ID, action.Name, method, route)],
			})
		}
	}
	if err := validateActionEndpoints(endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func apiEndpointsFromIR(ir gwdkir.Program) ([]APIEndpoint, error) {
	bindings := irBindingsByEndpoint(ir.Endpoints)
	var endpoints []APIEndpoint
	for _, page := range ir.Pages {
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
				Guards:  append([]string(nil), page.Guards...),
				Binding: bindings[irEndpointKey(gwdkir.EndpointAPI, page.ID, api.Name, method, route)],
			})
		}
	}
	return endpoints, nil
}

func irBindingsByEndpoint(endpoints []gwdkir.Endpoint) map[string]manifest.BackendBinding {
	out := map[string]manifest.BackendBinding{}
	for _, endpoint := range endpoints {
		if endpoint.Binding.Status == "" && endpoint.Binding.ImportPath == "" && endpoint.Binding.FunctionName == "" {
			continue
		}
		kind := "action"
		if endpoint.Kind == gwdkir.EndpointAPI {
			kind = "api"
		}
		out[irEndpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)] = manifest.BackendBinding{
			Kind:         kind,
			PageID:       endpoint.PageID,
			Source:       endpoint.SourceFile,
			BlockName:    endpoint.Symbol,
			Method:       endpoint.Method,
			Route:        endpoint.Path,
			ImportPath:   endpoint.Binding.ImportPath,
			PackageName:  endpoint.Binding.PackageName,
			FunctionName: endpoint.Binding.FunctionName,
			Signature:    endpoint.Binding.Signature,
			InputType:    endpoint.Binding.InputType,
			InputPointer: endpoint.Binding.InputPointer,
			InputFields:  append([]manifest.BackendInputField(nil), endpoint.Binding.InputFields...),
			Status:       endpoint.Binding.Status,
			Message:      endpoint.Binding.Message,
		}
	}
	return out
}

func irEndpointKey(kind gwdkir.EndpointKind, pageID, symbol, method, route string) string {
	return strings.Join([]string{string(kind), pageID, symbol, method, route}, "\x00")
}

func actionFragmentsFromIR(action gwdkir.Action) ([]ActionFragment, error) {
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
