package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
)

func actionEndpointsFromIR(ir gwdkir.Program) ([]ActionEndpoint, error) {
	bindings := irBindingsByEndpoint(ir.Endpoints)
	var endpoints []ActionEndpoint
	for _, page := range ir.Pages {
		fieldsByAction, err := actionFormSchemaFromBlocks(page.Blocks)
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
				PageID:           page.ID,
				ActionName:       action.Name,
				Method:           method,
				Route:            route,
				Guards:           append([]string(nil), page.Guards...),
				InputName:        action.InputName,
				InputType:        action.InputType,
				InputFields:      actionInputFields(fieldsByAction[action.Name]),
				RequiredFields:   actionRequiredFields(fieldsByAction[action.Name]),
				RequiredMessages: actionRequiredMessages(fieldsByAction[action.Name]),
				ValidationRules:  actionValidationRules(fieldsByAction[action.Name]),
				ValidatesInput:   action.ValidatesInput,
				Redirect:         action.Redirect,
				Fragments:        fragments,
				ErrorPage:        action.ErrorPage,
				Binding:          bindings[irEndpointKey(gwdkir.EndpointAction, page.ID, action.Name, method, route)],
			})
		}
	}
	for _, endpoint := range ir.Endpoints {
		if endpoint.Kind != gwdkir.EndpointAction || endpoint.Source != gwdkir.EndpointSourceGo {
			continue
		}
		binding := bindings[irEndpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)]
		endpoints = append(endpoints, ActionEndpoint{
			PageID:      endpoint.PageID,
			ActionName:  endpoint.Symbol,
			Method:      endpoint.Method,
			Route:       endpoint.Path,
			ErrorPage:   endpoint.ErrorPage,
			InputFields: bindingInputFieldNames(binding.InputFields),
			Binding:     binding,
		})
	}
	if err := validateActionEndpoints(endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func actionFormSchemaFromBlocks(blocks gwdkir.Blocks) (map[string][]view.ActionFormField, error) {
	if len(blocks.ViewNodes) > 0 {
		return view.ActionFormSchemaFromNodes(blocks.ViewNodes)
	}
	return view.ActionFormSchema(blocks.ViewBody)
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
				PageID:    page.ID,
				APIName:   api.Name,
				Method:    method,
				Route:     route,
				Guards:    append([]string(nil), page.Guards...),
				ErrorPage: api.ErrorPage,
				Binding:   bindings[irEndpointKey(gwdkir.EndpointAPI, page.ID, api.Name, method, route)],
			})
		}
	}
	for _, endpoint := range ir.Endpoints {
		if endpoint.Kind != gwdkir.EndpointAPI || endpoint.Source != gwdkir.EndpointSourceGo {
			continue
		}
		endpoints = append(endpoints, APIEndpoint{
			PageID:    endpoint.PageID,
			APIName:   endpoint.Symbol,
			Method:    endpoint.Method,
			Route:     endpoint.Path,
			ErrorPage: endpoint.ErrorPage,
			Binding:   bindings[irEndpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)],
		})
	}
	if err := validateAPIEndpoints(endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func fragmentEndpointsFromIR(ir gwdkir.Program) ([]FragmentEndpoint, error) {
	var endpoints []FragmentEndpoint
	bindings := irBindingsByEndpoint(ir.Endpoints)
	components := fragmentComponentsFromIR(ir.Components)
	for _, page := range ir.Pages {
		for _, fragment := range page.Blocks.Fragments {
			uses := irUsesMap(page.Uses)
			html, err := renderFragmentHTML(fragment.Body, page.Package, uses, components)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: fragment %s: %w", page.ID, fragment.Name, fragment.Target, err)
			}
			method := strings.TrimSpace(fragment.Method)
			if method == "" {
				method = "GET"
			}
			endpoints = append(endpoints, FragmentEndpoint{
				PageID:       page.ID,
				FragmentName: fragment.Name,
				Method:       method,
				Route:        strings.TrimSpace(fragment.Route),
				RouteParams:  gwdkir.RouteParamsFromPath(strings.TrimSpace(fragment.Route)),
				Target:       fragment.Target,
				HTML:         html,
				Package:      page.Package,
				Uses:         uses,
				Guards:       append([]string(nil), page.Guards...),
				Binding:      bindings[irEndpointKey(gwdkir.EndpointFragment, page.ID, fragment.Name, method, strings.TrimSpace(fragment.Route))],
			})
		}
	}
	if err := validateFragmentEndpoints(endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func renderFragmentHTML(body string, packageName string, uses map[string]string, components map[string]view.Component) (string, error) {
	return view.RenderWithOptions(body, componentRegistryForFragment(packageName, uses, components), nil, view.Options{
		Package: packageName,
		Uses:    uses,
	})
}

func fragmentComponentsFromIR(components []gwdkir.Component) map[string]view.Component {
	out := map[string]view.Component{}
	for _, component := range components {
		compiled := view.Component{
			Name:         component.Name,
			Package:      component.Package,
			Uses:         irUsesMap(component.Uses),
			Props:        irPropNames(component.Props),
			PropTypes:    irPropTypes(component.Props),
			PropDefaults: irPropDefaults(component.Props),
			Exports:      irExportTypes(component.Exports),
			Body:         component.Blocks.ViewBody,
			Nodes:        append([]view.Node(nil), component.Blocks.ViewNodes...),
		}
		addFragmentComponent(out, compiled)
	}
	return out
}

func addFragmentComponent(registry map[string]view.Component, component view.Component) {
	if component.Name == "" || component.Body == "" {
		return
	}
	registry[fragmentComponentKey(component.Package, component.Name)] = component
	if component.Package == "" {
		registry[component.Name] = component
	}
}

func componentRegistryForFragment(packageName string, uses map[string]string, registry map[string]view.Component) map[string]view.Component {
	if packageName == "" && len(uses) == 0 {
		return registry
	}
	out := map[string]view.Component{}
	for key, component := range registry {
		out[key] = component
		if component.Package == packageName {
			out[component.Name] = component
		}
	}
	for alias, usePackage := range uses {
		for _, component := range registry {
			if component.Package == usePackage {
				out[alias+"."+component.Name] = component
			}
		}
	}
	return out
}

func fragmentComponentKey(packageName string, componentName string) string {
	if packageName == "" {
		return componentName
	}
	return packageName + "." + componentName
}

func irUsesMap(uses []gwdkir.Use) map[string]string {
	if len(uses) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, use := range uses {
		out[use.Alias] = use.Package
	}
	return out
}

func irPropNames(props []gwdkir.Prop) []string {
	out := make([]string, 0, len(props))
	for _, prop := range props {
		out = append(out, prop.Name)
	}
	return out
}

func irPropTypes(props []gwdkir.Prop) map[string]clientlang.ValueType {
	if len(props) == 0 {
		return nil
	}
	out := map[string]clientlang.ValueType{}
	for _, prop := range props {
		out[prop.Name] = clientlang.NormalizeType(prop.Type)
	}
	return out
}

func irPropDefaults(props []gwdkir.Prop) map[string]string {
	out := map[string]string{}
	for _, prop := range props {
		if prop.DefaultSet {
			out[prop.Name] = prop.Default
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func irExportTypes(exports []gwdkir.Export) map[string]clientlang.ValueType {
	if len(exports) == 0 {
		return nil
	}
	out := map[string]clientlang.ValueType{}
	for _, export := range exports {
		out[export.Name] = clientlang.NormalizeType(export.Type)
	}
	return out
}

func irBindingsByEndpoint(endpoints []gwdkir.Endpoint) map[string]source.BackendBinding {
	out := map[string]source.BackendBinding{}
	for _, endpoint := range endpoints {
		if endpoint.Binding.Status == "" && endpoint.Binding.ImportPath == "" && endpoint.Binding.FunctionName == "" {
			continue
		}
		kind := "action"
		if endpoint.Kind == gwdkir.EndpointAPI {
			kind = "api"
		}
		out[irEndpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)] = source.BackendBinding{
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
			InputFields:  append([]source.BackendInputField(nil), endpoint.Binding.InputFields...),
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

func bindingInputFieldNames(fields []source.BackendInputField) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.FormName)
	}
	return out
}
