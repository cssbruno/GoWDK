package staticgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/lang"
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
}

func TestBuildPreservesUnchangedArtifactModTimes(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main><h1>Home</h1></main>`,
		},
	}}}

	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(outputDir, "index.html"),
		filepath.Join(outputDir, routeManifestFile),
		filepath.Join(outputDir, assetManifestFile),
	}
	first := map[string]time.Time{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		first[path] = info.ModTime()
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(first[path]) {
			t.Fatalf("expected unchanged mod time for %s: before=%s after=%s", path, first[path], info.ModTime())
		}
	}
}

func TestBuildIncrementalRendersOnlyChangedPageSources(t *testing.T) {
	outputDir := t.TempDir()
	homeSource := filepath.Join(t.TempDir(), "home.page.gwdk")
	aboutSource := filepath.Join(t.TempDir(), "about.page.gwdk")
	initial := manifest.Manifest{Pages: []manifest.Page{
		{
			Source: homeSource,
			ID:     "home",
			Route:  "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Home before</main>`,
			},
		},
		{
			Source: aboutSource,
			ID:     "about",
			Route:  "/about",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>About stable</main>`,
			},
		},
	}}
	if _, err := Build(gowdk.Config{}, initial, outputDir); err != nil {
		t.Fatal(err)
	}
	aboutPath := filepath.Join(outputDir, "about", "index.html")
	aboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	changed := manifest.Manifest{Pages: []manifest.Page{
		{
			Source: homeSource,
			ID:     "home",
			Route:  "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Home after</main>`,
			},
		},
		{
			Source: aboutSource,
			ID:     "about",
			Route:  "/about",
			Blocks: manifest.Blocks{
				Build:     true,
				BuildBody: `=> missing.BuildData()`,
				View:      true,
				ViewBody:  `<main>About stable</main>`,
			},
		},
	}}
	result, err := BuildIncremental(gowdk.Config{}, changed, outputDir, []string{homeSource})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("expected route manifest artifacts for all pages, got %#v", result.Artifacts)
	}
	if html := readFile(t, filepath.Join(outputDir, "index.html")); !strings.Contains(html, "Home after") {
		t.Fatalf("expected changed home output, got:\n%s", html)
	}
	afterAboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterAboutInfo.ModTime().Equal(aboutInfo.ModTime()) {
		t.Fatalf("expected unchanged about output mod time: before=%s after=%s", aboutInfo.ModTime(), afterAboutInfo.ModTime())
	}
	routes := readRouteManifest(t, outputDir)
	if len(routes.Routes) != 2 {
		t.Fatalf("expected both routes in route manifest, got %#v", routes.Routes)
	}
}

func TestBuildIncrementalRemovesStaleChangedPageRouteOutput(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "home.page.gwdk")
	initial := manifest.Manifest{Pages: []manifest.Page{{
		Source: source,
		ID:     "home",
		Route:  "/old",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Old route</main>`,
		},
	}}}
	if _, err := Build(gowdk.Config{}, initial, outputDir); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(outputDir, "old", "index.html")
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal(err)
	}

	changed := manifest.Manifest{Pages: []manifest.Page{{
		Source: source,
		ID:     "home",
		Route:  "/new",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>New route</main>`,
		},
	}}}
	if _, err := BuildIncremental(gowdk.Config{}, changed, outputDir, []string{source}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old route output to be removed, stat err: %v", err)
	}
	if html := readFile(t, filepath.Join(outputDir, "new", "index.html")); !strings.Contains(html, "New route") {
		t.Fatalf("expected new route output, got:\n%s", html)
	}
}

