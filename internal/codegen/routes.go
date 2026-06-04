package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/compiler"
	"github.com/gowdk/gowdk/internal/manifest"
)

// RouteKind describes generated binary route behavior.
type RouteKind string

const (
	RouteStatic RouteKind = "static"
	RouteAction RouteKind = "action"
	RouteSSR    RouteKind = "ssr"
	RouteAPI    RouteKind = "api"
)

// RouteBinding is the route-level codegen plan used by the generated binary.
type RouteBinding struct {
	Kind    RouteKind
	Method  string
	Route   string
	PageID  string
	Handler string
}

// BuildRouteBindings converts a validated manifest into generated route
// behavior. It does not emit Go source yet; it gives codegen a typed plan.
func BuildRouteBindings(config gowdk.Config, app manifest.Manifest) ([]RouteBinding, error) {
	if err := compiler.ValidateManifest(config, app); err != nil {
		return nil, err
	}

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
				Kind:    RouteStatic,
				Method:  "GET",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: fmt.Sprintf(`embedded.Static("pages/%s.html")`, assetName(page.ID)),
			})
		}

		for _, action := range page.Blocks.Actions {
			routes = append(routes, RouteBinding{
				Kind:    RouteAction,
				Method:  "POST",
				Route:   page.Route,
				PageID:  page.ID,
				Handler: "actions." + exportedName(page.ID) + exportedName(action.Name),
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
			routes = append(routes, RouteBinding{
				Kind:    RouteAPI,
				Method:  method,
				Route:   route,
				PageID:  page.ID,
				Handler: "api." + handlerName,
			})
		}
	}

	return routes, nil
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
