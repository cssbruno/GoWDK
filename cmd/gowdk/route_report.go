package main

import (
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/source"
)

type routeMetadataReport struct {
	Version   int                   `json:"version"`
	Routes    []routeBindingJSON    `json:"routes"`
	Endpoints []endpointBindingJSON `json:"endpoints,omitempty"`
	Info      []routeInfoJSON       `json:"info,omitempty"`
}

type endpointMetadataReport struct {
	Version   int                   `json:"version"`
	Endpoints []endpointBindingJSON `json:"endpoints"`
}

type routeBindingJSON struct {
	Kind          compiler.RouteKind `json:"kind"`
	Method        string             `json:"method"`
	Route         string             `json:"route"`
	PageID        string             `json:"pageId"`
	Package       string             `json:"package,omitempty"`
	Render        string             `json:"render,omitempty"`
	Cache         string             `json:"cache,omitempty"`
	DynamicParams []string           `json:"dynamicParams,omitempty"`
	RouteParams   []routeParamJSON   `json:"routeParams,omitempty"`
	Layouts       []string           `json:"layouts,omitempty"`
	Guards        []string           `json:"guards,omitempty"`
	Source        string             `json:"source,omitempty"`
	SourceSpan    *sourceSpanJSON    `json:"sourceSpan,omitempty"`
	Handler       string             `json:"handler"`
}

type routeParamJSON struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type endpointBindingJSON struct {
	Kind           compiler.EndpointKind `json:"kind"`
	EndpointSource string                `json:"endpointSource,omitempty"`
	Source         string                `json:"source,omitempty"`
	SourceSpan     *sourceSpanJSON       `json:"sourceSpan,omitempty"`
	Package        string                `json:"package,omitempty"`
	PackagePath    string                `json:"packagePath,omitempty"`
	PackageName    string                `json:"packageName,omitempty"`
	Symbol         string                `json:"symbol,omitempty"`
	Method         string                `json:"method"`
	Route          string                `json:"route"`
	Cache          string                `json:"cache,omitempty"`
	Guards         []string              `json:"guards,omitempty"`
	CSRF           bool                  `json:"csrf,omitempty"`
	PageID         string                `json:"pageId"`
	Handler        string                `json:"handler"`
	BindingStatus  string                `json:"bindingStatus,omitempty"`
	Signature      string                `json:"signature,omitempty"`
	InputType      string                `json:"inputType,omitempty"`
	BackendBinding *backendBindingJSON   `json:"backendBinding,omitempty"`
	Contract       *contractBindingJSON  `json:"contract,omitempty"`
}

type sourceSpanJSON struct {
	Start sourcePositionJSON `json:"start"`
	End   sourcePositionJSON `json:"end"`
}

type sourcePositionJSON struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type backendBindingJSON struct {
	Status       string `json:"status"`
	PackageName  string `json:"packageName,omitempty"`
	ImportPath   string `json:"importPath,omitempty"`
	FunctionName string `json:"functionName,omitempty"`
	Signature    string `json:"signature,omitempty"`
	InputType    string `json:"inputType,omitempty"`
	Message      string `json:"message,omitempty"`
}