func TestBuildExpandsExplicitComponents(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" tagline="Portable & static" /></main>`,
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
	if !strings.Contains(output, `<section><h1>GOWDK</h1><p>Portable &amp; static</p></section>`) {
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

func TestBuildEmitsConfiguredStylesheetLinks(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{
		Build: gowdk.BuildConfig{
			Stylesheets: []gowdk.Stylesheet{
				{Href: "/assets/app.css"},
				{Href: "/assets/theme.css?version=1&mode=dark"},
				{},
			},
		},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<link rel="stylesheet" href="/assets/app.css">`,
		`<link rel="stylesheet" href="/assets/theme.css?version=1&amp;mode=dark">`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in output:\n%s", expected, output)
		}
	}
}

func TestBuildDiscoversAndLinksPageCSS(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("styles", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "styles", "global.css"), "body { color: black; }\n")
	writeFile(t, filepath.Join(root, "styles", "home.css"), ".home { display: grid; }\n")
	writeFile(t, filepath.Join(root, "styles", "reset.css"), "* { box-sizing: border-box; }\n")
	writeFile(t, filepath.Join(root, "styles", "tokens.css"), ":root { --brand: blue; }\n")
	writeFile(t, filepath.Join(root, "styles", "forms.css"), "input { font: inherit; }\n")

	outputDir := filepath.Join(root, "dist")
	app := manifest.Manifest{Pages: []manifest.Page{
		{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Home</main>`,
			},
		},
		{
			ID:    "dashboard",
			Route: "/dashboard",
			CSS:   []string{"reset", "tokens", "forms"},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Dashboard</main>`,
			},
		},
		{
			ID:    "embed",
			Route: "/embed",
			CSS:   []string{"none"},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Embed</main>`,
			},
		},
	}}

	result, err := Build(gowdk.Config{
		CSS: gowdk.CSSConfig{
			Include: []string{"styles/*.css"},
		},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CSSArtifacts) != 2 {
		t.Fatalf("expected page css for home and dashboard, got %#v", result.CSSArtifacts)
	}

	homeHTML := readFile(t, filepath.Join(outputDir, "index.html"))
	if !strings.Contains(homeHTML, `<link rel="stylesheet" href="/assets/gowdk/home.css">`) {
		t.Fatalf("expected home page css link:\n%s", homeHTML)
	}
	homeCSS := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "home.css"))
	for _, expected := range []string{"gowdk css: global", "body { color: black; }", "gowdk css: home", ".home { display: grid; }"} {
		if !strings.Contains(homeCSS, expected) {
			t.Fatalf("expected %q in home css:\n%s", expected, homeCSS)
		}
	}
	if strings.Contains(homeCSS, "input { font: inherit; }") {
		t.Fatalf("did not expect forms css in default home css:\n%s", homeCSS)
	}

	dashboardHTML := readFile(t, filepath.Join(outputDir, "dashboard", "index.html"))
	if !strings.Contains(dashboardHTML, `<link rel="stylesheet" href="/assets/gowdk/dashboard.css">`) {
		t.Fatalf("expected dashboard page css link:\n%s", dashboardHTML)
	}
	dashboardCSS := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "dashboard.css"))
	for _, expected := range []string{"gowdk css: reset", "gowdk css: tokens", "gowdk css: forms"} {
		if !strings.Contains(dashboardCSS, expected) {
			t.Fatalf("expected %q in dashboard css:\n%s", expected, dashboardCSS)
		}
	}
	if strings.Contains(dashboardCSS, "gowdk css: global") {
		t.Fatalf("did not expect global css in exact dashboard selection:\n%s", dashboardCSS)
	}

	embedHTML := readFile(t, filepath.Join(outputDir, "embed", "index.html"))
	if strings.Contains(embedHTML, "/assets/gowdk/embed.css") {
		t.Fatalf("did not expect embed page css link:\n%s", embedHTML)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "assets", "gowdk", "embed.css")); !os.IsNotExist(err) {
		t.Fatalf("expected no embed css file, stat err: %v", err)
	}
}

