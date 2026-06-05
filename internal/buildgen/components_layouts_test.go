package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildExpandsExplicitComponents(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" tagline="Portable & app-first" /></main>`,
			},
		}},
		Components: []manifest.Component{{
			Name: "Hero",
			Props: []manifest.Prop{
				{Name: "title", Type: "string"},
				{Name: "tagline", Type: "string"},
			},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section><h1>{title}</h1><p>{tagline}</p></section>`,
			},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<section><h1>GOWDK</h1><p>Portable &amp; app-first</p></section>`) {
		t.Fatalf("expected expanded component in output: %s", output)
	}
}

func TestBuildExpandsWrapperComponentSlots(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Hello <slots>" }`,
				View:      true,
				ViewBody:  `<main><Panel title="Featured"><h1>{title}</h1></Panel></main>`,
			},
		}},
		Components: []manifest.Component{{
			Name:  "Panel",
			Props: []manifest.Prop{{Name: "title", Type: "string"}},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section .panel><h2>{title}</h2><slot /></section>`,
			},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	want := `<main><section class="panel"><h2>Featured</h2><h1>Hello &lt;slots&gt;</h1></section></main>`
	if !strings.Contains(html, want) {
		t.Fatalf("expected wrapper component slot output %q in:\n%s", want, html)
	}
}

func TestBuildCompilesRealisticFixtureProjectEndToEnd(t *testing.T) {
	outputDir := t.TempDir()
	app, diagnostics := lang.ParseBuildFiles([]string{
		filepath.FromSlash("testdata/full_fixture/docs.page.gwdk"),
		filepath.FromSlash("testdata/full_fixture/hero.cmp.gwdk"),
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Route != "/docs/getting-started" {
		t.Fatalf("unexpected build artifacts: %#v", result.Artifacts)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "docs", "getting-started", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<section><h1>Getting Started</h1><p>Portable Go web compiler</p></section>`,
		`<p>getting-started</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in fixture output:\n%s", expected, output)
		}
	}
	assertOutputMatchesFixture(t, outputDir, "docs/getting-started/index.html")
	assertOutputMatchesFixture(t, outputDir, routeManifestFile)
	assertOutputMatchesFixture(t, outputDir, assetManifestFile)
}

func TestBuildComposesPageLayouts(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "home",
			Route:   "/",
			Layouts: []string{"root", "marketing"},
			Blocks: manifest.Blocks{
				Build:     true,
				BuildBody: `=> { title: "GOWDK & layouts" }`,
				View:      true,
				ViewBody:  `<main>{title}</main>`,
			},
		}},
		Layouts: []manifest.Layout{
			{
				ID: "root",
				Blocks: manifest.Blocks{
					View:     true,
					ViewBody: `<div .root><slot /></div>`,
				},
			},
			{
				ID: "marketing",
				Blocks: manifest.Blocks{
					View:     true,
					ViewBody: `<section .marketing><slot /></section>`,
				},
			},
		},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	want := `<div class="root"><section class="marketing"><main>GOWDK &amp; layouts</main></section></div>`
	if !strings.Contains(html, want) {
		t.Fatalf("expected composed layouts %q in:\n%s", want, html)
	}
}

func TestBuildRejectsLayoutWithoutOneSlot(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "home",
			Route:   "/",
			Layouts: []string{"root"},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Home</main>`,
			},
		}},
		Layouts: []manifest.Layout{{
			ID: "root",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<div><slot /><slot /></div>`,
			},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected layout slot error")
	}
	if !strings.Contains(err.Error(), `layout root must contain exactly one <slot /> placeholder`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRejectsMissingComponentBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /><Missing /></main>`,
			},
		}},
		Components: []manifest.Component{{
			Name: "Hero",
			Props: []manifest.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>{title}</section>`},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected build error")
	}
	message := err.Error()
	if !strings.Contains(message, `missing component "Missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRejectsDuplicateComponentsBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /></main>`,
			},
		}},
		Components: []manifest.Component{
			{
				Name: "Hero",
				Props: []manifest.Prop{
					{Name: "title", Type: "string"},
				},
				Blocks: manifest.Blocks{View: true, ViewBody: `<section>{title}</section>`},
			},
			{
				Name:   "Hero",
				Blocks: manifest.Blocks{View: true, ViewBody: `<section>Duplicate</section>`},
			},
		},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(err.Error(), `duplicate component name "Hero"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}