type contractBindingJSON struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Status      string   `json:"status"`
	Message     string   `json:"message,omitempty"`
	ImportAlias string   `json:"importAlias,omitempty"`
	ImportPath  string   `json:"importPath,omitempty"`
	Type        string   `json:"type,omitempty"`
	Result      string   `json:"result,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Handler     string   `json:"handler,omitempty"`
	Register    string   `json:"register,omitempty"`
}

type routeInfoJSON struct {
	Code    string `json:"code"`
	PageID  string `json:"pageId"`
	Route   string `json:"route"`
	Message string `json:"message"`
}

func routeMetadataJSON(metadata compiler.RouteMetadata) routeMetadataReport {
	routes := make([]routeBindingJSON, 0, len(metadata.Routes))
	for _, binding := range metadata.Routes {
		routes = append(routes, routeBindingJSON{
			Kind:          binding.Kind,
			Method:        binding.Method,
			Route:         binding.Route,
			PageID:        binding.PageID,
			Package:       binding.Package,
			Render:        string(binding.Render),
			Cache:         binding.Cache,
			DynamicParams: append([]string(nil), binding.DynamicParams...),
			RouteParams:   routeParamsJSON(binding.RouteParams),
			Layouts:       append([]string(nil), binding.Layouts...),
			Guards:        append([]string(nil), binding.Guards...),
			Source:        binding.Source,
			SourceSpan:    endpointSourceSpanJSON(binding.SourceSpan),
			Handler:       binding.Handler,
		})
	}
	endpoints := endpointsJSON(metadata.Endpoints)
	info := make([]routeInfoJSON, 0, len(metadata.Info))
	for _, item := range metadata.Info {
		info = append(info, routeInfoJSON{
			Code:    item.Code,
			PageID:  item.PageID,
			Route:   item.Route,
			Message: item.Message,
		})
	}
	return routeMetadataReport{
		Version:   1,
		Routes:    routes,
		Endpoints: endpoints,
		Info:      info,
	}
}

func endpointMetadataJSON(metadata compiler.RouteMetadata) endpointMetadataReport {
	return endpointMetadataReport{
		Version:   1,
		Endpoints: endpointsJSON(metadata.Endpoints),
	}
}

func endpointsJSON(bindings []compiler.EndpointBinding) []endpointBindingJSON {
	endpoints := make([]endpointBindingJSON, 0, len(bindings))
	for _, binding := range bindings {
		item := endpointBindingJSON{
			Kind:           binding.Kind,
			EndpointSource: binding.EndpointSource,
			Source:         binding.Source,
			SourceSpan:     endpointSourceSpanJSON(binding.SourceSpan),
			Package:        binding.Package,
			PackagePath:    binding.PackagePath,
			PackageName:    binding.PackageName,
			Symbol:         binding.Symbol,
			Method:         binding.Method,
			Route:          binding.Route,
			Cache:          binding.Cache,
			Guards:         append([]string(nil), binding.Guards...),
			CSRF:           binding.CSRF,
			PageID:         binding.PageID,
			Handler:        binding.Handler,
			BindingStatus:  string(binding.BindingStatus),
			Signature:      string(binding.BindingSignature),
			InputType:      binding.BindingInputType,
		}
		if binding.BindingStatus != "" {
			item.BackendBinding = &backendBindingJSON{
				Status:       string(binding.BindingStatus),
				PackageName:  binding.BindingPackage,
				ImportPath:   binding.BindingImportPath,
				FunctionName: binding.BindingFunction,
				Signature:    string(binding.BindingSignature),
				InputType:    binding.BindingInputType,
				Message:      binding.BindingMessage,
			}
		}
		if binding.Contract.Name != "" {
			item.Contract = &contractBindingJSON{
				Name:        binding.Contract.Name,
				Kind:        string(binding.Contract.Kind),
				Status:      string(binding.Contract.Status),
				Message:     binding.Contract.Message,
				ImportAlias: binding.Contract.ImportAlias,
				ImportPath:  binding.Contract.ImportPath,
				Type:        binding.Contract.Type,
				Result:      binding.Contract.Result,
				Roles:       append([]string(nil), binding.Contract.Roles...),
				Handler:     binding.Contract.Handler,
				Register:    binding.Contract.Register,
			}
		}
		endpoints = append(endpoints, item)
	}
	return endpoints
}

func routeParamsJSON(params []source.RouteParam) []routeParamJSON {
	if len(params) == 0 {
		return nil
	}
	out := make([]routeParamJSON, 0, len(params))
	for _, param := range params {
		out = append(out, routeParamJSON{Name: param.Name, Type: param.Type})
	}
	return out
}

func endpointSourceSpanJSON(span source.SourceSpan) *sourceSpanJSON {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return &sourceSpanJSON{
		Start: sourcePositionJSON{Line: span.Start.Line, Column: span.Start.Column},
		End:   sourcePositionJSON{Line: span.End.Line, Column: span.End.Column},
	}
}
