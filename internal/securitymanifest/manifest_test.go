package securitymanifest

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cssbruno/gowdk"
	authaddon "github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestRelativizeMakesPosturePortableAcrossCheckouts(t *testing.T) {
	// The same project checked out at two different absolute roots must produce
	// an identical relativized posture, so any digest derived from it (and the
	// raw-HTML fingerprints a policy exception pins) match across machines/CI.
	build := func(root string) SecurityManifest {
		source := filepath.Join(root, "pages", "home.page.gwdk")
		return SecurityManifest{
			Version: SchemaVersion,
			Routes:  []RouteEntry{{PageID: "home", Route: "/", Source: source + ":4"}},
			Endpoints: []EndpointEntry{{
				ID: "Submit", Kind: "action", Method: "POST", Path: "/submit",
				Source: filepath.Join(root, "pages", "home.go") + ":12",
			}},
			Frontend: FrontendSurface{
				UnguardedRoutes: []UnguardedRoute{{Route: "/draft", Source: source + ":2"}},
				RawHTMLSinks: []RawHTMLSink{{
					OwnerKind: "page", OwnerID: "home", Field: "{X}", Source: source + ":9", Ordinal: 0,
				}},
			},
		}
	}

	rootA := filepath.Join(string(filepath.Separator)+"home", "dev", "app")
	rootB := filepath.Join(string(filepath.Separator)+"workspace", "ci", "checkout")
	relA := build(rootA).Relativize(rootA)
	relB := build(rootB).Relativize(rootB)

	if got := relA.Routes[0].Source; got != "pages/home.page.gwdk:4" {
		t.Fatalf("route source not relativized: %q", got)
	}
	if relA.Frontend.RawHTMLSinks[0].Fingerprint == "" {
		t.Fatal("expected the raw-HTML fingerprint to be recomputed from the relative source")
	}
	if !reflect.DeepEqual(relA, relB) {
		t.Fatalf("relativized posture differs across checkouts:\n%#v\n%#v", relA, relB)
	}
}

func TestRelativizeLeavesNonFileAndRelativeRefsUntouched(t *testing.T) {
	manifest := SecurityManifest{
		Version: SchemaVersion,
		Routes: []RouteEntry{
			{PageID: "a", Route: "/a", Source: "config:Build.SecurityHeaders"},
			{PageID: "b", Route: "/b", Source: "already/relative.page.gwdk:3"},
		},
	}
	out := manifest.Relativize(string(filepath.Separator) + "root")
	if out.Routes[0].Source != "config:Build.SecurityHeaders" {
		t.Fatalf("non-file source must pass through, got %q", out.Routes[0].Source)
	}
	if out.Routes[1].Source != "already/relative.page.gwdk:3" {
		t.Fatalf("already-relative source must pass through, got %q", out.Routes[1].Source)
	}
}

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

func TestBuildFromValidatedProgramRejectsZeroValue(t *testing.T) {
	_, err := BuildFromValidatedProgram(gowdk.Config{}, compiler.ValidatedProgram{})
	if err == nil {
		t.Fatal("expected zero-value validated program error")
	}
	if err.Error() != "validated program was not constructed by compiler validation" {
		t.Fatalf("unexpected error: %v", err)
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

func TestBuildPopulatesAuthAddonPosture(t *testing.T) {
	manifest := Build(gowdk.Config{Addons: []gowdk.Addon{authaddon.Addon()}}, gwdkir.Program{
		Pages: []gwdkir.Page{{ID: "home", Route: "/", Guards: []string{"auth.required"}}},
	})
	if manifest.Auth == nil {
		t.Fatal("expected auth posture when auth addon is configured")
	}
	if manifest.Auth.SessionMode != "signed-cookie" || !manifest.Auth.AddonConfigured {
		t.Fatalf("unexpected auth posture: %#v", manifest.Auth)
	}
	if manifest.Auth.Revocation == "" || manifest.Auth.AuthorizationVersion == "" {
		t.Fatalf("expected revocation and authorization-version details: %#v", manifest.Auth)
	}
}

func TestRawHTMLSinkFingerprintIsStableAndSourceSensitive(t *testing.T) {
	base := RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:12", 0)
	if base == "" {
		t.Fatal("fingerprint should not be empty")
	}
	if base != RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:12", 0) {
		t.Fatal("fingerprint should be stable for identical inputs")
	}
	if base == RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:40", 0) {
		t.Fatal("moving the source should change the fingerprint")
	}
	if base == RawHTMLFingerprint("page", "home", "{Rendered}", "home.page.gwdk:12", 0) {
		t.Fatal("changing the expression should change the fingerprint")
	}
	if base == RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:12", 1) {
		t.Fatal("a different sink ordinal should change the fingerprint")
	}
}

