package compiler

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
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
	Kind    RouteKind
	Method  string
	Route   string
	PageID  string
	Cache   string
	Handler string
}

// EndpointBinding is backend action/API metadata. Endpoints are not route
// kinds; they hang off the generated app/runtime backend layer.
type EndpointBinding struct {
	Kind              EndpointKind
	EndpointSource    string
	Source            string
	SourceSpan        manifest.SourceSpan
	Package           string
	PackagePath       string
	PackageName       string
	Symbol            string
	Method            string
	Route             string
	PageID            string
	Handler           string
	BindingStatus     manifest.BackendBindingStatus
	BindingMessage    string
	BindingImportPath string
	BindingPackage    string
	BindingFunction   string
	BindingSignature  manifest.BackendSignatureKind
	BindingInputType  string
}

// RouteInfo is non-fatal route metadata surfaced by CLI inspection commands.
type RouteInfo struct {
	Code    string
	PageID  string
	Route   string
	Message string
}

// BuildRouteMetadata converts a validated manifest into route and endpoint
// metadata for CLI reporting.
func BuildRouteMetadata(config gowdk.Config, app manifest.Manifest) (RouteMetadata, error) {
	if err := ValidateManifest(config, app); err != nil {
		return RouteMetadata{}, err
	}
	if len(app.BackendBindings) == 0 {
		app = BindBackendHandlers(app)
	}
	ir := gwdkanalysis.BuildIR(config, app)
	return BuildRouteMetadataFromIR(config, ir), nil
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
				Kind:    RouteSSR,
				Method:  route.Method,
				Route:   route.Path,
				PageID:  route.PageID,
				Cache:   route.Cache,
				Handler: "ssr.Render" + exportedRouteName(route.PageID),
			})
			info = append(info, RouteInfo{
				Code:    "spa_disabled",
				PageID:  route.PageID,
				Route:   route.Path,
				Message: fmt.Sprintf("%s uses @render ssr; generated SPA/static page output is disabled for this route", route.PageID),
			})
		case gwdkir.RouteHybrid:
			routes = append(routes, RouteBinding{
				Kind:    RouteHybrid,
				Method:  route.Method,
				Route:   route.Path,
				PageID:  route.PageID,
				Cache:   route.Cache,
				Handler: "hybrid.Render" + exportedRouteName(route.PageID),
			})
		default:
			routes = append(routes, RouteBinding{
				Kind:    RouteSPA,
				Method:  route.Method,
				Route:   route.Path,
				PageID:  route.PageID,
				Cache:   route.Cache,
				Handler: fmt.Sprintf(`embedded.SPA("pages/%s.html")`, routeAssetName(route.PageID)),
			})
			info = append(info, RouteInfo{
				Code:    "ssr_disabled",
				PageID:  route.PageID,
				Route:   route.Path,
				Message: fmt.Sprintf("%s uses @render %s; request-time page rendering is disabled for this route", route.PageID, route.Render),
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

	return RouteMetadata{Routes: routes, Endpoints: endpoints, Info: info}
}

func routeAssetName(pageID string) string {
	return strings.ReplaceAll(pageID, ".", "/")
}

func exportedRouteName(value string) string {
	var out strings.Builder
	upperNext := true
	for _, r := range value {
		if r == '.' || r == '-' || r == '_' || r == '/' || r == '{' || r == '}' {
			upperNext = true
			continue
		}
		if upperNext {
			out.WriteRune(unicode.ToUpper(r))
			upperNext = false
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