func TestBuildRejectsUnknownPageCSSReferenceBeforeWriting(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("styles", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "styles", "global.css"), "body {}\n")

	outputDir := filepath.Join(root, "dist")
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		CSS:   []string{"missing"},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{
		CSS: gowdk.CSSConfig{
			Include: []string{"styles/*.css"},
		},
	}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown css reference error")
	}
	if !strings.Contains(err.Error(), `home: unknown css input "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRendersLiteralBuildData(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Portable Go web compiler", slug: "home" }`,
				View:      true,
				ViewBody:  `<main data-page="{slug}"><Hero title="{title}" /></main>`,
			},
		}},
		Components: []manifest.Component{{
			Name: "Hero",
			Props: []manifest.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section><h1>{title}</h1></section>`,
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
	if !strings.Contains(output, `<main data-page="home"><section><h1>Portable Go web compiler</h1></section></main>`) {
		t.Fatalf("expected build data in output:\n%s", output)
	}
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

func TestBuildLowersGPostDirectiveForActionPage(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "signup",
		Route:  "/signup",
		Render: gowdk.Action,
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<form g:post={submit}><input name="email" /></form>`,
			Actions: []manifest.Action{{
				Name:     "submit",
				Redirect: "/signup?ok=1",
			}},
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "signup", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<form method="post" action="/signup"><input name="email"></input></form>`) {
		t.Fatalf("expected lowered g:post form in output:\n%s", output)
	}
}

func TestBuildEmitsPartialRuntimeForFragmentForms(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: manifest.Blocks{
			View: true,
			ViewBody: `<main>
  <form g:post={refresh} g:target="#patients" g:swap="innerHTML"><input name="query" /></form>
  <section id="patients">Initial</section>
</main>`,
			Actions: []manifest.Action{{
				Name: "refresh",
				Fragments: []manifest.Fragment{{
					Target: "#patients",
					Body:   `<p>Updated</p>`,
				}},
			}},
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AssetArtifacts) != 1 || result.AssetArtifacts[0].Path != filepath.Join(outputDir, "assets", "gowdk", "gowdk.js") {
		t.Fatalf("unexpected runtime assets: %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "patients", "index.html"))
	for _, expected := range []string{
		`<form method="post" action="/patients" data-gowdk-target="#patients" data-gowdk-swap="innerHTML">`,
		`<script src="/assets/gowdk/gowdk.js" defer></script>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in partial page:\n%s", expected, html)
		}
	}
	runtime := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "gowdk.js"))
	if !strings.Contains(runtime, `X-GOWDK-Partial`) {
		t.Fatalf("expected client runtime source, got:\n%s", runtime)
	}

	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	if !strings.Contains(assetManifestPayload, `"assets/gowdk/gowdk.js": "assets/gowdk/gowdk.js"`) {
		t.Fatalf("expected runtime in asset manifest:\n%s", assetManifestPayload)
	}
}

func TestBuildRejectsUnknownGPostActionBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "signup",
		Route: "/signup",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<form g:post={missing}></form>`,
			Actions: []manifest.Action{{
				Name:     "submit",
				Redirect: "/signup?ok=1",
			}},
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown g:post action error")
	}
	if !strings.Contains(err.Error(), `signup: unknown action "missing" for g:post`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRejectsInvalidBuildDataBeforeWriting(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "malformed",
			body:      `title: "Home"`,
			wantError: `build line 1 must use`,
		},
		{
			name: "multiple returns",
			body: `=> { title: "Home" }
=> { tagline: "Second" }`,
			wantError: `build {} supports one literal data declaration`,
		},
		{
			name:      "duplicate field",
			body:      `=> { title: "Home", title: "Again" }`,
			wantError: `duplicate build field "title"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir()
			app := manifest.Manifest{Pages: []manifest.Page{{
				ID:    "home",
				Route: "/",
				Blocks: manifest.Blocks{
					Build:     true,
					BuildBody: tt.body,
					View:      true,
					ViewBody:  `<main>Home</main>`,
				},
			}}}

			_, err := Build(gowdk.Config{}, app, outputDir)
			if err == nil {
				t.Fatal("expected build data error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
			if entries, err := os.ReadDir(outputDir); err != nil {
				t.Fatal(err)
			} else if len(entries) != 0 {
				t.Fatalf("expected no partial output, got %#v", entries)
			}
		})
	}
}

