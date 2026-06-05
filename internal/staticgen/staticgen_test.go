package staticgen

import (
	"bytes"
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

func TestBuildAllowsGPostWithLocalValueBinding(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<form g:post={submit}><input name="query" g:bind:value={Query} /></form>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "search",
			Route:  "/search",
			Render: gowdk.Action,
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
				Actions: []manifest.Action{{
					Name:     "submit",
					Redirect: "/search",
				}},
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Search.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "search", "index.html"))
	for _, expected := range []string{
		`<form method="post" action="/search">`,
		`name="query"`,
		`data-gowdk-bind-value="Query"`,
		`value="initial"`,
		`<script src="/assets/gowdk/islands/Search.js" defer></script>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in g:post binding page:\n%s", expected, html)
		}
	}
	if strings.Contains(html, `data-gowdk-on-submit`) || strings.Contains(html, `data-gowdk-event-submit`) {
		t.Fatalf("did not expect local value binding to add submit event interception:\n%s", html)
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

func TestBuildEmitsJSIslandAssetsForStatefulComponent(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent()},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.js" defer></script>`,
		`<gowdk-island data-gowdk-component="Counter" data-gowdk-runtime="js"`,
		`data-gowdk-on-click="Count++"`,
		`data-gowdk-bind="Count">1</span>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	if !strings.Contains(js, `data-gowdk-runtime=\"js\"`) || !strings.Contains(js, `applyExpression`) {
		t.Fatalf("expected generated JS island runtime, got:\n%s", js)
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	if !strings.Contains(assetManifestPayload, `"assets/gowdk/islands/Counter.js": "assets/gowdk/islands/Counter.js"`) {
		t.Fatalf("expected island JS in asset manifest:\n%s", assetManifestPayload)
	}
}

func TestBuildEmitsClientFunctionHandlersForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn Add(step int) {
  let next int = Count + step
  Count = next
}`
	component.Blocks.ViewBody = `<button g:on:click={Add(Count + 1)}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-client="{&#34;Add&#34;:{&#34;params&#34;:[&#34;step&#34;],&#34;statements&#34;:[&#34;let next int = Count + step&#34;,&#34;Count = next&#34;]}}"`,
		`data-gowdk-on-click="Add(Count + 1)"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	for _, expected := range []string{
		`data-gowdk-client`,
		`nextScope[param] = valueOf(args[index] || "", state, scope, helpers);`,
		`let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);`,
		`scope[local[1]] = valueOf(local[2], state, scope, helpers);`,
		`with (env) { return (`,
		`applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsClientHelperFunctionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn Next(value int) int {
  return value + 1
}

fn Add() {
  Count = Next(Count)
}`
	component.Blocks.ViewBody = `<button g:on:click={Add()}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`&#34;handlers&#34;:{&#34;Add&#34;:{&#34;statements&#34;:[&#34;Count = Next(Count)&#34;]}}`,
		`&#34;helpers&#34;:{&#34;Next&#34;:{&#34;params&#34;:[&#34;value&#34;],&#34;return&#34;:&#34;value + 1&#34;}}`,
		`data-gowdk-on-click="Add()"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	for _, expected := range []string{
		`function callHelper(name, args, state, helpers, stack)`,
		`env[name] = (...args) => callHelper(name, args, state, helpers, stack || []);`,
		`const helpers = client.helpers || {};`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsEventModifierRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button g:on:click.prevent.stop.once.capture.debounce(250ms)={Count++}>{Count}</button><button g:on:input.throttle(1s)={Count++}>Throttle</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-on-click="Count++"`,
		`data-gowdk-event-click="prevent stop once capture debounce:250"`,
		`data-gowdk-event-input="throttle:1000"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function eventModifiers(source)`,
		`if (modifiers.prevent) domEvent.preventDefault();`,
		`if (modifiers.stop) domEvent.stopPropagation();`,
		`debounceTimer = setTimeout(invoke, modifiers.debounce);`,
		`if (now < throttleUntil) return;`,
		`node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsLifecycleRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `on mount {
  Open = true
}

effect when Count {
  Open = false
  return {
    Open = true
  }
}

on destroy {
  Open = false
}`
	component.Blocks.ViewBody = `<button g:on:click={Count++}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`&#34;mount&#34;:[&#34;Open = true&#34;]`,
		`&#34;destroy&#34;:[&#34;Open = false&#34;]`,
		`&#34;effects&#34;:[{&#34;field&#34;:&#34;Count&#34;,&#34;statements&#34;:[&#34;Open = false&#34;],&#34;cleanup&#34;:[&#34;Open = true&#34;]}]`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const mountStatements = client.mount || [];`,
		`const destroyStatements = client.destroy || [];`,
		`const effects = client.effects || [];`,
		`const effectCleanups = Object.create(null);`,
		`const runEffectCleanup = (effect) => {`,
		`for (let pass = 0; pass < 10; pass++)`,
		`runEffectCleanup(effect);`,
		`effectCleanups[effect.field] = effect.cleanup || null;`,
		`applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds);`,
		`runAllEffectCleanups();`,
		`window.addEventListener("pagehide"`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsDOMRefRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`
	component.Blocks.ViewBody = `<input g:ref={searchInput} /><button g:on:click={FocusSearch()}>Focus</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "search",
			Route: "/search",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "search", "index.html"))
	for _, expected := range []string{
		`data-gowdk-ref="searchInput"`,
		`data-gowdk-on-click="FocusSearch()"`,
		`&#34;FocusSearch&#34;:{&#34;statements&#34;:[&#34;searchInput.Focus()&#34;]}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`root.querySelectorAll("[data-gowdk-ref]")`,
		`refs[node.getAttribute("data-gowdk-ref")] = node;`,
		`let refCall = expr.match`,
		`node.focus();`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsGIfRuntimeUpdatesForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<section g:if={Open}><button g:on:click={Open = !Open}>{Count}</button></section><section g:else>Closed</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if-group="c1" data-gowdk-if-index="0" data-gowdk-if="Open" hidden`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="1" data-gowdk-else`,
		`data-gowdk-on-click="Open = !Open"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const conditionalGroups = new Map();`,
		`root.querySelectorAll("[data-gowdk-if-group]")`,
		`const visible = !matched && (condition == null || Boolean(valueOf(condition, state, null, helpers)));`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsNestedAndIndexExpressionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := nestedComponent()
	component.Blocks.ViewBody = `<section g:if={User.Open && Items[0].Name == "first" && Flags[Count]}>{Count}</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "nested",
			Route: "/nested",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "nested", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="User.Open &amp;&amp; Items[0].Name == &#34;first&#34; &amp;&amp; Flags[Count]"`,
		`&#34;Items&#34;:[{&#34;Done&#34;:false,&#34;ID&#34;:&#34;first&#34;,&#34;Name&#34;:&#34;first&#34;},{&#34;Done&#34;:true,&#34;ID&#34;:&#34;second&#34;,&#34;Name&#34;:&#34;second&#34;}]`,
		`&#34;User&#34;:{&#34;Name&#34;:&#34;Ada&#34;,&#34;Open&#34;:true}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in nested island page:\n%s", expected, html)
		}
	}
	if strings.Contains(html, `data-gowdk-if="User.Open`) && strings.Contains(html, ` hidden`) {
		t.Fatalf("expected initial nested condition to render visible:\n%s", html)
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	if !strings.Contains(js, `with (env) { return (`) {
		t.Fatalf("expected expression lowering path in generated JS:\n%s", js)
	}
}

func TestBuildEmitsGForListRenderingForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := nestedComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn AddItem() {
  append(Items, { ID: "third", Name: "third", Done: false })
}

fn RemoveFirst() {
  remove(Items, 0)
}

fn SwapFirstTwo() {
  move(Items, 1, 0)
}`
	component.Blocks.ViewBody = `<ul><li g:for={item, i in Items} g:key={item.ID}><button g:on:click={remove(Items, i)}>{i}: {item.Name}</button></li></ul><button g:on:click={AddItem()}>Add</button><button g:on:click={RemoveFirst()}>Remove</button><button g:on:click={SwapFirstTwo()}>Swap</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "list",
			Route: "/list",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "list", "index.html"))
	for _, expected := range []string{
		`<template data-gowdk-for="l1" data-gowdk-for-var="item" data-gowdk-for-source="Items" data-gowdk-for-key="item.ID" data-gowdk-for-index-var="i"`,
		`data-gowdk-for-template="&lt;li data-gowdk-for-item=&#34;l1&#34; data-gowdk-key-value=&#34;{{item.ID}}&#34;&gt;&lt;button data-gowdk-on-click=&#34;remove(Items, i)&#34;&gt;{{i}}: {{item.Name}}&lt;/button&gt;&lt;/li&gt;"`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="first"><button data-gowdk-on-click="remove(Items, i)">0: first</button></li>`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="second"><button data-gowdk-on-click="remove(Items, i)">1: second</button></li>`,
		`data-gowdk-on-click="AddItem()"`,
		`append(Items, { ID: \&#34;third\&#34;, Name: \&#34;third\&#34;, Done: false })`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in g:for page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, expected := range []string{
		`function renderListLoops(root, state, helpers)`,
		`template[data-gowdk-for]`,
		`call[1] === "append"`,
		`state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);`,
		`state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));`,
		`next.splice(to, 0, item);`,
		`const existing = new Map();`,
		`syncElement(reused, fresh);`,
		`if (indexName) scope[indexName] = index;`,
		`const rerender = () => {`,
		`data-gowdk-bound-on-`,
		`interpolateTemplate(template, state, scope, helpers)`,
		`renderListLoops(root, state, helpers);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsGoishConditionalExpressionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn ToggleCount() {
  Count = if Open { Count + 1 } else { 0 }
}`
	component.Blocks.ViewBody = `<section g:if={if Open { Count > 0 } else { false }}><button g:on:click={ToggleCount()}>{Count}</button></section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="if Open { Count &gt; 0 } else { false }" hidden`,
		`&#34;ToggleCount&#34;:{&#34;statements&#34;:[&#34;Count = if Open { Count + 1 } else { 0 }&#34;]}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in conditional island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function expressionSource(source)`,
		`return "(" + expressionSource(cond) + " ? " + expressionSource(thenExpr) + " : " + expressionSource(elseExpr) + ")"`,
		`return Function("env", "with (env) { return (" + expressionSource(token) + "); }")(env);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsComputedStateForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `computed Label string {
  return if Open { "open" } else { "closed" }
}

computed Visible bool {
  return Label == "open"
}

fn Toggle() {
  Open = !Open
}`
	component.Blocks.ViewBody = `<section g:if={Visible}>{Label}<button g:on:click={Toggle()}>{Count}</button></section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="Visible" hidden`,
		`data-gowdk-bind="Label">closed</span>`,
		`&#34;computed&#34;:[{&#34;name&#34;:&#34;Label&#34;,&#34;expr&#34;:&#34;if Open { \&#34;open\&#34; } else { \&#34;closed\&#34; }&#34;},{&#34;name&#34;:&#34;Visible&#34;,&#34;expr&#34;:&#34;Label == \&#34;open\&#34;&#34;}]`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in computed island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function recomputeComputed(state, computeds, helpers)`,
		`state[computed.name] = valueOf(computed.expr, state, null, helpers);`,
		`const computeds = client.computed || [];`,
		`recomputeComputed(state, computeds, helpers);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildOrdersComputedDependenciesForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `computed Visible bool {
  return Label == "open"
}

computed Label string {
  return if Open { "open" } else { "closed" }
}`
	component.Blocks.ViewBody = `<section g:if={Visible}>{Label}</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	expected := `&#34;computed&#34;:[{&#34;name&#34;:&#34;Label&#34;,&#34;expr&#34;:&#34;if Open { \&#34;open\&#34; } else { \&#34;closed\&#34; }&#34;},{&#34;name&#34;:&#34;Visible&#34;,&#34;expr&#34;:&#34;Label == \&#34;open\&#34;&#34;}]`
	if !strings.Contains(html, expected) {
		t.Fatalf("expected dependency-ordered computed bootstrap in page:\n%s", html)
	}
}

func TestBuildEmitsClientBuiltinsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := manifest.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed ItemCount string {
  return string(len(Items))
}

fn SetCount() {
  Count = len(Items) + int("1")
}`,
			View:     true,
			ViewBody: `<button g:on:click={SetCount()}>{ItemCount}:{Count}</button>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "nested",
			Route: "/nested",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "nested", "index.html"))
	for _, expected := range []string{
		`>2:0</button>`,
		`data-gowdk-on-click="SetCount()"`,
		`Count = len(Items) + int(\&#34;1\&#34;)`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in built-in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, expected := range []string{
		`const builtins = Object.freeze({`,
		`len(value) {`,
		`string(value) {`,
		`int(value) {`,
		`float(value) {`,
		`Object.assign(Object.create(null), builtins, state, scope || {})`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "search",
			Route: "/search",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{textComponent()},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "search", "index.html"))
	for _, expected := range []string{
		`data-gowdk-bind-value="Query"`,
		`value="initial"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`[data-gowdk-bind-value]`,
		`state[field] = node.value;`,
		`const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";`,
		`node.addEventListener(event`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsTextareaAndSelectValueBindings(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<textarea g:bind:value={Query}></textarea><select g:bind:value={Query}><option value="other">Other</option><option value="initial">Initial</option></select>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "controls",
			Route: "/controls",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "controls", "index.html"))
	for _, expected := range []string{
		`<textarea data-gowdk-bind-value="Query">initial</textarea>`,
		`<option value="initial" selected>Initial</option>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in controls page:\n%s", expected, html)
		}
	}
}

func TestBuildEmitsNumericValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<input type="number" g:bind:value={Count} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "number",
			Route: "/number",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "number", "index.html"))
	for _, expected := range []string{
		`type="number"`,
		`data-gowdk-bind-value="Count"`,
		`data-gowdk-bind-type="int"`,
		`value="1"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in numeric binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const type = node.getAttribute("data-gowdk-bind-type") || "string";`,
		`parseInt(node.value, 10)`,
		`parseFloat(node.value)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsRadioValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<input type="radio" name="choice" value="other" g:bind:value={Query} /><input type="radio" name="choice" value="initial" g:bind:value={Query} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "radios",
			Route: "/radios",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "radios", "index.html"))
	for _, expected := range []string{
		`type="radio"`,
		`data-gowdk-bind-value="Query"`,
		`value="initial" checked`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in radio binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`node.checked = String(state[field] == null ? "" : state[field]) === node.value;`,
		`node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";`,
		`if (!node.checked) return;`,
		`state[field] = node.value;`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsCheckedBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<input type="checkbox" g:bind:checked={Open} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "toggle",
			Route: "/toggle",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "toggle", "index.html"))
	for _, expected := range []string{
		`type="checkbox"`,
		`data-gowdk-bind-checked="Open"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in checked binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`[data-gowdk-bind-checked]`,
		`state[field] = node.checked;`,
		`node.addEventListener("change"`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsReactiveAttributeRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button disabled={Open} aria-expanded={Open} g:on:click={Open = !Open}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "attrs",
			Route: "/attrs",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "attrs", "index.html"))
	for _, expected := range []string{
		`data-gowdk-attr-disabled="Open"`,
		`data-gowdk-attr-aria-expanded="Open"`,
		`aria-expanded="false"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in reactive attr page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-attr-`,
		`booleanAttrs.has(name)`,
		`node.setAttribute(name, String(value));`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsClassToggleRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button class="base" class:active={Open} g:on:click={Open = !Open}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "classes",
			Route: "/classes",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "classes", "index.html"))
	for _, expected := range []string{
		`data-gowdk-class-active="Open"`,
		`class="base"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in class toggle page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-class-`,
		`node.classList.toggle(name, Boolean(valueOf(attr.value, state, null, helpers)));`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsStyleBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<div style="color: red" style:height.px={Count} g:on:click={Count++}>{Count}</div>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "styles",
			Route: "/styles",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "styles", "index.html"))
	for _, expected := range []string{
		`data-gowdk-style-height="Count"`,
		`data-gowdk-style-unit-height="px"`,
		`style="color: red; height: 1px"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in style binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-style-`,
		`node.style.setProperty(name, String(value) + unit);`,
		`node.style.removeProperty(name);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildSerializesStateInitByGoFieldName(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "tagged",
			Route: "/tagged",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><TaggedCounter /></main>`,
			},
		}},
		Components: []manifest.Component{taggedCounterComponent()},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "tagged", "index.html"))
	for _, expected := range []string{
		`data-gowdk-state="{&#34;Count&#34;:0}"`,
		`data-gowdk-bind="Count">0</span>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in tagged state page:\n%s", expected, html)
		}
	}
}

