package compiler

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk"
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
	EndpointAction EndpointKind = "action"
	EndpointAPI    EndpointKind = "api"
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
	backendBindings := routeBackendBindingsByBlock(app.BackendBindings)

	var routes []RouteBinding
	var endpoints []EndpointBinding
	var info []RouteInfo
	for _, page := range app.Pages {
		mode := page.RenderMode(config.Render.DefaultMode())
		switch mode {
		case gowdk.SSR:
			routes = append(routes, RouteBinding{
				Kind:    RouteSSR,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: "ssr.Render" + exportedRouteName(page.ID),
			})
			info = append(info, RouteInfo{
				Code:    "spa_disabled",
				PageID:  page.ID,
				Route:   page.Route,
				Message: fmt.Sprintf("%s uses @render ssr; generated SPA/static page output is disabled for this route", page.ID),
			})
		case gowdk.Hybrid:
			routes = append(routes, RouteBinding{
				Kind:    RouteHybrid,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: "hybrid.Render" + exportedRouteName(page.ID),
			})
		default:
			routes = append(routes, RouteBinding{
				Kind:    RouteSPA,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: fmt.Sprintf(`embedded.SPA("pages/%s.html")`, routeAssetName(page.ID)),
			})
			info = append(info, RouteInfo{
				Code:    "ssr_disabled",
				PageID:  page.ID,
				Route:   page.Route,
				Message: fmt.Sprintf("%s uses @render %s; request-time SSR is disabled for this route", page.ID, mode),
			})
		}

		for _, action := range page.Blocks.Actions {
			method := action.Method
			if method == "" {
				method = "POST"
			}
			route := action.Route
			if route == "" {
				route = page.Route
			}
			binding := backendBindings[routeBackendBindingKey(actionHandlerKind, page.ID, action.Name, method, route)]
			endpoints = append(endpoints, EndpointBinding{
				Kind:              EndpointAction,
				EndpointSource:    "gwdk",
				Source:            page.Source,
				SourceSpan:        action.Span,
				Package:           page.Package,
				PackagePath:       binding.ImportPath,
				PackageName:       binding.PackageName,
				Symbol:            action.Name,
				Method:            method,
				Route:             route,
				PageID:            page.ID,
				Handler:           "actions." + exportedRouteName(page.ID) + exportedRouteName(action.Name),
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingPackage:    binding.PackageName,
				BindingFunction:   binding.FunctionName,
				BindingSignature:  binding.Signature,
				BindingInputType:  binding.InputType,
			})
		}

		for _, api := range page.Blocks.APIs {
			method := api.Method
			if method == "" {
				method = "GET"
			}
			route := api.Route
			if route == "" {
				route = page.Route
			}
			handlerName := exportedRouteName(page.ID)
			if api.Name != "" {
				handlerName += exportedRouteName(api.Name)
			}
			binding := backendBindings[routeBackendBindingKey(apiHandlerKind, page.ID, api.Name, method, route)]
			endpoints = append(endpoints, EndpointBinding{
				Kind:              EndpointAPI,
				EndpointSource:    "gwdk",
				Source:            page.Source,
				SourceSpan:        api.Span,
				Package:           page.Package,
				PackagePath:       binding.ImportPath,
				PackageName:       binding.PackageName,
				Symbol:            api.Name,
				Method:            method,
				Route:             route,
				PageID:            page.ID,
				Handler:           "api." + handlerName,
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingPackage:    binding.PackageName,
				BindingFunction:   binding.FunctionName,
				BindingSignature:  binding.Signature,
				BindingInputType:  binding.InputType,
			})
		}
	}

	return RouteMetadata{Routes: routes, Endpoints: endpoints, Info: info}, nil
}

func routeBackendBindingsByBlock(bindings []manifest.BackendBinding) map[string]manifest.BackendBinding {
	out := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		out[routeBackendBindingKey(binding.Kind, binding.PageID, binding.BlockName, binding.Method, binding.Route)] = binding
	}
	return out
}

func routeBackendBindingKey(kind, pageID, blockName, method, route string) string {
	return strings.Join([]string{kind, pageID, blockName, method, route}, "\x00")
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
