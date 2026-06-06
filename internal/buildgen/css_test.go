package buildgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

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

func TestMinifyCSSPreservesRequiredValueSpacing(t *testing.T) {
	input := []byte(`
/* remove */
.hero {
  background: url("/hero image.png") center / cover;
  font-family: "Open Sans", sans-serif;
}
`)
	got := string(minifyCSS(input))
	expected := `.hero{background:url("/hero image.png") center / cover;font-family:"Open Sans",sans-serif;}`
	if got != expected {
		t.Fatalf("unexpected minified css:\nwant %q\n got %q", expected, got)
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
	homeArtifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, "assets/gowdk/home.css")
	dashboardArtifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, "assets/gowdk/dashboard.css")

	homeHTML := readFile(t, filepath.Join(outputDir, "index.html"))
	homeHref := "/" + strings.TrimPrefix(filepath.ToSlash(mustRelativePath(t, outputDir, homeArtifact.Path)), "/")
	if !strings.Contains(homeHTML, `<link rel="stylesheet" href="`+homeHref+`">`) {
		t.Fatalf("expected home page css link:\n%s", homeHTML)
	}
	if !strings.Contains(homeHref, "/assets/gowdk/home.") || !strings.HasSuffix(homeHref, ".css") {
		t.Fatalf("expected hashed home css href, got %q", homeHref)
	}
	homeCSS := readFile(t, homeArtifact.Path)
	for _, expected := range []string{"body{color:black;}", ".home{display:grid;}"} {
		if !strings.Contains(homeCSS, expected) {
			t.Fatalf("expected %q in home css:\n%s", expected, homeCSS)
		}
	}
	if strings.Contains(homeCSS, "input{font:inherit;}") {
		t.Fatalf("did not expect forms css in default home css:\n%s", homeCSS)
	}

	dashboardHTML := readFile(t, filepath.Join(outputDir, "dashboard", "index.html"))
	dashboardHref := "/" + strings.TrimPrefix(filepath.ToSlash(mustRelativePath(t, outputDir, dashboardArtifact.Path)), "/")
	if !strings.Contains(dashboardHTML, `<link rel="stylesheet" href="`+dashboardHref+`">`) {
		t.Fatalf("expected dashboard page css link:\n%s", dashboardHTML)
	}
	dashboardCSS := readFile(t, dashboardArtifact.Path)
	for _, expected := range []string{"*{box-sizing:border-box;}", ":root{--brand:blue;}", "input{font:inherit;}"} {
		if !strings.Contains(dashboardCSS, expected) {
			t.Fatalf("expected %q in dashboard css:\n%s", expected, dashboardCSS)
		}
	}
	if strings.Contains(dashboardCSS, "body{color:black;}") {
		t.Fatalf("did not expect global css in exact dashboard selection:\n%s", dashboardCSS)
	}

	embedHTML := readFile(t, filepath.Join(outputDir, "embed", "index.html"))
	if strings.Contains(embedHTML, "/assets/gowdk/embed.css") {
		t.Fatalf("did not expect embed page css link:\n%s", embedHTML)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "assets", "gowdk", "embed.css")); !os.IsNotExist(err) {
		t.Fatalf("expected no embed css file, stat err: %v", err)
	}
	assetManifestPayload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(assetManifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Resolve("assets/gowdk/home.css") != strings.TrimPrefix(homeHref, "/") {
		t.Fatalf("expected logical home css to resolve to hashed path, manifest: %s", assetManifestPayload)
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
	cssPath := result.CSSArtifacts[0].Path
	if result.CSSArtifacts[0].LogicalPath != "assets/app.css" {
		t.Fatalf("expected logical css path to stay stable, got %#v", result.CSSArtifacts[0])
	}
	if !strings.Contains(filepath.ToSlash(cssPath), "/assets/app.") || !strings.HasSuffix(cssPath, ".css") {
		t.Fatalf("expected hashed css path, got %s", cssPath)
	}
	payload, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "body{color:black}" {
		t.Fatalf("unexpected css payload: %q", payload)
	}
	html, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, cssPath))
	if !strings.Contains(string(html), `<link rel="stylesheet" href="/`+emittedRel+`">`) {
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
	if assets.Version != 1 || assets.Resolve("assets/app.css") != emittedRel {
		t.Fatalf("unexpected asset manifest: %s", assetManifestPayload)
	}
	if hash := assets.Hash("assets/app.css"); !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("expected asset hash, got %q in %s", hash, assetManifestPayload)
	}
	if policy := assets.CachePolicy("assets/app.css"); policy != immutableAssetCachePolicy {
		t.Fatalf("expected immutable asset cache policy, got %q", policy)
	}
}

func TestBuildAppliesPageAwareCSSProcessorStylesheets(t *testing.T) {
	outputDir := t.TempDir()
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
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main>Dashboard</main>`,
			},
		},
	}}

	_, err := Build(gowdk.Config{
		CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}},
		Addons: []gowdk.Addon{pageAwareCSSProcessor{
			pageStylesheets: map[string][]gowdk.Stylesheet{
				"dashboard": {{Href: "/assets/dashboard.css"}},
			},
		}},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(home), "/assets/dashboard.css") {
		t.Fatalf("did not expect dashboard stylesheet in home output:\n%s", home)
	}
	dashboard, err := os.ReadFile(filepath.Join(outputDir, "dashboard", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dashboard), `<link rel="stylesheet" href="/assets/dashboard.css">`) {
		t.Fatalf("expected dashboard stylesheet in dashboard output:\n%s", dashboard)
	}
}

func TestBuildRejectsUnknownPageAwareCSSProcessorSelection(t *testing.T) {
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
		CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}},
		Addons: []gowdk.Addon{pageAwareCSSProcessor{
			pageStylesheets: map[string][]gowdk.Stylesheet{
				"missing": {{Href: "/assets/missing.css"}},
			},
		}},
	}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown page selection error")
	}
	if !strings.Contains(err.Error(), `css processor page-aware-css selected unknown page "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
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

type pageAwareCSSProcessor struct {
	pageStylesheets map[string][]gowdk.Stylesheet
}

func (processor pageAwareCSSProcessor) Name() string {
	return "page-aware-css"
}

func (processor pageAwareCSSProcessor) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (processor pageAwareCSSProcessor) ProcessCSS(gowdk.CSSContext) (gowdk.CSSResult, error) {
	return gowdk.CSSResult{PageStylesheets: processor.pageStylesheets}, nil
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
