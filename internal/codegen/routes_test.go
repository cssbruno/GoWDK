package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildRouteBindingsMapsStaticActionsSSRAndAPI(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{
				ID:     "newsletter",
				Route:  "/newsletter",
				Render: gowdk.Static,
				Blocks: manifest.Blocks{
					View:    true,
					Actions: []manifest.Action{{Name: "subscribe"}},
				},
			},
			{
				ID:     "dashboard",
				Route:  "/dashboard",
				Render: gowdk.SSR,
				Blocks: manifest.Blocks{
					Load: true,
					View: true,
				},
			},
			{
				ID:     "patients.index",
				Route:  "/patients",
				Render: gowdk.Static,
				Blocks: manifest.Blocks{
					View: true,
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

func TestGenerateRouteRegistrationEmitsFormattedGoSource(t *testing.T) {
	bindings := []RouteBinding{
		{Kind: RouteStatic, Method: "GET", Route: "/", PageID: "home", Handler: `embedded.Static("pages/home.html")`},
		{Kind: RouteAction, Method: "POST", Route: "/newsletter", PageID: "newsletter", Handler: "actions.NewsletterSubscribe"},
		{Kind: RouteAPI, Method: "GET", Route: "/api/health", PageID: "status", Handler: "api.StatusHealth"},
		{Kind: RouteSSR, Method: "GET", Route: "/dashboard", PageID: "dashboard", Handler: "ssr.RenderDashboard"},
	}

	source, err := GenerateRouteRegistration(bindings, RouteRegistrationOptions{
		PackageName: "routes",
		Imports: map[string]string{
			"actions":  "example.com/site/internal/actions",
			"api":      "example.com/site/internal/api",
			"embedded": "example.com/site/internal/embedded",
			"ssr":      "example.com/site/internal/ssr",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "routes.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated route registration is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package routes`,
		`embedded "example.com/site/internal/embedded"`,
		`mux.HandleFunc("GET /", embedded.Static("pages/home.html"))`,
		`mux.HandleFunc("POST /newsletter", actions.NewsletterSubscribe)`,
		`mux.HandleFunc("GET /api/health", api.StatusHealth)`,
		`mux.HandleFunc("GET /dashboard", ssr.RenderDashboard)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateRouteRegistrationRequiresHandlerImports(t *testing.T) {
	_, err := GenerateRouteRegistration([]RouteBinding{{
		Kind:    RouteAction,
		Method:  "POST",
		Route:   "/newsletter",
		PageID:  "newsletter",
		Handler: "actions.NewsletterSubscribe",
	}}, RouteRegistrationOptions{})
	if err == nil {
		t.Fatal("expected missing import path error")
	}
	if !strings.Contains(err.Error(), `missing import path for route handler package "actions"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRouteBindingsRejectsSSRWithoutAddon(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				View: true,
			},
		}},
	}

	_, err := BuildRouteBindings(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing SSR addon error")
	}
}

func TestBuildRouteBindingsRejectsHybridWithoutExplicitLoad(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}}}

	_, err := BuildRouteBindings(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err == nil {
		t.Fatal("expected hybrid request policy error")
	}
	if !strings.Contains(err.Error(), "implicit SSR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRouteBindingsMapsHybridWithLoadToSSRRoute(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			Load: true,
			View: true,
		},
	}}}

	routes, err := BuildRouteBindings(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
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
