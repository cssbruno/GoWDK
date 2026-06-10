package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/lang"
)

func TestBuildExpandsExplicitComponents(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "home",
			Route: "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" tagline="Portable & app-first" /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Name: "Hero",
			Props: []gwdkir.Prop{
				{Name: "title", Type: "string"},
				{Name: "tagline", Type: "string"},
			},
			Blocks: gwdkir.Blocks{
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

func TestBuildExpandsImportedGOWDKPackageComponent(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><ui.Hero title="GOWDK" /></main>`,
			},
		}},
		Components: []gwdkir.Component{
			{
				Package: "components",
				Name:    "Hero",
				Props:   []gwdkir.Prop{{Name: "title", Type: "string"}},
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<section><Badge label={title} /></section>`,
				},
			},
			{
				Package: "components",
				Name:    "Badge",
				Props:   []gwdkir.Prop{{Name: "label", Type: "string"}},
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<strong>{label}</strong>`,
				},
			},
		},
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
	for _, expected := range []string{
		`<section><gowdk-island data-gowdk-component="Badge"`,
		`<strong><span data-gowdk-bind="label" data-gowdk-binding-text="b1">GOWDK</span></strong>`,
		`<script src="/assets/gowdk/islands/Badge.js" defer></script>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in imported component output: %s", expected, output)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "assets", "gowdk", "islands", "Badge.js")); err != nil {
		t.Fatalf("expected imported child island asset: %v", err)
	}
	if strings.Contains(output, "ui.Hero.js") {
		t.Fatalf("expected island asset to use real component names, got: %s", output)
	}
	if !strings.Contains(output, `<main>`) {
		t.Fatalf("expected imported component output: %s", output)
	}
}

func TestBuildExpandsComponentScopedGOWDKPackageUse(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Marketing title="GOWDK" /></main>`,
			},
		}},
		Components: []gwdkir.Component{
			{
				Package: "pages",
				Name:    "Marketing",
				Uses:    []gwdkir.Use{{Alias: "icons", Package: "icons"}},
				Props:   []gwdkir.Prop{{Name: "title", Type: "string"}},
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<section><icons.Badge label={title} /></section>`,
				},
			},
			{
				Package: "icons",
				Name:    "Badge",
				Props:   []gwdkir.Prop{{Name: "label", Type: "string"}},
				Emits:   []gwdkir.Emit{{Name: "select"}},
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<strong>{label}</strong>`,
				},
			},
		},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	output := readFile(t, filepath.Join(outputDir, "index.html"))
	for _, expected := range []string{
		`<section><gowdk-island data-gowdk-component="Badge"`,
		`<strong><span data-gowdk-bind="label" data-gowdk-binding-text="b1">GOWDK</span></strong>`,
		`<script src="/assets/gowdk/islands/Badge.js" defer></script>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in component-scoped use output: %s", expected, output)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "assets", "gowdk", "islands", "Badge.js")); err != nil {
		t.Fatalf("expected component-scoped child island asset: %v", err)
	}
	if strings.Contains(output, "icons.Badge.js") {
		t.Fatalf("expected island asset to use real component names, got: %s", output)
	}
}

func TestBuildRejectsImportedPackageComponentByBarePageName(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "Hero",
			Props:   []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section>{title}</section>`},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected missing component error")
	}
	if !strings.Contains(err.Error(), `missing component "Hero"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildExpandsWrapperComponentSlots(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "home",
			Route: "/",
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Hello <slots>" }`,
				View:      true,
				ViewBody:  `<main><Panel title="Featured"><h1>{title}</h1></Panel></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Name:  "Panel",
			Props: []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks: gwdkir.Blocks{
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
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "home",
			Route:   "/",
			Layouts: []string{"root", "marketing"},
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { title: "GOWDK & layouts" }`,
				View:      true,
				ViewBody:  `<main>{title}</main>`,
			},
		}},
		Layouts: []gwdkir.Layout{
			{
				ID: "root",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<div .root><slot /></div>`,
				},
			},
			{
				ID: "marketing",
				Blocks: gwdkir.Blocks{
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

func TestBuildComposesQualifiedPageLayouts(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "chrome", Package: "layouts"}},
			Layouts: []string{"chrome.root", "local"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Home</main>`,
			},
		}},
		Layouts: []gwdkir.Layout{
			{
				Package: "layouts",
				ID:      "root",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<div .root><slot /></div>`,
				},
			},
			{
				Package: "pages",
				ID:      "local",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<section .local><slot /></section>`,
				},
			},
		},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	want := `<div class="root"><section class="local"><main>Home</main></section></div>`
	if !strings.Contains(html, want) {
		t.Fatalf("expected composed qualified layouts %q in:\n%s", want, html)
	}
}

func TestBuildRejectsLayoutWithoutOneSlot(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "home",
			Route:   "/",
			Layouts: []string{"root"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Home</main>`,
			},
		}},
		Layouts: []gwdkir.Layout{{
			ID: "root",
			Blocks: gwdkir.Blocks{
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
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "home",
			Route: "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /><Missing /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Name: "Hero",
			Props: []gwdkir.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>{title}</section>`},
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
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "home",
			Route: "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /></main>`,
			},
		}},
		Components: []gwdkir.Component{
			{
				Name: "Hero",
				Props: []gwdkir.Prop{
					{Name: "title", Type: "string"},
				},
				Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>{title}</section>`},
			},
			{
				Name:   "Hero",
				Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>Duplicate</section>`},
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
