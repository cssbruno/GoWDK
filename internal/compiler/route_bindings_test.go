package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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

func TestBuildRouteMetadataMapsHybridWithoutExplicitLoadToSPA(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}}}

	metadata, err := BuildRouteMetadata(gowdk.Config{}, app)
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, metadata.Routes, RouteSPA, "GET", "/dashboard", `embedded.SPA("pages/dashboard.html")`)
	assertInfo(t, metadata.Info, "ssr_disabled", "dashboard")
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

func TestBuildRouteMetadataFromIR(t *testing.T) {
	metadata := BuildRouteMetadataFromIR(gowdk.Config{}, gwdkir.Program{
		Version: gwdkir.Version,
		Routes: []gwdkir.Route{
			{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/newsletter", PageID: "newsletter", Render: gowdk.SPA},
			{Kind: gwdkir.RouteSSR, Method: "GET", Path: "/dashboard", PageID: "dashboard", Render: gowdk.SSR},
		},
		Endpoints: []gwdkir.Endpoint{
			{
				Kind:       gwdkir.EndpointAction,
				Source:     gwdkir.EndpointSourceGOWDK,
				PageID:     "newsletter",
				Symbol:     "Subscribe",
				Method:     "POST",
				Path:       "/newsletter",
				SourceFile: "newsletter.page.gwdk",
				Binding: gwdkir.Binding{
					Status:       manifest.BackendBindingBound,
					ImportPath:   "example.com/app/newsletter",
					PackageName:  "newsletter",
					FunctionName: "Subscribe",
					Signature:    manifest.BackendSignatureAction0,
				},
			},
		},
	})

	assertRoute(t, metadata.Routes, RouteSPA, "GET", "/newsletter", `embedded.SPA("pages/newsletter.html")`)
	assertRoute(t, metadata.Routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
	assertEndpoint(t, metadata.Endpoints, EndpointAction, "POST", "/newsletter", "actions.NewsletterSubscribe")
	if metadata.Endpoints[0].BindingStatus != manifest.BackendBindingBound {
		t.Fatalf("expected binding status from IR, got %#v", metadata.Endpoints[0])
	}
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