func TestBuildRejectsBuildDataRouteParamConflictBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			Build:     true,
			BuildBody: `=> { slug: "conflict" }`,
			View:      true,
			ViewBody:  `<main>{slug}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected build data route param conflict")
	}
	if !strings.Contains(err.Error(), `build data field "slug" conflicts with route param`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildMergesBuildDataWithDynamicRouteParams(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			Build:     true,
			BuildBody: `=> { title: "Post" }`,
			View:      true,
			ViewBody:  `<main><h1>{title}</h1><p>{slug}</p></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "blog", "hello-gowdk", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `<main><h1>Post</h1><p>hello-gowdk</p></main>`) {
		t.Fatalf("expected build data and route param in output:\n%s", payload)
	}
}

func TestBuildBindsRouteParamsIntoBuildDataValues(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			Build:     true,
			BuildBody: `=> { title: "Post {slug}", canonical: "/blog/{param(\"slug\")}" }`,
			View:      true,
			ViewBody:  `<main data-canonical="{canonical}"><h1>{title}</h1></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "blog", "hello-gowdk", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `<main data-canonical="/blog/hello-gowdk"><h1>Post hello-gowdk</h1></main>`) {
		t.Fatalf("expected route params in build data output:\n%s", payload)
	}
}

func TestBuildUsesImportedGoBuildData(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "go.imported",
		Route: "/go-imported",
		Imports: []manifest.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: manifest.Blocks{
			Build:     true,
			BuildBody: `=> interop.FeaturedCopyForBuild()`,
			View:      true,
			ViewBody:  `<main><h1>{title}</h1><p>{tagline}</p></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "go-imported", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<h1>Imported Go data</h1>`,
		`<p>This page rendered data from a Go package imported directly in .gwdk.</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in imported build data output:\n%s", expected, output)
		}
	}
}

func TestBuildRejectsMissingGoBuildDataImport(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "go.interop",
		Route: "/go-interop",
		Blocks: manifest.Blocks{
			Build:     true,
			BuildBody: `=> interop.FeaturedCopyForBuild()`,
			View:      true,
			ViewBody:  `<main>{title}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected missing import error")
	}
	if !strings.Contains(err.Error(), `build import "interop" is not declared`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRejectsUnknownRouteParamInBuildDataValue(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			Build:     true,
			BuildBody: `=> { title: "Post {missing}" }`,
			View:      true,
			ViewBody:  `<main>{title}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown route param error")
	}
	if !strings.Contains(err.Error(), `build field title: unknown route param "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRendersExplicitRouteParamReferences(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			View:      true,
			ViewBody:  `<main data-slug="{param(\"slug\")}"><h1>{param("slug")}</h1></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "blog", "hello-gowdk", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `<main data-slug="hello-gowdk"><h1>hello-gowdk</h1></main>`) {
		t.Fatalf("expected route param reference in output:\n%s", payload)
	}
}