func TestBuildRecordsRawHTMLSinkFingerprint(t *testing.T) {
	ir := gwdkir.Program{
		Templates: []gwdkir.Template{{
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "home",
			Source:    "home.page.gwdk",
			Body:      `<main><div g:unsafe-html={Body}></div></main>`,
			BodyStart: source.SourcePosition{Line: 12, Column: 1},
			Span:      source.SourceSpan{Start: source.SourcePosition{Line: 11, Column: 1}},
		}},
	}
	manifest := Build(gowdk.Config{}, ir)
	if len(manifest.Frontend.RawHTMLSinks) != 1 {
		t.Fatalf("expected one sink, got %#v", manifest.Frontend.RawHTMLSinks)
	}
	sink := manifest.Frontend.RawHTMLSinks[0]
	want := RawHTMLFingerprint(sink.OwnerKind, sink.OwnerID, sink.Field, sink.Source, sink.Ordinal)
	if sink.Fingerprint != want || sink.Fingerprint == "" {
		t.Fatalf("sink fingerprint should match the exported derivation, got %#v", sink)
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

func TestBuildRecordsNormalizedAndRedactedHeaderValues(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{
		Mode: gowdk.Production,
		SecurityHeaders: gowdk.SecurityHeadersConfig{
			Enabled: true,
			Headers: map[string]string{
				"Content-Security-Policy": "  default-src   'self' ",
				"X-Internal-Token":        "live_sk_abcdef0123456789abcdef",
			},
		},
	}}
	manifest := Build(config, gwdkir.Program{})
	if manifest.BuildMode != string(gowdk.Production) {
		t.Fatalf("expected production build mode in manifest, got %q", manifest.BuildMode)
	}
	byName := map[string]string{}
	for _, header := range manifest.Frontend.ConfiguredHeaders {
		byName[header.Name] = header.Value
	}
	if got := byName["Content-Security-Policy"]; got != "default-src 'self'" {
		t.Fatalf("expected normalized CSP value, got %q", got)
	}
	if got := byName["X-Internal-Token"]; got != "[redacted]" {
		t.Fatalf("expected secret-shaped header value to be redacted, got %q", got)
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

func TestBuildRecordsEffectiveRequestLimitPosture(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{BodyLimits: gowdk.BodyLimitsConfig{APIBytes: 512 << 10}}}
	ir := gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{
			{Kind: gwdkir.EndpointAction, Symbol: "Submit", Method: "POST", Path: "/a", PageID: "p", Guards: []string{"public"}},
			{Kind: gwdkir.EndpointAPI, Symbol: "List", Method: "POST", Path: "/api/l", PageID: "p", Guards: []string{"public"}},
		},
	}
	manifest := Build(config, ir)
	byID := map[string]EndpointEntry{}
	for _, endpoint := range manifest.Endpoints {
		byID[endpoint.ID] = endpoint
	}

	api := byID["List"].RequestLimits
	if api.RawBodyBytes != 512<<10 || byID["List"].BodyLimitBytes != api.RawBodyBytes {
		t.Fatalf("api raw body limit should mirror BodyLimitBytes, got %#v", byID["List"])
	}
	if !api.InstalledBeforeParse || api.Phase != "before-body-parse-and-csrf" {
		t.Fatalf("api limit should be installed before parse/CSRF, got %#v", api)
	}
	if api.CompressedBodyHandling != "raw-bytes-bounded" {
		t.Fatalf("compressed bodies should be bounded by the raw cap, got %#v", api)
	}
	if api.Origin != "config:Build.BodyLimits.APIBytes" {
		t.Fatalf("api limit origin should attribute to config, got %q", api.Origin)
	}

	action := byID["Submit"].RequestLimits
	if action.RawBodyBytes != gowdk.DefaultRequestBodyLimitBytes {
		t.Fatalf("action should fall back to the default cap, got %#v", action)
	}
	if action.Origin != "default:gowdk.DefaultRequestBodyLimitBytes" {
		t.Fatalf("default action limit should attribute to the default, got %q", action.Origin)
	}
}

func TestBuildRecordsCORSOnlyForGeneratedCORSRoutes(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{CORS: gowdk.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
	}}}
	actionOnly := Build(config, gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{
			{Kind: gwdkir.EndpointAction, Symbol: "Submit", Method: http.MethodPost, Path: "/submit", Guards: []string{"public"}},
			{Kind: gwdkir.EndpointFragment, Symbol: "Panel", Method: http.MethodGet, Path: "/panel", Guards: []string{"public"}},
		},
	})
	if actionOnly.CORS.Enabled || actionOnly.CORS.AllowsAnyOrigin {
		t.Fatalf("action/fragment-only output should not record generated CORS posture, got %#v", actionOnly.CORS)
	}

	apiOutput := Build(config, gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{
			{Kind: gwdkir.EndpointAPI, Symbol: "Health", Method: http.MethodGet, Path: "/api/health", Guards: []string{"public"}},
		},
	})
	if !apiOutput.CORS.Enabled || !apiOutput.CORS.AllowsAnyOrigin {
		t.Fatalf("API output should record generated CORS posture, got %#v", apiOutput.CORS)
	}
}

