package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestPlanFromIRMatchesManifestPlan(t *testing.T) {
	config := gowdk.Config{}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			Source:  "pages/home.page.gwdk",
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: manifest.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Home" }`,
				View:      true,
				ViewBody:  `<main>{title}</main>`,
			},
		}},
	}

	fromManifest, err := plan(config, app, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fromIR, err := planFromIR(config, gwdkanalysis.BuildIR(config, app), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if len(fromManifest.pages) != len(fromIR.pages) {
		t.Fatalf("page plan count mismatch: %d != %d", len(fromManifest.pages), len(fromIR.pages))
	}
	if fromIR.pages[0].PageID != "home" || fromIR.pages[0].Route != "/" {
		t.Fatalf("unexpected IR page plan: %#v", fromIR.pages[0])
	}
	if string(fromIR.pages[0].contents) != string(fromManifest.pages[0].contents) {
		t.Fatalf("IR plan content differs:\n%s\n---\n%s", fromIR.pages[0].contents, fromManifest.pages[0].contents)
	}
}

func TestBuildFromIRWritesArtifacts(t *testing.T) {
	config := gowdk.Config{}
	app := manifest.Manifest{Pages: []manifest.Page{{
		Source:  "pages/home.page.gwdk",
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}
	outputDir := t.TempDir()

	result, err := BuildFromIR(config, gwdkanalysis.BuildIR(config, app), outputDir)
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

func TestBuildMemoryFromIRCollectsArtifacts(t *testing.T) {
	config := gowdk.Config{}
	app := manifest.Manifest{Pages: []manifest.Page{{
		Source:  "pages/home.page.gwdk",
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	result, err := BuildMemoryFromIR(config, gwdkanalysis.BuildIR(config, app), t.TempDir())
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