func TestBuildRejectsUndeclaredRouteParamReferenceBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			View:      true,
			ViewBody:  `<main>{param("missing")}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected undeclared route param reference error")
	}
	if !strings.Contains(err.Error(), `view references route param "missing" that is not declared by route "/blog/{slug}"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRejectsRouteParamInDangerousAttributeBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "alert(1)" }`,
			View:      true,
			ViewBody:  `<img src="x" onerror="{param(\"slug\")}" />`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected dangerous route param attribute error")
	}
	if !strings.Contains(err.Error(), `route param interpolation is not allowed in "onerror" attributes`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildInvokesCSSProcessorAndWritesAssets(t *testing.T) {
	outputDir := t.TempDir()
	processor := &recordingCSSProcessor{}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			Source: "pages/home.page.gwdk",
			ID:     "home",
			Route:  "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main .home-shell>Home</main>`,
			},
		}},
		Components: []manifest.Component{{
			Source: "components/hero.cmp.gwdk",
			Name:   "Hero",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section class="hero-card">Hero</section>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{
		Build: gowdk.BuildConfig{Output: outputDir},
		CSS:   gowdk.CSSConfig{Include: []string{"styles/**/*.css"}},
		Addons: []gowdk.Addon{
			processor,
		},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if processor.calls != 1 {
		t.Fatalf("expected one css processor call, got %d", processor.calls)
	}
	if len(processor.sources) != 2 || processor.sources[0].Kind != "page" || processor.sources[1].Kind != "component" {
		t.Fatalf("unexpected css sources: %#v", processor.sources)
	}
	if strings.Join(processor.sources[0].CSSClasses, ",") != "home-shell" || strings.Join(processor.sources[1].CSSClasses, ",") != "hero-card" {
		t.Fatalf("unexpected css source classes: %#v", processor.sources)
	}
	if processor.context.Build.Output != outputDir || strings.Join(processor.context.CSS.Include, ",") != "styles/**/*.css" {
		t.Fatalf("unexpected css processor config context: %#v", processor.context)
	}
	if len(result.CSSArtifacts) != 1 {
		t.Fatalf("expected one css artifact, got %#v", result.CSSArtifacts)
	}
	cssPath := filepath.Join(outputDir, "assets", "app.css")
	if result.CSSArtifacts[0].Path != cssPath {
		t.Fatalf("expected css path %s, got %s", cssPath, result.CSSArtifacts[0].Path)
	}
	payload, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "body{color:black}\n" {
		t.Fatalf("unexpected css payload: %q", payload)
	}
	html, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<link rel="stylesheet" href="/assets/app.css">`) {
		t.Fatalf("expected css link in html:\n%s", html)
	}
	assetManifestPayload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(assetManifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Version != 1 || assets.Resolve("assets/app.css") != "assets/app.css" {
		t.Fatalf("unexpected asset manifest: %s", assetManifestPayload)
	}
}

func TestBuildRejectsUnsafeCSSAssetsBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{Addons: []gowdk.Addon{badCSSProcessor{path: "../app.css"}}}, app, outputDir)
	if err == nil {
		t.Fatal("expected css asset path error")
	}
	if !strings.Contains(err.Error(), `css asset path "../app.css" must stay inside output directory`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRejectsDuplicateCSSAssetsBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{Addons: []gowdk.Addon{duplicateCSSProcessor{}}}, app, outputDir)
	if err == nil {
		t.Fatal("expected duplicate css asset path error")
	}
	if !strings.Contains(err.Error(), `duplicate css asset path "assets/app.css"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
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

type recordingCSSProcessor struct {
	calls   int
	sources []gowdk.CSSSource
	context gowdk.CSSContext
}

func (processor *recordingCSSProcessor) Name() string {
	return "recording-css"
}

func (processor *recordingCSSProcessor) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (processor *recordingCSSProcessor) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	processor.calls++
	processor.sources = append([]gowdk.CSSSource(nil), context.Sources...)
	processor.context = context
	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{{
			Path:     "assets/app.css",
			Contents: []byte("body{color:black}\n"),
		}},
		Stylesheets: []gowdk.Stylesheet{{Href: "/assets/app.css"}},
	}, nil
}

type badCSSProcessor struct {
	path string
}

type duplicateCSSProcessor struct{}

func (processor duplicateCSSProcessor) Name() string {
	return "duplicate-css"
}

func (processor duplicateCSSProcessor) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (processor duplicateCSSProcessor) ProcessCSS(gowdk.CSSContext) (gowdk.CSSResult, error) {
	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{
			{Path: "assets/app.css", Contents: []byte("one")},
			{Path: "assets/app.css", Contents: []byte("two")},
		},
	}, nil
}

func (processor badCSSProcessor) Name() string {
	return "bad-css"
}

