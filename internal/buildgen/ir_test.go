package buildgen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestPlanFromIRMatchesSourcePlan(t *testing.T) {
	config := gowdk.Config{}
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Source:  "pages/home.page.gwdk",
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Home" }`,
				View:      true,
				ViewBody:  `<main>{title}</main>`,
			},
		}},
	}

	fromSources, err := plan(config, app, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fromIR, err := planFromIR(config, gwdkanalysis.BuildProgram(config, app), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if len(fromSources.pages) != len(fromIR.pages) {
		t.Fatalf("page plan count mismatch: %d != %d", len(fromSources.pages), len(fromIR.pages))
	}
	if fromIR.pages[0].PageID != "home" || fromIR.pages[0].Route != "/" {
		t.Fatalf("unexpected IR page plan: %#v", fromIR.pages[0])
	}
	if string(fromIR.pages[0].contents) != string(fromSources.pages[0].contents) {
		t.Fatalf("IR plan content differs:\n%s\n---\n%s", fromIR.pages[0].contents, fromSources.pages[0].contents)
	}
}

func TestBuildFromIRWritesArtifacts(t *testing.T) {
	config := gowdk.Config{}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/home.page.gwdk",
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}
	outputDir := t.TempDir()

	result, err := BuildFromIR(config, gwdkanalysis.BuildProgram(config, app), outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].PageID != "home" {
		t.Fatalf("unexpected artifacts: %#v", result.Artifacts)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "<main>Home</main>") {
		t.Fatalf("expected generated page content, got:\n%s", payload)
	}
}

func TestBuildFromAnalyzedProgramChecksInvariants(t *testing.T) {
	invalidIR := gwdkir.Program{Routes: []gwdkir.Route{{
		Kind:   gwdkir.RouteSPA,
		Method: "GET",
		Path:   "/",
		PageID: "missing",
	}}}
	_, err := BuildFromAnalyzedProgram(gowdk.Config{}, compiler.AnalyzedProgramFromIR(invalidIR), t.TempDir())
	if err == nil {
		t.Fatal("expected invalid IR error")
	}
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T", err)
	}
	if !strings.Contains(err.Error(), "internal compiler error: invalid IR") {
		t.Fatalf("expected invariant error, got %v", err)
	}
	requireBuildReportEvent(t, buildErr.Report, "validate", "failed")
}

func TestBuildFromValidatedProgramRejectsZeroValue(t *testing.T) {
	_, err := BuildFromValidatedProgram(gowdk.Config{}, compiler.ValidatedProgram{}, t.TempDir())
	if err == nil {
		t.Fatal("expected zero-value validated program error")
	}
	if !strings.Contains(err.Error(), "not constructed by compiler validation") {
		t.Fatalf("expected validated program construction error, got %v", err)
	}
}

func TestBuildFromPlanRejectsZeroValue(t *testing.T) {
	_, err := BuildFromPlan(BuildPlan{})
	if err == nil {
		t.Fatal("expected zero-value build plan error")
	}
	if !strings.Contains(err.Error(), "build plan was not constructed") {
		t.Fatalf("expected build plan construction error, got %v", err)
	}
}

func TestBuildFromPlanDoesNotPublishBeforeManifestPlanningSucceeds(t *testing.T) {
	outputDir := t.TempDir()
	plannedCSSPath := filepath.Join(outputDir, "assets", "site.css")
	plannedPagePath := filepath.Join(outputDir, "index.html")
	plan := BuildPlan{
		reporter:  newBuildReporter("build", outputDir),
		outputDir: outputDir,
		valid:     true,
		planned: buildPlan{
			css: []plannedCSSArtifact{{
				CSSArtifact: CSSArtifact{Path: plannedCSSPath},
				contents:    []byte("body{color:red}"),
			}},
			pages: []plannedArtifact{{
				Artifact: Artifact{PageID: "home", Route: "/", Path: filepath.Join(filepath.Dir(outputDir), "outside.html")},
				contents: []byte("<main>new</main>"),
			}},
		},
	}

	_, err := BuildFromPlan(plan)
	if err == nil || !strings.Contains(err.Error(), "must stay inside output directory") {
		t.Fatalf("expected manifest planning error, got %v", err)
	}
	for _, path := range []string{plannedCSSPath, plannedPagePath} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to stay unpublished, stat err=%v", path, statErr)
		}
	}
}

func TestBuildMemoryFromIRCollectsArtifacts(t *testing.T) {
	config := gowdk.Config{}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/home.page.gwdk",
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	result, err := BuildMemoryFromIR(config, gwdkanalysis.BuildProgram(config, app), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].PageID != "home" {
		t.Fatalf("unexpected artifacts: %#v", result.Artifacts)
	}
	if !strings.Contains(string(result.Files["index.html"]), "<main>Home</main>") {
		t.Fatalf("expected generated page content, got:\n%s", result.Files["index.html"])
	}
}

func TestBuildMemoryFromIRWithOptionsDoesNotRequireOutputDir(t *testing.T) {
	config := gowdk.Config{}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  "pages/home.page.gwdk",
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	result, err := BuildMemoryFromIRWithOptions(config, gwdkanalysis.BuildProgram(config, app), MemoryBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.OutputDir != "." {
		t.Fatalf("expected virtual output base '.', got %q", result.Report.OutputDir)
	}
	if result.RouteManifestPath != routeManifestFile {
		t.Fatalf("expected relative route manifest path, got %q", result.RouteManifestPath)
	}
	if !strings.Contains(string(result.Files["index.html"]), "<main>Home</main>") {
		t.Fatalf("expected generated page content, got:\n%s", result.Files["index.html"])
	}
}
