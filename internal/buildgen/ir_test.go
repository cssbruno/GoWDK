package buildgen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
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

func TestBuildFromValidatedIRChecksInvariants(t *testing.T) {
	_, err := BuildFromValidatedIR(gowdk.Config{}, gwdkir.Program{}, t.TempDir())
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