func (processor badCSSProcessor) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (processor badCSSProcessor) ProcessCSS(gowdk.CSSContext) (gowdk.CSSResult, error) {
	if processor.path == "error" {
		return gowdk.CSSResult{}, fmt.Errorf("failed")
	}
	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{{Path: processor.path, Contents: []byte("body{}")}},
	}, nil
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

func TestBuildWritesNestedRouteIndex(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Patients</main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(outputDir, "patients", "index.html")
	if result.Artifacts[0].Path != wantPath {
		t.Fatalf("expected %s, got %s", wantPath, result.Artifacts[0].Path)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatal(err)
	}
}

func TestBuildExpandsDynamicStaticPaths(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "blog.post",
			Route: "/blog/{slug}",
			Paths: true,
			Blocks: manifest.Blocks{
				PathsBody: `=> { slug: "hello-gowdk" }
=> { slug: "static-first" }`,
				View:     true,
				ViewBody: `<main data-slug="{slug}"><h1>{slug}</h1><PostTitle title="{slug}" /></main>`,
			},
		}},
		Components: []manifest.Component{{
			Name: "PostTitle",
			Props: []manifest.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<p>{title}</p>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("expected two dynamic artifacts, got %#v", result.Artifacts)
	}
	for _, slug := range []string{"hello-gowdk", "static-first"} {
		path := filepath.Join(outputDir, "blog", slug, "index.html")
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf(`<main data-slug="%s"><h1>%s</h1><p>%s</p></main>`, slug, slug, slug)
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("unexpected dynamic output for %s:\n%s", slug, payload)
		}
	}

	routes := readRouteManifest(t, outputDir)
	if len(routes.Routes) != 2 {
		t.Fatalf("expected two route manifest entries, got %#v", routes.Routes)
	}
	seen := map[string]string{}
	for _, route := range routes.Routes {
		seen[route.Route] = route.Path
	}
	if seen["/blog/hello-gowdk"] != "blog/hello-gowdk/index.html" {
		t.Fatalf("missing hello route in manifest: %#v", seen)
	}
	if seen["/blog/static-first"] != "blog/static-first/index.html" {
		t.Fatalf("missing static route in manifest: %#v", seen)
	}
}

func TestBuildRejectsUnknownDynamicInterpolationBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Paths: true,
		Blocks: manifest.Blocks{
			PathsBody: `=> { slug: "hello-gowdk" }`,
			View:      true,
			ViewBody:  `<main>{missing}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(err.Error(), `unknown interpolation "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}

func TestBuildRejectsInvalidDynamicPathsBeforeWriting(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "malformed",
			body:      `slug: "hello-gowdk"`,
			wantError: `paths line 1 must use`,
		},
		{
			name:      "missing param",
			body:      `=> { title: "hello-gowdk" }`,
			wantError: `missing route param "slug"`,
		},
		{
			name:      "unused param",
			body:      `=> { slug: "hello-gowdk", extra: "ignored" }`,
			wantError: `unused route param "extra"`,
		},
		{
			name:      "unsafe segment",
			body:      `=> { slug: "../SECRET_TOKEN" }`,
			wantError: `must not contain /, ?, or #`,
		},
		{
			name: "duplicate output",
			body: `=> { slug: "hello-gowdk" }
=> { slug: "hello-gowdk" }`,
			wantError: `generated output path "blog/hello-gowdk/index.html" duplicates page blog.post`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir()
			app := manifest.Manifest{Pages: []manifest.Page{{
				ID:    "blog.post",
				Route: "/blog/{slug}",
				Paths: true,
				Blocks: manifest.Blocks{
					PathsBody: tt.body,
					View:      true,
					ViewBody:  `<main>Post</main>`,
				},
			}}}

			_, err := Build(gowdk.Config{}, app, outputDir)
			if err == nil {
				t.Fatal("expected build error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
			if strings.Contains(err.Error(), "SECRET_TOKEN") {
				t.Fatalf("expected build error to omit sensitive path param value, got %v", err)
			}
			if entries, err := os.ReadDir(outputDir); err != nil {
				t.Fatal(err)
			} else if len(entries) != 0 {
				t.Fatalf("expected no partial output, got %#v", entries)
			}
		})
	}
}

