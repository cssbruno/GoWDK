package securitymanifest

import (
	"os"
	"path/filepath"
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

	if got := manifest.Frontend.UnguardedRoutes; len(got) != 1 || got[0].Route != "/draft" {
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

func TestBuildPopulatesFrontendAuditSurface(t *testing.T) {
	root := t.TempDir()
	componentPath := filepath.Join(root, "card.cmp.gwdk")
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`api_key=live_sk_abc123`), 0o644); err != nil {
		t.Fatal(err)
	}
	config := gowdk.Config{Build: gowdk.BuildConfig{SecurityHeaders: gowdk.SecurityHeadersConfig{
		Enabled: true,
		Headers: map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"Content-Security-Policy": "default-src 'self'",
			"content-security-policy": "default-src 'self'",
		},
	}}}
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			Source: "home.page.gwdk",
			ID:     "home",
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { api_key: "live_sk_abc123" }`,
				Spans:     gwdkir.BlockSpans{Build: source.SourceSpan{Start: source.SourcePosition{Line: 9, Column: 1}}},
			},
		}},
		Assets: []gwdkir.Asset{
			{Kind: gwdkir.AssetFile, Source: "card.cmp.gwdk", Path: ".env", Span: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}}},
			{Kind: gwdkir.AssetFile, Source: componentPath, Path: "./config.json", Span: source.SourceSpan{Start: source.SourcePosition{Line: 5, Column: 1}}},
			{Kind: gwdkir.AssetJS, Source: "home.page.gwdk", Inline: `const token = "secret";`, Span: source.SourceSpan{Start: source.SourcePosition{Line: 5, Column: 1}}},
		},
		Templates: []gwdkir.Template{{
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "home",
			Source:    "home.page.gwdk",
			Body:      `<main><div g:unsafe-html={TrustedHTML}></div></main>`,
			BodyStart: source.SourcePosition{Line: 12, Column: 1},
			Span:      source.SourceSpan{Start: source.SourcePosition{Line: 11, Column: 1}},
		}},
	}

	manifest := Build(config, ir)

	if got := manifest.Frontend.ConfiguredHeaders; len(got) != 2 || got[0].Name != "Content-Security-Policy" || got[1].Name != "X-Content-Type-Options" {
		t.Fatalf("expected sorted configured headers, got %#v", got)
	}
	if got := manifest.Frontend.BundleSecrets; len(got) != 4 {
		t.Fatalf("expected four bundle secret findings, got %#v", got)
	}
	if !hasBundleLeak(manifest.Frontend.BundleSecrets, componentPath+":5", "file-asset:secret-key-value") {
		t.Fatalf("expected source-selected file asset content secret finding, got %#v", manifest.Frontend.BundleSecrets)
	}
	if got := manifest.Frontend.RawHTMLSinks; len(got) != 1 || got[0].OwnerID != "home" || got[0].Field != "TrustedHTML" || got[0].Source != "home.page.gwdk:12" {
		t.Fatalf("expected raw HTML sink source, got %#v", got)
	}
}

func hasBundleLeak(leaks []BundleLeak, source string, kind string) bool {
	for _, leak := range leaks {
		if leak.Source == source && leak.Kind == kind {
			return true
		}
	}
	return false
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
