package buildgen

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestBuildWritesSPAHTMLForSimpleRoute(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><h1>GOWDK & friends</h1></main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", result.Artifacts)
	}
	if result.RouteManifestPath != filepath.Join(outputDir, routeManifestFile) {
		t.Fatalf("expected route manifest path, got %q", result.RouteManifestPath)
	}
	if result.AssetManifestPath != filepath.Join(outputDir, assetManifestFile) {
		t.Fatalf("expected asset manifest path, got %q", result.AssetManifestPath)
	}
	if result.BuildReportPath != filepath.Join(outputDir, buildReportFile) {
		t.Fatalf("expected build report path, got %q", result.BuildReportPath)
	}
	expectedSecurityManifestPath, err := securityManifestPath(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.SecurityManifestPath != expectedSecurityManifestPath {
		t.Fatalf("expected external security manifest path %q, got %q", expectedSecurityManifestPath, result.SecurityManifestPath)
	}
	if strings.HasPrefix(result.SecurityManifestPath, outputDir+string(filepath.Separator)) {
		t.Fatalf("security manifest must not live under served output dir: %q", result.SecurityManifestPath)
	}
	if result.Report.Version != 1 || result.Report.Mode != "build" {
		t.Fatalf("unexpected build report: %#v", result.Report)
	}
	requireBuildReportEvent(t, result.Report, "validate", "ir_valid")
	requireBuildReportEvent(t, result.Report, "plan", "artifacts_planned")
	requireBuildReportEvent(t, result.Report, "manifest", "route_manifest_written")
	requireBuildReportEvent(t, result.Report, "complete", "build_complete")

	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, "<title>home</title>") {
		t.Fatalf("expected title in output: %s", output)
	}
	if !strings.Contains(output, "GOWDK &amp; friends") {
		t.Fatalf("expected escaped body text in output: %s", output)
	}

	manifestPayload, err := os.ReadFile(filepath.Join(outputDir, routeManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var routes struct {
		Version int `json:"version"`
		Routes  []struct {
			PageID string `json:"page"`
			Route  string `json:"route"`
			Path   string `json:"path"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(manifestPayload, &routes); err != nil {
		t.Fatal(err)
	}
	if routes.Version != 1 || len(routes.Routes) != 1 {
		t.Fatalf("unexpected route manifest: %s", manifestPayload)
	}
	if routes.Routes[0].PageID != "home" || routes.Routes[0].Route != "/" || routes.Routes[0].Path != "index.html" {
		t.Fatalf("unexpected route manifest route: %#v", routes.Routes[0])
	}

	assetManifestPayload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(assetManifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Version != 1 || len(assets.Files) != 0 {
		t.Fatalf("unexpected asset manifest: %s", assetManifestPayload)
	}

	reportPayload, err := os.ReadFile(filepath.Join(outputDir, buildReportFile))
	if err != nil {
		t.Fatal(err)
	}
	var report BuildReport
	if err := json.Unmarshal(reportPayload, &report); err != nil {
		t.Fatal(err)
	}
	if report.Mode != "build" || !hasBuildReportEvent(report, "complete", "build_complete") {
		t.Fatalf("unexpected build report payload: %s", reportPayload)
	}

	securityPayload, err := os.ReadFile(result.SecurityManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(securityPayload), `"generatedFrom": "ir"`) {
		t.Fatalf("expected security manifest payload, got:\n%s", securityPayload)
	}
	if _, err := os.Stat(filepath.Join(outputDir, securityManifestFile)); !os.IsNotExist(err) {
		t.Fatalf("security manifest must not be written to served output root, stat err=%v", err)
	}
}

func TestBuildEmitsSPANavigationRuntimeForInternalLinks(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{
			{
				ID:    "home",
				Route: "/",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><Nav /></main>`,
				},
			},
			{
				ID:    "docs",
				Route: "/docs",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><h1>Docs</h1></main>`,
				},
			},
		},
		Components: []gwdkir.Component{{
			Name: "Nav",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<nav><a href="/docs">Docs</a><a href="https://example.com">External</a></nav>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AssetArtifacts) != 1 || result.AssetArtifacts[0].Path != filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath)) {
		t.Fatalf("expected SPA navigation runtime asset, got %#v", result.AssetArtifacts)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<script src="`+clientRuntimeHref+`" defer></script>`) {
		t.Fatalf("expected SPA navigation runtime script in output:\n%s", output)
	}
	runtimePayload, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(runtimePayload), `X-GOWDK-Navigate`) {
		t.Fatalf("expected navigation code in runtime:\n%s", runtimePayload)
	}
}

