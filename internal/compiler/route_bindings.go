package compiler

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// RouteKind describes route behavior in the CLI routes report.
type RouteKind string

const (
	RouteStatic RouteKind = "static"
	RouteSPA    RouteKind = "spa"
	RouteSSR    RouteKind = "ssr"
	RouteHybrid RouteKind = "hybrid"
)

// EndpointKind describes backend endpoint behavior separate from page/file
// routes.
type EndpointKind string

const (
	EndpointAction   EndpointKind = "action"
	EndpointAPI      EndpointKind = "api"
	EndpointFragment EndpointKind = "fragment"
	EndpointCommand  EndpointKind = "command"
	EndpointQuery    EndpointKind = "query"
)

// RouteMetadata is route and endpoint metadata used by the CLI routes report.
type RouteMetadata struct {
	Routes    []RouteBinding
	Endpoints []EndpointBinding
	Info      []RouteInfo
}

// RouteBinding is route-level metadata. Route kinds are intentionally limited
// to static files, SPA routes, SSR routes, and hybrid routes.
type RouteBinding struct {
	Kind          RouteKind
	Method        string
	Route         string
	PageID        string
	Package       string
	Render        gowdk.RenderMode
	Cache         string
	DynamicParams []string
	RouteParams   []source.RouteParam
	Layouts       []string
	Guards        []string
	Source        string
	SourceSpan    source.SourceSpan
	Handler       string
}

// EndpointBinding is backend action/API metadata. Endpoints are not route
// kinds; they hang off the generated app/runtime backend layer.
type EndpointBinding struct {
	Kind              EndpointKind
	EndpointSource    string
	Source            string
	SourceSpan        source.SourceSpan
	Package           string
	PackagePath       string
	PackageName       string
	Symbol            string
	Method            string
	Route             string
	Cache             string
	DynamicParams     []string
	RouteParams       []source.RouteParam
	Guards            []string
	CSRF              bool
	PageID            string
	Handler           string
	BindingStatus     source.BackendBindingStatus
	BindingMessage    string
	BindingImportPath string
	BindingPackage    string
	BindingFunction   string
	BindingSignature  source.BackendSignatureKind
	BindingInputType  string
	Contract          ContractEndpointBinding
}

// ContractEndpointBinding describes a command/query contract exposed through a
// generated backend endpoint.
type ContractEndpointBinding struct {
	Name              string
	Kind              gwdkir.ContractKind
	Status            gwdkir.ContractBindingStatus
	Message           string
	ImportAlias       string
	ImportPath        string
	Type              string
	Result            string
	Roles             []string
	Handler           string
	Register          string
	DeclarationSource string
	DeclarationSpan   source.SourceSpan
}

// RouteInfo is non-fatal route metadata surfaced by CLI inspection commands.
type RouteInfo struct {
	Code    string
	PageID  string
	Route   string
	Message string
}

