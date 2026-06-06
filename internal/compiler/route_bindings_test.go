package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildRouteMetadataSeparatesRoutesFromEndpoints(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{
				ID:     "newsletter",
				Route:  "/newsletter",
				Render: gowdk.SPA,
				Blocks: manifest.Blocks{
					View:    true,
					Actions: []manifest.Action{{Name: "Subscribe"}},
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
				Render: gowdk.SPA,
				Blocks: manifest.Blocks{
					View: true,
					APIs: []manifest.API{{Name: "List", Method: "GET", Route: "/api/patients"}},
				},
			},
		},
	}

	metadata, err := BuildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}

	assertRoute(t, metadata.Routes, RouteSPA, "GET", "/newsletter", `embedded.SPA("pages/newsletter.html")`)
	assertRoute(t, metadata.Routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
	assertEndpoint(t, metadata.Endpoints, EndpointAction, "POST", "/newsletter", "actions.NewsletterSubscribe")
	assertEndpoint(t, metadata.Endpoints, EndpointAPI, "GET", "/api/patients", "api.PatientsIndexList")
	assertInfo(t, metadata.Info, "ssr_disabled", "newsletter")
	assertInfo(t, metadata.Info, "spa_disabled", "dashboard")
}

func TestBuildRouteMetadataRejectsSSRWithoutAddon(t *testing.T) {
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

	_, err := BuildRouteMetadata(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing SSR addon error")
	}
}

func TestBuildRouteMetadataRejectsHybridWithoutExplicitLoad(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}}}

	_, err := BuildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err == nil {
		t.Fatal("expected hybrid request policy error")
	}
	if !strings.Contains(err.Error(), "implicit SSR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRouteMetadataMapsHybridWithLoadToHybridRoute(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			Load: true,
			View: true,
		},
	}}}

	metadata, err := BuildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, metadata.Routes, RouteHybrid, "GET", "/dashboard", "hybrid.RenderDashboard")
}

func assertRoute(t *testing.T, routes []RouteBinding, kind RouteKind, method, route, handler string) {
	t.Helper()
	for _, binding := range routes {
		if binding.Kind == kind && binding.Method == method && binding.Route == route && binding.Handler == handler {
			return
		}
	}
	t.Fatalf("Missing route kind=%s method=%s route=%s handler=%s in %#v", kind, method, route, handler, routes)
}

func assertEndpoint(t *testing.T, endpoints []EndpointBinding, kind EndpointKind, method, route, handler string) {
	t.Helper()
	for _, binding := range endpoints {
		if binding.Kind == kind && binding.Method == method && binding.Route == route && binding.Handler == handler {
			return
		}
	}
	t.Fatalf("Missing endpoint kind=%s method=%s route=%s handler=%s in %#v", kind, method, route, handler, endpoints)
}

func assertInfo(t *testing.T, infos []RouteInfo, code string, pageID string) {
	t.Helper()
	for _, info := range infos {
		if info.Code == code && info.PageID == pageID {
			return
		}
	}
	t.Fatalf("Missing info code=%s page=%s in %#v", code, pageID, infos)
}