func TestBuildSkipsRequestTimePagesAndKeepsStaticArtifacts(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Dashboard</main>`,
			},
		},
		{
			ID:    "blog.post",
			Route: "/blog/{slug}",
			Paths: true,
			Blocks: manifest.Blocks{
				PathsBody: `=> { slug: "hello-gowdk" }`,
				View:      true,
				ViewBody:  `<main>Post</main>`,
			},
		},
	}}

	result, err := Build(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected only one static artifact, got %#v", result.Artifacts)
	}
	if result.Artifacts[0].PageID != "blog.post" {
		t.Fatalf("expected SSR page to be skipped, got %#v", result.Artifacts)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "dashboard", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no SSR static output, stat err: %v", err)
	}
}

func TestSSRArtifactsRenderConcreteSSRPage(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				BuildBody: `=> { title: "Dashboard" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1><p>Live</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	if artifacts[0].PageID != "dashboard" || artifacts[0].Route != "/dashboard" {
		t.Fatalf("unexpected SSR artifact metadata: %#v", artifacts[0])
	}
	if !strings.Contains(artifacts[0].HTML, "<h1>Dashboard</h1>") {
		t.Fatalf("expected rendered SSR HTML, got %s", artifacts[0].HTML)
	}
}

func TestSSRArtifactsRenderDynamicSSRPageWithPlaceholders(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				BuildBody: `=> { title: "Post {slug}" }`,
				View:      true,
				ViewBody:  `<main data-slug="{param(\"slug\")}"><h1>{title}</h1><p>{param("slug")}</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if artifact.Route != "/blog/{slug}" {
		t.Fatalf("unexpected dynamic route: %#v", artifact)
	}
	if len(artifact.Replacements) != 1 || artifact.Replacements[0].Param != "slug" {
		t.Fatalf("unexpected replacements: %#v", artifact.Replacements)
	}
	if !strings.Contains(artifact.HTML, artifact.Replacements[0].Placeholder) {
		t.Fatalf("expected SSR HTML placeholder %q in %s", artifact.Replacements[0].Placeholder, artifact.HTML)
	}
}

func TestSSRArtifactsRejectRouteParamInDangerousAttribute(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<img src="x" onerror="{param(\"slug\")}" />`,
			},
		}},
	}

	_, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err == nil {
		t.Fatal("expected dangerous route param attribute error")
	}
	if !strings.Contains(err.Error(), `route param interpolation is not allowed in "onerror" attributes`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSRArtifactsRejectLoadUntilRequestExecutionExists(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			Load:     true,
			LoadBody: `=> { user }`,
			View:     true,
			ViewBody: `<main>Dashboard</main>`,
		},
	}}}

	_, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err == nil {
		t.Fatal("expected unsupported load error")
	}
	if !strings.Contains(err.Error(), "generated SSR load {} execution is not implemented yet") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type testRouteManifest struct {
	Version int `json:"version"`
	Routes  []struct {
		PageID string `json:"page"`
		Route  string `json:"route"`
		Path   string `json:"path"`
	} `json:"routes"`
}

func readRouteManifest(t *testing.T, outputDir string) testRouteManifest {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(outputDir, routeManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var routes testRouteManifest
	if err := json.Unmarshal(payload, &routes); err != nil {
		t.Fatal(err)
	}
	return routes
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func assertOutputMatchesFixture(t *testing.T, outputDir, relativePath string) {
	t.Helper()
	actual, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "full_fixture", "expected", filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Fatalf("generated output mismatch for %s\nexpected:\n%s\nactual:\n%s", relativePath, expected, actual)
	}
}