// BuildRouteMetadataFromIR converts stable compiler IR into CLI route and
// endpoint metadata.
func BuildRouteMetadataFromIR(config gowdk.Config, ir gwdkir.Program) RouteMetadata {
	var routes []RouteBinding
	var endpoints []EndpointBinding
	var info []RouteInfo
	for _, route := range ir.Routes {
		switch route.Kind {
		case gwdkir.RouteSSR:
			routes = append(routes, RouteBinding{
				Kind:          RouteSSR,
				Method:        route.Method,
				Route:         route.Path,
				PageID:        route.PageID,
				Package:       route.Package,
				Render:        route.Render,
				Cache:         route.Cache,
				DynamicParams: append([]string(nil), route.DynamicParams...),
				RouteParams:   append([]source.RouteParam(nil), route.RouteParams...),
				Layouts:       append([]string(nil), route.Layouts...),
				Guards:        append([]string(nil), route.Guards...),
				Source:        route.Source,
				SourceSpan:    route.Span,
				Handler:       "ssr.Render" + exportedRouteName(route.PageID),
			})
			info = append(info, RouteInfo{
				Code:    "spa_disabled",
				PageID:  route.PageID,
				Route:   route.Path,
				Message: fmt.Sprintf("%s uses request-time page behavior; generated SPA/static page output is disabled for this route", route.PageID),
			})
		case gwdkir.RouteHybrid:
			routes = append(routes, RouteBinding{
				Kind:          RouteHybrid,
				Method:        route.Method,
				Route:         route.Path,
				PageID:        route.PageID,
				Package:       route.Package,
				Render:        route.Render,
				Cache:         route.Cache,
				DynamicParams: append([]string(nil), route.DynamicParams...),
				RouteParams:   append([]source.RouteParam(nil), route.RouteParams...),
				Layouts:       append([]string(nil), route.Layouts...),
				Guards:        append([]string(nil), route.Guards...),
				Source:        route.Source,
				SourceSpan:    route.Span,
				Handler:       "hybrid.Render" + exportedRouteName(route.PageID),
			})
		default:
			routes = append(routes, RouteBinding{
				Kind:          RouteSPA,
				Method:        route.Method,
				Route:         route.Path,
				PageID:        route.PageID,
				Package:       route.Package,
				Render:        route.Render,
				Cache:         route.Cache,
				DynamicParams: append([]string(nil), route.DynamicParams...),
				RouteParams:   append([]source.RouteParam(nil), route.RouteParams...),
				Layouts:       append([]string(nil), route.Layouts...),
				Guards:        append([]string(nil), route.Guards...),
				Source:        route.Source,
				SourceSpan:    route.Span,
				Handler:       fmt.Sprintf(`embedded.SPA("pages/%s.html")`, routeAssetName(route.PageID)),
			})
			info = append(info, RouteInfo{
				Code:    "ssr_disabled",
				PageID:  route.PageID,
				Route:   route.Path,
				Message: fmt.Sprintf("%s uses build-time page output; request-time page rendering is disabled for this route", route.PageID),
			})
		}
	}

	for _, endpoint := range ir.Endpoints {
		binding := endpoint.Binding
		switch endpoint.Kind {
		case gwdkir.EndpointAction:
			endpoints = append(endpoints, EndpointBinding{
				Kind:              EndpointAction,
				EndpointSource:    string(endpoint.Source),
				Source:            endpoint.SourceFile,
				SourceSpan:        endpoint.Span,
				Package:           endpoint.Package,
				PackagePath:       binding.ImportPath,
				PackageName:       binding.PackageName,
				Symbol:            endpoint.Symbol,
				Method:            endpoint.Method,
				Route:             endpoint.Path,
				Cache:             endpoint.Cache,
				DynamicParams:     append([]string(nil), endpoint.DynamicParams...),
				RouteParams:       append([]source.RouteParam(nil), endpoint.RouteParams...),
				Guards:            append([]string(nil), endpoint.Guards...),
				CSRF:              endpoint.CSRF,
				PageID:            endpoint.PageID,
				Handler:           "actions." + exportedRouteName(endpoint.PageID) + exportedRouteName(endpoint.Symbol),
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingPackage:    binding.PackageName,
				BindingFunction:   binding.FunctionName,
				BindingSignature:  binding.Signature,
				BindingInputType:  binding.InputType,
			})
		case gwdkir.EndpointAPI:
			handlerName := exportedRouteName(endpoint.PageID)
			if endpoint.Symbol != "" {
				handlerName += exportedRouteName(endpoint.Symbol)
			}
			endpoints = append(endpoints, EndpointBinding{
				Kind:              EndpointAPI,
				EndpointSource:    string(endpoint.Source),
				Source:            endpoint.SourceFile,
				SourceSpan:        endpoint.Span,
				Package:           endpoint.Package,
				PackagePath:       binding.ImportPath,
				PackageName:       binding.PackageName,
				Symbol:            endpoint.Symbol,
				Method:            endpoint.Method,
				Route:             endpoint.Path,
				Cache:             endpoint.Cache,
				DynamicParams:     append([]string(nil), endpoint.DynamicParams...),
				RouteParams:       append([]source.RouteParam(nil), endpoint.RouteParams...),
				Guards:            append([]string(nil), endpoint.Guards...),
				CSRF:              endpoint.CSRF,
				PageID:            endpoint.PageID,
				Handler:           "api." + handlerName,
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingPackage:    binding.PackageName,
				BindingFunction:   binding.FunctionName,
				BindingSignature:  binding.Signature,
				BindingInputType:  binding.InputType,
			})
		case gwdkir.EndpointFragment:
			endpoints = append(endpoints, EndpointBinding{
				Kind:              EndpointFragment,
				EndpointSource:    string(endpoint.Source),
				Source:            endpoint.SourceFile,
				SourceSpan:        endpoint.Span,
				Package:           endpoint.Package,
				PackagePath:       binding.ImportPath,
				PackageName:       binding.PackageName,
				Symbol:            endpoint.Symbol,
				Method:            endpoint.Method,
				Route:             endpoint.Path,
				Cache:             endpoint.Cache,
				DynamicParams:     append([]string(nil), endpoint.DynamicParams...),
				RouteParams:       append([]source.RouteParam(nil), endpoint.RouteParams...),
				Guards:            append([]string(nil), endpoint.Guards...),
				CSRF:              endpoint.CSRF,
				PageID:            endpoint.PageID,
				Handler:           "fragments." + exportedRouteName(endpoint.PageID) + exportedRouteName(endpoint.Symbol),
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingPackage:    binding.PackageName,
				BindingFunction:   binding.FunctionName,
				BindingSignature:  binding.Signature,
			})
		}
	}
	for _, ref := range ir.ContractRefs {
		if strings.TrimSpace(ref.Method) == "" || strings.TrimSpace(ref.Path) == "" {
			continue
		}
		kind := EndpointCommand
		if ref.Kind == gwdkir.ContractQuery {
			kind = EndpointQuery
		}
		endpoints = append(endpoints, EndpointBinding{
			Kind:           kind,
			EndpointSource: "contract",
			Source:         ref.Source,
			SourceSpan:     ref.Span,
			Package:        ref.Package,
			PackagePath:    ref.ImportPath,
			Symbol:         ref.Name,
			Method:         ref.Method,
			Route:          ref.Path,
			Cache:          "no-store",
			Guards:         append([]string(nil), ref.Guards...),
			CSRF:           config.Build.CSRF.EnabledForGeneratedEndpoints() && ref.Kind == gwdkir.ContractCommand,
			PageID:         ref.OwnerID,
			Handler:        "contracts." + string(ref.Kind) + "." + ref.Name,
			Contract: ContractEndpointBinding{
				Name:              ref.Name,
				Kind:              ref.Kind,
				Status:            contractBindingStatus(ref.Status),
				Message:           ref.Message,
				ImportAlias:       ref.ImportAlias,
				ImportPath:        ref.ImportPath,
				Type:              ref.Type,
				Result:            ref.Result,
				Roles:             append([]string(nil), ref.Roles...),
				Handler:           ref.Handler,
				Register:          ref.Register,
				DeclarationSource: ref.DeclarationSource,
				DeclarationSpan:   ref.DeclarationSpan,
			},
		})
	}

	return RouteMetadata{Routes: routes, Endpoints: endpoints, Info: info}
}

func contractBindingStatus(status gwdkir.ContractBindingStatus) gwdkir.ContractBindingStatus {
	if status == "" {
		return gwdkir.ContractBindingUnknown
	}
	return status
}

func routeAssetName(pageID string) string {
	return strings.ReplaceAll(pageID, ".", "/")
}

func exportedRouteName(value string) string {
	out := make([]rune, 0, len(value))
	upperNext := true
	for _, r := range value {
		if r == '.' || r == '-' || r == '_' || r == '/' || r == '{' || r == '}' {
			upperNext = true
			continue
		}
		if upperNext {
			out = append(out, unicode.ToUpper(r))
			upperNext = false
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
