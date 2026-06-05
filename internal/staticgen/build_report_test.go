package staticgen

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestBuildWritesStaticHTMLForSimpleRoute(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
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
	if result.Report.Version != 1 || result.Report.Mode != "build" {
		t.Fatalf("unexpected build report: %#v", result.Report)
	}
	requireBuildReportEvent(t, result.Report, "validate", "manifest_valid")
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
}

func TestBuildMemoryReturnsStaticArtifactsWithoutWriting(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "dist")
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
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
}

func TestBuildReturnsReportOnValidationError(t *testing.T) {
	_, err := Build(gowdk.Config{}, manifest.Manifest{}, "")
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

func requireBuildReportEvent(t *testing.T, report BuildReport, stage string, kind string) {
	t.Helper()
	if !hasBuildReportEvent(report, stage, kind) {
		t.Fatalf("missing report event %s/%s in %#v", stage, kind, report.Events)
	}
}

func hasBuildReportEvent(report BuildReport, stage string, kind string) bool {
	for _, event := range report.Events {
		if event.Stage == stage && event.Kind == kind {
			return true
		}
	}
	return false
}