func TestBuildEmitsWASMIslandAssetsOnlyWhenExplicit(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent()},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	wasmPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.wasm")
	loaderPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.wasm.js")
	if hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("did not expect default JS asset for explicit wasm usage: %#v", result.AssetArtifacts)
	}
	if !hasAssetArtifact(result.AssetArtifacts, wasmPath) || !hasAssetArtifact(result.AssetArtifacts, loaderPath) {
		t.Fatalf("expected wasm and loader assets, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.wasm.js" defer></script>`,
		`data-gowdk-runtime="wasm"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in wasm island page:\n%s", expected, html)
		}
	}
	wasm := readBytes(t, wasmPath)
	if !bytes.Equal(wasm, []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}) {
		t.Fatalf("expected minimal valid wasm module, got %#v", wasm)
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	for _, expected := range []string{
		`"assets/gowdk/islands/Counter.wasm": "assets/gowdk/islands/Counter.wasm"`,
		`"assets/gowdk/islands/Counter.wasm.js": "assets/gowdk/islands/Counter.wasm.js"`,
	} {
		if !strings.Contains(assetManifestPayload, expected) {
			t.Fatalf("expected %q in asset manifest:\n%s", expected, assetManifestPayload)
		}
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

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func hasAssetArtifact(artifacts []AssetArtifact, path string) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

func counterComponent() manifest.Component {
	return manifest.Component{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Count}</button>`,
		},
	}
}

func taggedCounterComponent() manifest.Component {
	return manifest.Component{
		Name:    "TaggedCounter",
		Source:  "components/tagged-counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TaggedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTaggedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<span>{Count}</span>`,
		},
	}
}

func textComponent() manifest.Component {
	return manifest.Component{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Query} />`,
		},
	}
}

func nestedComponent() manifest.Component {
	return manifest.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Open}>{Count}</section>`,
		},
	}
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
