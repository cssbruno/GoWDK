package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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

func TestBuildModelFromIRPreservesFragmentEndpoints(t *testing.T) {
	app := compiler.ManifestFromIR(gwdkir.Program{
		Version: gwdkir.Version,
		Pages: []gwdkir.Page{{
			ID:      "patients",
			Route:   "/patients",
			Package: "pages",
			Blocks: gwdkir.Blocks{
				Fragments: []gwdkir.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/patients/list",
					Target: "#patients",
					Body:   "<section>Patients</section>",
				}},
				Spans: gwdkir.BlockSpans{
					Fragments: []manifest.NamedSpan{{Name: "List"}},
				},
			},
		}},
	})

	if len(app.Pages) != 1 || len(app.Pages[0].Blocks.Fragments) != 1 {
		t.Fatalf("expected fragment endpoint in manifest model, got %#v", app.Pages)
	}
	fragment := app.Pages[0].Blocks.Fragments[0]
	if fragment.Name != "List" || fragment.Route != "/patients/list" || fragment.Target != "#patients" || fragment.Body != "<section>Patients</section>" {
		t.Fatalf("unexpected fragment endpoint: %#v", fragment)
	}
	if len(app.Pages[0].Blocks.Spans.Fragments) != 1 || app.Pages[0].Blocks.Spans.Fragments[0].Name != "List" {
		t.Fatalf("expected fragment spans, got %#v", app.Pages[0].Blocks.Spans.Fragments)
	}
}