func TestBuildWritesPageMetadataToSPAHTMLHead(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Metadata: gwdkir.PageMetadata{
			Title:       "GOWDK - Go-native web apps",
			Description: "Portable .gwdk pages compiled into Go web output.",
			Canonical:   "https://gowdk.com/",
			Image:       "https://gowdk.com/assets/social.png",
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><h1>GOWDK</h1></main>`,
		},
	}}}

	config := gowdk.Config{Build: gowdk.BuildConfig{
		Head: gowdk.HeadConfig{
			SiteName:    "GOWDK",
			Favicon:     "/favicon.ico",
			TwitterCard: "summary",
		},
		Stylesheets: []gowdk.Stylesheet{{Href: "/assets/app.css"}},
		Scripts:     []gowdk.Script{{Src: "/assets/app.js", Type: "module"}},
	}}
	if _, err := Build(config, app, outputDir); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, want := range []string{
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>GOWDK - Go-native web apps</title>`,
		`<meta name="description" content="Portable .gwdk pages compiled into Go web output.">`,
		`<link rel="canonical" href="https://gowdk.com/">`,
		`<link rel="icon" href="/favicon.ico">`,
		`<meta property="og:site_name" content="GOWDK">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:url" content="https://gowdk.com/">`,
		`<meta property="og:title" content="GOWDK - Go-native web apps">`,
		`<meta property="og:description" content="Portable .gwdk pages compiled into Go web output.">`,
		`<meta property="og:image" content="https://gowdk.com/assets/social.png">`,
		`<meta name="twitter:card" content="summary">`,
		`<meta name="twitter:title" content="GOWDK - Go-native web apps">`,
		`<meta name="twitter:description" content="Portable .gwdk pages compiled into Go web output.">`,
		`<meta name="twitter:image" content="https://gowdk.com/assets/social.png">`,
		`<link rel="stylesheet" href="/assets/app.css">`,
		`<script type="module" src="/assets/app.js" defer></script>`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
	assertHTMLOrder(t, output,
		`<meta name="twitter:image" content="https://gowdk.com/assets/social.png">`,
		`<link rel="stylesheet" href="/assets/app.css">`,
	)
	assertHTMLOrder(t, output,
		`<link rel="stylesheet" href="/assets/app.css">`,
		`<script type="module" src="/assets/app.js" defer></script>`,
	)
}

func TestBuildMemoryReturnsSPAArtifactsWithoutWriting(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "dist")
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><h1>Browser compiler</h1></main>`,
		},
	}}}

	result, err := BuildMemory(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", result.Artifacts)
	}
	if result.RouteManifestPath != filepath.Join(outputDir, routeManifestFile) {
		t.Fatalf("expected route manifest path, got %q", result.RouteManifestPath)
	}
	if result.AssetManifestPath != filepath.Join(outputDir, assetManifestFile) {
		t.Fatalf("expected asset manifest path, got %q", result.AssetManifestPath)
	}
	if result.BuildReportPath != filepath.Join(outputDir, buildReportFile) {
		t.Fatalf("expected build report path, got %q", result.BuildReportPath)
	}
	expectedSecurityManifestPath, err := securityManifestPath(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.SecurityManifestPath != expectedSecurityManifestPath {
		t.Fatalf("expected external security manifest path %q, got %q", expectedSecurityManifestPath, result.SecurityManifestPath)
	}
	if result.Report.Version != 1 || result.Report.Mode != "memory" {
		t.Fatalf("unexpected memory build report: %#v", result.Report)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("BuildMemory should not write files, stat error = %v", err)
	}

	html := string(result.Files["index.html"])
	if !strings.Contains(html, "Browser compiler") {
		t.Fatalf("expected rendered HTML in memory result: %s", html)
	}
	if !strings.Contains(string(result.Files[routeManifestFile]), `"route": "/"`) {
		t.Fatalf("expected route manifest in memory result: %s", result.Files[routeManifestFile])
	}
	if !strings.Contains(string(result.Files[assetManifestFile]), `"version": 1`) {
		t.Fatalf("expected asset manifest in memory result: %s", result.Files[assetManifestFile])
	}
	if !strings.Contains(string(result.Files[buildReportFile]), `"mode": "memory"`) {
		t.Fatalf("expected build report in memory result: %s", result.Files[buildReportFile])
	}
	if _, ok := result.Files[securityManifestFile]; ok {
		t.Fatalf("security manifest must not be returned as a served memory output file")
	}
	if _, err := os.Stat(result.SecurityManifestPath); !os.IsNotExist(err) {
		t.Fatalf("memory build should not write the external security manifest, stat error = %v", err)
	}
}

func TestBuildRemovesStaleServedSecurityManifest(t *testing.T) {
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, securityManifestFile), []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, securityManifestFile)); !os.IsNotExist(err) {
		t.Fatalf("expected stale served security manifest to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(result.SecurityManifestPath); err != nil {
		t.Fatalf("expected external security manifest to be written: %v", err)
	}
}

func TestBuildReportIncludesContractReferences(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/patients.page.gwdk",
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Guards:  []string{"public", "auth.required"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><form method="post" action="/patients" g:command="patients.CreatePatient"><input name="name" /></form></main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "bind", "contract_reference")
	if event == nil {
		t.Fatalf("missing contract_reference event in %#v", result.Report.Events)
	}
	if event.PageID != "patients" || event.Path != "pages/patients.page.gwdk" {
		t.Fatalf("unexpected contract reference owner: %#v", event)
	}
	if event.Data["kind"] != "command" || event.Data["name"] != "patients.CreatePatient" || event.Data["status"] != "unknown" || event.Data["ownerKind"] != "page" {
		t.Fatalf("unexpected contract reference data: %#v", event.Data)
	}
	if event.Data["importAlias"] != "patients" || event.Data["type"] != "CreatePatient" {
		t.Fatalf("unexpected command contract type metadata: %#v", event.Data)
	}
	if event.Data["method"] != "POST" || event.Data["path"] != "/patients" {
		t.Fatalf("unexpected command method/path: %#v", event.Data)
	}
	if event.Data["guards"] != "public,auth.required" {
		t.Fatalf("unexpected command guards: %#v", event.Data)
	}
}

func TestBuildReportDerivesCommandReferencePathFromPageRoute(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/patients.page.gwdk",
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><form g:command="patients.CreatePatient"><input name="name" /></form></main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "bind", "contract_reference")
	if event == nil {
		t.Fatalf("missing contract_reference event in %#v", result.Report.Events)
	}
	if event.Data["kind"] != "command" || event.Data["method"] != "POST" || event.Data["path"] != "/patients" {
		t.Fatalf("unexpected derived command route metadata: %#v", event.Data)
	}
}

func TestBuildReportIncludesQueryContractReferences(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/patients.page.gwdk",
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><section g:query="patients.GetPatientPage"><h1>Patients</h1></section></main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "bind", "contract_reference")
	if event == nil {
		t.Fatalf("missing contract_reference event in %#v", result.Report.Events)
	}
	if event.Data["kind"] != "query" || event.Data["name"] != "patients.GetPatientPage" || event.Data["status"] != "unknown" || event.Data["ownerKind"] != "page" {
		t.Fatalf("unexpected contract reference data: %#v", event.Data)
	}
	if event.Data["importAlias"] != "patients" || event.Data["type"] != "GetPatientPage" {
		t.Fatalf("unexpected query contract type metadata: %#v", event.Data)
	}
	if event.Data["method"] != "GET" || event.Data["path"] != "/patients" {
		t.Fatalf("unexpected query method/path: %#v", event.Data)
	}
}

func TestBuildReportIncludesBoundContractReferenceRoles(t *testing.T) {
	outputDir := t.TempDir()
	result, err := BuildFromIR(gowdk.Config{}, gwdkir.Program{
		Version: gwdkir.Version,
		Pages: []gwdkir.Page{{
			Source: "pages/patients.page.gwdk",
			ID:     "patients",
			Route:  "/patients",
			Render: gowdk.SPA,
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Patients</main>`,
			},
		}},
		ContractRefs: []gwdkir.ContractReference{{
			Kind:      gwdkir.ContractCommand,
			Name:      "patients.CreatePatient",
			Type:      "CreatePatient",
			Result:    "CreatePatientResult",
			Roles:     []string{"web", "admin"},
			Method:    "POST",
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "HandleCreatePatient",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Source:    "pages/patients.page.gwdk",
		}},
	}, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "bind", "contract_reference")
	if event == nil {
		t.Fatalf("missing contract_reference event in %#v", result.Report.Events)
	}
	if event.Data["roles"] != "web,admin" || event.Data["status"] != "bound" {
		t.Fatalf("unexpected contract reference role data: %#v", event.Data)
	}
}

