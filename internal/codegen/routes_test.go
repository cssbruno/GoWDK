package codegen

import (
	"testing"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/addons/ssr"
	"github.com/gowdk/gowdk/internal/manifest"
)

func TestBuildRouteBindingsMapsStaticActionsSSRAndAPI(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{
				ID:     "newsletter",
				Route:  "/newsletter",
				Render: gowdk.Static,
				Blocks: manifest.Blocks{
					Actions: []manifest.Action{{Name: "subscribe"}},
				},
			},
			{
				ID:     "dashboard",
				Route:  "/dashboard",
				Render: gowdk.SSR,
				Blocks: manifest.Blocks{
					Load: true,
				},
			},
			{
				ID:     "patients.index",
				Route:  "/patients",
				Render: gowdk.Static,
				Blocks: manifest.Blocks{
					APIs: []manifest.API{{Method: "GET", Route: "/api/patients"}},
				},
			},
		},
	}

	routes, err := BuildRouteBindings(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}

	assertRoute(t, routes, RouteStatic, "GET", "/newsletter", `embedded.Static("pages/newsletter.html")`)
	assertRoute(t, routes, RouteAction, "POST", "/newsletter", "actions.NewsletterSubscribe")
	assertRoute(t, routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
	assertRoute(t, routes, RouteAPI, "GET", "/api/patients", "api.PatientsIndex")
}

func TestBuildRouteBindingsRejectsSSRWithoutAddon(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{ID: "dashboard", Route: "/dashboard", Render: gowdk.SSR}},
	}

	_, err := BuildRouteBindings(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing SSR addon error")
	}
}

func assertRoute(t *testing.T, routes []RouteBinding, kind RouteKind, method, route, handler string) {
	t.Helper()
	for _, binding := range routes {
		if binding.Kind == kind && binding.Method == method && binding.Route == route && binding.Handler == handler {
			return
		}
	}
	t.Fatalf("missing route kind=%s method=%s route=%s handler=%s in %#v", kind, method, route, handler, routes)
}
