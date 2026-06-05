package codegen

import (
	"fmt"
	"go/format"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// RouteKind describes generated binary route behavior.
type RouteKind string

const (
	RouteSPA    RouteKind = "spa"
	RouteAction RouteKind = "action"
	RouteSSR    RouteKind = "ssr"
	RouteAPI    RouteKind = "api"
)

var goIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// RouteBinding is the route-level codegen plan used by the generated binary.
type RouteBinding struct {
	Kind              RouteKind
	Method            string
	Route             string
	PageID            string
	Handler           string
	BindingStatus     manifest.BackendBindingStatus
	BindingMessage    string
	BindingImportPath string
	BindingFunction   string
}

// RouteRegistrationOptions configures generated route registration source.
type RouteRegistrationOptions struct {
	PackageName string
	Imports     map[string]string
}

// BuildRouteBindings converts a validated manifest into generated route
// behavior. It does not emit Go source yet; it gives codegen a typed plan.
func BuildRouteBindings(config gowdk.Config, app manifest.Manifest) ([]RouteBinding, error) {
	if err := compiler.ValidateManifest(config, app); err != nil {
		return nil, err
	}
	if len(app.BackendBindings) == 0 {
		app = compiler.BindBackendHandlers(app)
	}
	backendBindings := backendBindingsByBlock(app.BackendBindings)

	var routes []RouteBinding
	for _, page := range app.Pages {
		mode := page.RenderMode(config.Render.DefaultMode())
		if mode == gowdk.SSR || mode == gowdk.Hybrid {
			routes = append(routes, RouteBinding{
				Kind:    RouteSSR,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: "ssr.Render" + exportedName(page.ID),
			})
		} else {
			routes = append(routes, RouteBinding{
				Kind:    RouteSPA,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: fmt.Sprintf(`embedded.SPA("pages/%s.html")`, assetName(page.ID)),
			})
		}

		for _, action := range page.Blocks.Actions {
			binding := backendBindings[backendBindingKey(actionHandlerKind, page.ID, action.Name, "POST", page.Route)]
			routes = append(routes, RouteBinding{
				Kind:              RouteAction,
				Method:            "POST",
				Route:             page.Route,
				PageID:            page.ID,
				Handler:           "actions." + exportedName(page.ID) + exportedName(action.Name),
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingFunction:   binding.FunctionName,
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
			handlerName := exportedName(page.ID)
			if api.Name != "" {
				handlerName += exportedName(api.Name)
			}
			binding := backendBindings[backendBindingKey(apiHandlerKind, page.ID, api.Name, method, route)]
			routes = append(routes, RouteBinding{
				Kind:              RouteAPI,
				Method:            method,
				Route:             route,
				PageID:            page.ID,
				Handler:           "api." + handlerName,
				BindingStatus:     binding.Status,
				BindingMessage:    binding.Message,
				BindingImportPath: binding.ImportPath,
				BindingFunction:   binding.FunctionName,
			})
		}
	}

	return routes, nil
}

const (
	actionHandlerKind = "action"
	apiHandlerKind    = "api"
)

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

// GenerateRouteRegistration emits Go source that registers route bindings on an
// http.ServeMux. Handler package aliases used by bindings must be provided in
// options.Imports so the generated file is complete and formatted.
func GenerateRouteRegistration(bindings []RouteBinding, options RouteRegistrationOptions) ([]byte, error) {
	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "routes"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid route registration package name %q", packageName)
	}

	imports, err := routeRegistrationImports(bindings, options.Imports)
	if err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	source.WriteString("import (\n")
	source.WriteString("\t\"net/http\"\n")
	for _, item := range imports {
		source.WriteString("\t")
		source.WriteString(item.alias)
		source.WriteByte(' ')
		source.WriteString(strconv.Quote(item.path))
		source.WriteByte('\n')
	}
	source.WriteString(")\n\n")
	source.WriteString("func Register(mux *http.ServeMux) {\n")
	for _, binding := range bindings {
		pattern := strings.TrimSpace(binding.Method + " " + binding.Route)
		if strings.TrimSpace(binding.Method) == "" || strings.TrimSpace(binding.Route) == "" {
			return nil, fmt.Errorf("route binding for page %q must include method and route", binding.PageID)
		}
		if strings.TrimSpace(binding.Handler) == "" {
			return nil, fmt.Errorf("route binding %s must include handler", pattern)
		}
		source.WriteString("\tmux.HandleFunc(")
		source.WriteString(strconv.Quote(pattern))
		source.WriteString(", ")
		source.WriteString(binding.Handler)
		source.WriteString(")\n")
	}
	source.WriteString("}\n")

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format route registration source: %w", err)
	}
	return formatted, nil
}

type routeImport struct {
	alias string
	path  string
}

func routeRegistrationImports(bindings []RouteBinding, configured map[string]string) ([]routeImport, error) {
	needed := map[string]bool{}
	for _, binding := range bindings {
		alias := handlerPackageAlias(binding.Handler)
		if alias != "" {
			needed[alias] = true
		}
	}
	aliases := make([]string, 0, len(needed))
	for alias := range needed {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	imports := make([]routeImport, 0, len(aliases))
	for _, alias := range aliases {
		path := strings.TrimSpace(configured[alias])
		if path == "" {
			return nil, fmt.Errorf("missing import path for route handler package %q", alias)
		}
		imports = append(imports, routeImport{alias: alias, path: path})
	}
	return imports, nil
}

func handlerPackageAlias(handler string) string {
	handler = strings.TrimSpace(handler)
	dot := strings.IndexByte(handler, '.')
	if dot <= 0 {
		return ""
	}
	alias := handler[:dot]
	if !goIdentifierPattern.MatchString(alias) {
		return ""
	}
	return alias
}

func assetName(pageID string) string {
	return strings.ReplaceAll(pageID, ".", "/")
}

func exportedName(value string) string {
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