func TestBuildReportIncludesCachePolicySummary(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		{
			ID:         "home",
			Route:      "/",
			Cache:      "public, max-age=120",
			Revalidate: "30",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><a href="/about">About</a></main>`,
			},
		},
		{
			ID:    "about",
			Route: "/about",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>About</main>`,
			},
		},
	}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "report", "cache_policy")
	if event == nil {
		t.Fatalf("missing cache_policy event in %#v", result.Report.Events)
	}
	for key, expected := range map[string]string{
		"pageHtml":           "2",
		"assets":             "1",
		"defaultPageHTML":    "no-cache",
		"defaultRequestTime": "no-store",
		"pageHTMLPolicies":   `{"no-cache":1,"public, max-age=120, stale-while-revalidate=30":1}`,
		"assetPolicies":      `{"no-cache":1}`,
	} {
		if event.Data[key] != expected {
			t.Fatalf("expected cache policy data %s=%q, got %#v", key, expected, event.Data)
		}
	}
}

func TestBuildReportIncludesBackendBindingEndpointMetadata(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	sourcePath := filepath.Join(root, "features", "auth", "login.page.gwdk")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	ir := gwdkanalysis.BuildProgram(gowdk.Config{}, gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Source:  sourcePath,
			Package: "auth",
			ID:      "login",
			Route:   "/login",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Login</main>`},
		}},
	})
	bindings := []source.BackendBinding{{
		Kind:         "action",
		PageID:       "login",
		Source:       sourcePath,
		BlockName:    "Login",
		Method:       "POST",
		Route:        "/login",
		PackageName:  "auth",
		FunctionName: "Login",
		Status:       source.BackendBindingMissing,
		Message:      "GOWDK action handler auth.Login is not implemented",
	}}

	result, err := buildFromIR(gowdk.Config{}, ir, bindings, outputDir, true)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "bind", "backend_binding")
	if event == nil {
		t.Fatalf("missing backend binding event in %#v", result.Report.Events)
	}
	if event.PageID != "login" || event.Route != "/login" {
		t.Fatalf("unexpected backend binding event route data: %#v", event)
	}
	for key, expected := range map[string]string{
		"kind":     "action",
		"block":    "Login",
		"method":   "POST",
		"package":  "auth",
		"function": "Login",
		"status":   "missing",
		"message":  "GOWDK action handler auth.Login is not implemented",
	} {
		if event.Data[key] != expected {
			t.Fatalf("expected backend binding data %s=%q, got %#v", key, expected, event.Data)
		}
	}
}

func TestBuildReturnsReportOnValidationError(t *testing.T) {
	_, err := Build(gowdk.Config{}, gwdkanalysis.Sources{}, "")
	if err == nil {
		t.Fatal("expected build error")
	}
	if err.Error() != "build output directory is required" {
		t.Fatalf("unexpected error text: %v", err)
	}
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T", err)
	}
	if buildErr.Report.Version != 1 || buildErr.Report.Mode != "build" {
		t.Fatalf("unexpected error report: %#v", buildErr.Report)
	}
	requireBuildReportEvent(t, buildErr.Report, "validate", "failed")
}

func assertHTMLOrder(t *testing.T, html string, snippets ...string) {
	t.Helper()
	previous := -1
	for _, snippet := range snippets {
		index := strings.Index(html, snippet)
		if index < 0 {
			t.Fatalf("expected %q in HTML:\n%s", snippet, html)
		}
		if index <= previous {
			t.Fatalf("expected %q after earlier snippets:\n%s", snippet, html)
		}
		previous = index
	}
}

func requireBuildReportEvent(t *testing.T, report BuildReport, stage string, kind string) {
	t.Helper()
	if !hasBuildReportEvent(report, stage, kind) {
		t.Fatalf("missing report event %s/%s in %#v", stage, kind, report.Events)
	}
}

func hasBuildReportEvent(report BuildReport, stage string, kind string) bool {
	return findBuildReportEvent(report, stage, kind) != nil
}

func findBuildReportEvent(report BuildReport, stage string, kind string) *BuildEvent {
	for _, event := range report.Events {
		if event.Stage == stage && event.Kind == kind {
			return &event
		}
	}
	return nil
}
