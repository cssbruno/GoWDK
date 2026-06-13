package securitymanifest

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildProjectsRoutesAndEndpoints(t *testing.T) {
	ir := gwdkir.Program{
		Routes: []gwdkir.Route{
			{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/", PageID: "home", Render: gowdk.SPA, Guards: []string{"public"}, Source: "home.page.gwdk", Span: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}}},
			{Kind: gwdkir.RouteSSR, Method: "GET", Path: "/dashboard", PageID: "dashboard", Render: gowdk.SSR, Guards: []string{"auth.required"}, Source: "dashboard.page.gwdk", Span: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}}},
			{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/draft", PageID: "draft", Render: gowdk.SPA, Source: "draft.page.gwdk"},
		},
		Endpoints: []gwdkir.Endpoint{
			{Kind: gwdkir.EndpointAction, Symbol: "Submit", Method: "POST", Path: "/signup", PageID: "signup", Guards: []string{"public"}, CSRF: false, SourceFile: "signup.page.gwdk", Span: source.SourceSpan{Start: source.SourcePosition{Line: 8, Column: 1}}},
			{Kind: gwdkir.EndpointAPI, Symbol: "Health", Method: "GET", Path: "/api/health", PageID: "status", Guards: []string{"public"}, SourceFile: "status.page.gwdk"},
		},
	}

	manifest := Build(gowdk.Config{}, ir)

	if manifest.Version != SchemaVersion || manifest.GeneratedFrom != "ir" {
		t.Fatalf("unexpected manifest header: %+v", manifest)
	}
	if len(manifest.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(manifest.Routes))
	}

	byPage := map[string]RouteEntry{}
	for _, route := range manifest.Routes {
		byPage[route.PageID] = route
	}
	if home := byPage["home"]; !home.Public || home.DefaultDeny {
		t.Fatalf("home should be public and not default-deny: %+v", home)
	}
	if dash := byPage["dashboard"]; dash.Public || dash.DefaultDeny {
		t.Fatalf("dashboard should be protected (not public, not default-deny): %+v", dash)
	}
	if draft := byPage["draft"]; draft.Public || !draft.DefaultDeny {
		t.Fatalf("draft should be default-deny (no guard): %+v", draft)
	}
	if draft := byPage["draft"]; draft.Source != "draft.page.gwdk" {
		t.Fatalf("source without a span line should be the bare file: %q", draft.Source)
	}
	if home := byPage["home"]; home.Source != "home.page.gwdk:4" {
		t.Fatalf("source with a span line should be file:line, got %q", home.Source)
	}

	if got := manifest.Frontend.UnguardedRoutes; len(got) != 1 || got[0] != "/draft" {
		t.Fatalf("expected /draft in unguardedRoutes, got %#v", got)
	}

	byEndpoint := map[string]EndpointEntry{}
	for _, endpoint := range manifest.Endpoints {
		byEndpoint[endpoint.ID] = endpoint
	}
	submit, ok := byEndpoint["Submit"]
	if !ok {
		t.Fatalf("expected Submit endpoint, got %#v", manifest.Endpoints)
	}
	if submit.Kind != "action" || submit.CSRF {
		t.Fatalf("Submit should be an action without CSRF: %+v", submit)
	}
	if submit.BodyLimitBytes != gowdk.DefaultRequestBodyLimitBytes {
		t.Fatalf("Submit should carry the default action body limit, got %d", submit.BodyLimitBytes)
	}
	if submit.Source != "signup.page.gwdk:8" {
		t.Fatalf("Submit source should be file:line, got %q", submit.Source)
	}
}

func TestBuildHonorsConfiguredBodyLimits(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{BodyLimits: gowdk.BodyLimitsConfig{ActionBytes: 256 << 10, APIBytes: 512 << 10}}}
	ir := gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{
			{Kind: gwdkir.EndpointAction, Symbol: "Submit", Method: "POST", Path: "/a", PageID: "p", Guards: []string{"public"}},
			{Kind: gwdkir.EndpointAPI, Symbol: "List", Method: "GET", Path: "/api/l", PageID: "p", Guards: []string{"public"}},
		},
	}
	manifest := Build(config, ir)
	for _, endpoint := range manifest.Endpoints {
		switch endpoint.Kind {
		case "action":
			if endpoint.BodyLimitBytes != 256<<10 {
				t.Fatalf("action body limit = %d, want %d", endpoint.BodyLimitBytes, 256<<10)
			}
		case "api":
			if endpoint.BodyLimitBytes != 512<<10 {
				t.Fatalf("api body limit = %d, want %d", endpoint.BodyLimitBytes, 512<<10)
			}
		}
	}
}