func TestBuildRecordsGuardContractAndObservabilityEvidence(t *testing.T) {
	root := t.TempDir()
	absPage := filepath.Join(root, "patients.page.gwdk")
	config := gowdk.Config{
		Addons: []gowdk.Addon{
			gowdk.NewAddon("observability", gowdk.FeatureObservability),
		},
	}
	ir := gwdkir.Program{
		Routes: []gwdkir.Route{
			{Kind: gwdkir.RouteSSR, Method: "GET", Path: "/admin", PageID: "admin", Render: gowdk.SSR, Guards: []string{"auth.required", "role:admin"}, Source: "admin.page.gwdk", Span: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}}},
		},
		ContractRefs: []gwdkir.ContractReference{{
			Kind:              gwdkir.ContractCommand,
			Name:              "patients.CreatePatient",
			Method:            "POST",
			Path:              "/patients",
			Guards:            []string{"role:admin"},
			Status:            gwdkir.ContractBindingBound,
			DeclarationSource: "contracts/patients.go",
			DeclarationSpan:   source.SourceSpan{Start: source.SourcePosition{Line: 20, Column: 3}},
			Source:            absPage,
			Span:              source.SourceSpan{Start: source.SourcePosition{Line: 12, Column: 5}},
		}},
	}

	manifest := Build(config, ir)

	if len(manifest.Routes) != 1 {
		t.Fatalf("expected one route, got %#v", manifest.Routes)
	}
	guards := manifest.Routes[0].GuardEvidence
	if len(guards) != 2 {
		t.Fatalf("expected two guard evidence entries, got %#v", guards)
	}
	if guards[0].ID != "auth.required" || guards[0].BindingStatus != "unverified-app-owned" || guards[0].Owner != "app-owned" {
		t.Fatalf("expected auth.required to be unverified app-owned without auth addon, got %#v", guards[0])
	}
	if guards[1].ID != "role:admin" || guards[1].BindingStatus != "resolved-native" || guards[1].Owner != "gowdk-native" {
		t.Fatalf("expected role guard to be native evidence, got %#v", guards[1])
	}

	if len(manifest.Contracts) != 1 {
		t.Fatalf("expected one contract posture entry, got %#v", manifest.Contracts)
	}
	contract := manifest.Contracts[0]
	if contract.DeclarationSource != "contracts/patients.go:20" || contract.ExposureSource != absPage+":12" || contract.SourceAttribution != "declaration-and-exposure" {
		t.Fatalf("unexpected contract source attribution: %#v", contract)
	}

	if len(manifest.Observability) != 4 {
		t.Fatalf("expected trace viewer/data/events/browser posture entries, got %#v", manifest.Observability)
	}
	browser, ok := observabilityEntry(manifest.Observability, "trace.browser", "/_gowdk/traces/browser", true)
	if !ok {
		t.Fatalf("expected browser ingestion posture with absolute-source flag, got %#v", manifest.Observability)
	}
	if !hasMethod(browser.Methods, http.MethodPost) || browser.BodyLimitBytes != 1<<20 {
		t.Fatalf("expected browser ingestion body posture, got %#v", browser)
	}
	data, ok := observabilityEntry(manifest.Observability, "trace.data", "/_gowdk/traces/data", true)
	if !ok || !hasMethod(data.Methods, http.MethodGet) || !hasMethod(data.Methods, http.MethodPost) || data.BodyLimitBytes != 1<<20 {
		t.Fatalf("expected trace data GET/POST posture with body limit, got %#v", data)
	}
	events, ok := observabilityEntry(manifest.Observability, "trace.events", "/_gowdk/traces/events", true)
	if !ok || !hasMethod(events.Methods, http.MethodGet) || !hasMethod(events.Methods, http.MethodPost) || events.BodyLimitBytes != 1<<20 {
		t.Fatalf("expected trace events GET/POST posture with body limit, got %#v", events)
	}
	if events.SubscriberLimit != 0 {
		t.Fatalf("trace events should not report an unenforced subscriber cap, got %#v", events)
	}
}

func observabilityEntry(entries []ObservabilityEntry, id string, requestPath string, absoluteSources bool) (ObservabilityEntry, bool) {
	for _, entry := range entries {
		if entry.ID == id && entry.Path == requestPath && entry.ExportsAbsoluteSourcePaths == absoluteSources {
			return entry, true
		}
	}
	return ObservabilityEntry{}, false
}

func hasMethod(methods []string, method string) bool {
	for _, candidate := range methods {
		if candidate == method {
			return true
		}
	}
	return false
}
