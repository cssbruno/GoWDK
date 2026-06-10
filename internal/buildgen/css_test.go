package buildgen

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/manifest"
	runtimeapp "github.com/cssbruno/gowdk/runtime/app"
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

func TestMinifyCSSPreservesCalcOperatorSpacing(t *testing.T) {
	input := []byte(`
.a {
  width: calc(100% + 1rem);
  height: calc(100% - 2px);
  margin: min(1rem + 2px, 3rem);
}
.b + .c {
  color: red;
}
`)
	got := string(minifyCSS(input))
	expected := `.a{width:calc(100% + 1rem);height:calc(100% - 2px);margin:min(1rem + 2px,3rem);}.b+.c{color:red;}`
	if got != expected {
		t.Fatalf("unexpected minified css:\nwant %q\n got %q", expected, got)
	}
}

func TestMinifyCSSPreservesMediaQueryParenSpacing(t *testing.T) {
	input := []byte(`
@media screen and (min-width: 600px) {
  .a {
    color: red;
  }
}
@supports (display: grid) and (gap: 1rem) {
  .b {
    display: grid;
  }
}
`)
	got := string(minifyCSS(input))
	expected := `@media screen and (min-width:600px){.a{color:red;}}@supports (display:grid) and (gap:1rem){.b{display:grid;}}`
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

func TestBuildEmitsPageStyleBlock(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "styled",
		Route: "/styled",
		CSS:   []string{"none"},
		Blocks: manifest.Blocks{
			View:      true,
			ViewBody:  `<main class="hero">Styled</main>`,
			Style:     true,
			StyleBody: `.hero { color: red; }`,
		},
	}}}

	result, err := Build(gowdk.Config{CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	artifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, "assets/gowdk/styled.css")
	html := readFile(t, filepath.Join(outputDir, "styled", "index.html"))
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, artifact.Path))
	if !strings.Contains(html, `<link rel="stylesheet" href="/`+emittedRel+`">`) {
		t.Fatalf("expected inline style stylesheet link:\n%s", html)
	}
	if strings.Contains(html, "style {") {
		t.Fatalf("did not expect source style block in html:\n%s", html)
	}
	css := readFile(t, artifact.Path)
	if css != ".hero{color:red;}" {
		t.Fatalf("unexpected inline page css: %q", css)
	}
}

func TestBuildRecordsPageCachePolicyInAssetManifest(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:         "home",
		Route:      "/",
		Cache:      "public, max-age=120",
		Revalidate: "30",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(payload, &assets); err != nil {
		t.Fatal(err)
	}
	if cache := assets.CachePolicy("index.html"); cache != "public, max-age=120, stale-while-revalidate=30" {
		t.Fatalf("expected page cache policy in asset manifest, got %q in %s", cache, payload)
	}
	if assets.Resolve("index.html") != "" {
		t.Fatalf("did not expect HTML route to become an asset file entry: %s", payload)
	}
}

func TestBuildDefaultCSSDiscoveryExcludesGeneratedOutputDirs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "styles", "global.css"), "body { color: black; }\n")
	for _, generated := range []string{
		filepath.Join(root, ".gowdk", "app", "gowdkapp", "app", "login.f1503fdf7590.css"),
		filepath.Join(root, ".gowdk", "frontend", "gowdkapp", "app", "login.f1503fdf7590.css"),
		filepath.Join(root, "dist", "site", "login.f1503fdf7590.css"),
	} {
		if err := os.MkdirAll(filepath.Dir(generated), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, generated, ".generated { color: red; }\n")
	}

	outputDir := filepath.Join(root, "build")
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	homeArtifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, "assets/gowdk/home.css")
	homeCSS := readFile(t, homeArtifact.Path)
	if !strings.Contains(homeCSS, "body{color:black;}") {
		t.Fatalf("expected source css in home css:\n%s", homeCSS)
	}
	if strings.Contains(homeCSS, ".generated") {
		t.Fatalf("did not expect generated css inputs in home css:\n%s", homeCSS)
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

func TestBuildEmitsScopedComponentCSSWithManifestAndCacheHeaders(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("components", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "components", "hero.css"), `
.hero {
  animation: fade 1s ease;
  color: red;
}
.hero h1, .hero > p {
  color: blue;
}
@keyframes fade {
  from { opacity: 0; }
  to { opacity: 1; }
}
@media (min-width: 40rem) {
  .hero { padding: 1rem; }
}
`)
	outputDir := filepath.Join(root, "dist")
	component := manifest.Component{
		Package: "components",
		Source:  "components/hero.cmp.gwdk",
		Name:    "Hero",
		CSS:     []string{"./hero.css"},
		Props:   []manifest.Prop{{Name: "title", Type: "string"}},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section .hero><h1>{title}</h1></section>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			Package: "components",
			ID:      "home",
			Route:   "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero title="GOWDK" /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, "./hero.css")
	scopeID := cssscope.ScopeID(hashKey)
	logicalPath := componentCSSLogicalPath(irComponent(component), scopeID)
	artifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, logicalPath)
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, artifact.Path))
	if !strings.Contains(emittedRel, "/"+scopeID+".") || !strings.HasSuffix(emittedRel, ".css") {
		t.Fatalf("expected scoped component css to use hashed filename, got %q", emittedRel)
	}

	html := readFile(t, filepath.Join(outputDir, "index.html"))
	if !strings.Contains(html, `<link rel="stylesheet" href="/`+emittedRel+`">`) {
		t.Fatalf("expected scoped css link in html:\n%s", html)
	}
	if !strings.Contains(html, `class="hero" data-gowdk-scope="`+scopeID+`"`) ||
		!strings.Contains(html, `<h1 data-gowdk-scope="`+scopeID+`">GOWDK</h1>`) {
		t.Fatalf("expected component scope marker in html:\n%s", html)
	}

	css := readFile(t, artifact.Path)
	scopeSelector := componentCSSScopeSelector(scopeID)
	for _, expected := range []string{
		`.hero` + scopeSelector + `{animation:fade-` + scopeID + ` 1s ease;color:red;}`,
		`.hero h1` + scopeSelector + `,.hero>p` + scopeSelector + `{color:blue;}`,
		`@keyframes fade-` + scopeID + `{from{opacity:0;}to{opacity:1;}}`,
		`@media (min-width:40rem){.hero` + scopeSelector + `{padding:1rem;}}`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected %q in scoped css:\n%s", expected, css)
		}
	}

	manifestPayload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(manifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Resolve(logicalPath) != emittedRel {
		t.Fatalf("expected manifest mapping for component css, got %s", manifestPayload)
	}
	if hash := assets.Hash(logicalPath); !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("expected component css hash, got %q in %s", hash, manifestPayload)
	}
	if policy := assets.CachePolicy(logicalPath); policy != immutableAssetCachePolicy {
		t.Fatalf("expected immutable component css cache policy, got %q", policy)
	}

	handler := runtimeapp.Handler{
		Root: fstest.MapFS{
			emittedRel: {Data: []byte(css)},
		},
		Assets: assets,
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/"+emittedRel, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected generated binary asset status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != immutableAssetCachePolicy {
		t.Fatalf("expected generated binary cache header, got %q", cache)
	}
}

func TestRewriteCSSKeyframesRewritesAnimationNamesOnly(t *testing.T) {
	got := rewriteCSSKeyframes(`@-webkit-keyframes fade{to{opacity:1}}.a{animation-name:fade,fade-out;animation:fade 1s ease;}`, "scope")
	for _, expected := range []string{
		`@-webkit-keyframes fade-scope{`,
		`animation-name:fade-scope,fade-out`,
		`animation:fade-scope 1s ease`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in rewritten css:\n%s", expected, got)
		}
	}
	if strings.Contains(got, `fade-out-scope`) {
		t.Fatalf("must not rewrite partial animation identifiers:\n%s", got)
	}
}

func TestBuildEmitsComponentFileAssetsWithManifestAndCacheHeaders(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("components", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "components", "hero.png"), "fake image\n")
	outputDir := filepath.Join(root, "dist")
	component := manifest.Component{
		Package: "components",
		Source:  "components/hero.cmp.gwdk",
		Name:    "Hero",
		Assets:  []string{"./hero.png"},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section>Hero</section>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			Package: "components",
			ID:      "home",
			Route:   "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Hero /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	logicalPath := "assets/gowdk/components/components/Hero/hero.png"
	artifact := assetArtifactByLogicalPath(t, result.AssetArtifacts, logicalPath)
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, artifact.Path))
	if emittedRel == logicalPath || !strings.HasPrefix(emittedRel, "assets/gowdk/components/components/Hero/hero.") || !strings.HasSuffix(emittedRel, ".png") {
		t.Fatalf("expected content-hashed component asset filename, got %q", emittedRel)
	}
	if got := readFile(t, artifact.Path); got != "fake image\n" {
		t.Fatalf("unexpected emitted asset contents: %q", got)
	}

	manifestPayload := readBytes(t, filepath.Join(outputDir, assetManifestFile))
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(manifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Resolve(logicalPath) != emittedRel {
		t.Fatalf("expected component asset manifest mapping, got %s", manifestPayload)
	}
	if hash := assets.Hash(logicalPath); !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("expected component asset hash, got %q in %s", hash, manifestPayload)
	}
	if policy := assets.CachePolicy(logicalPath); policy != immutableAssetCachePolicy {
		t.Fatalf("expected immutable component asset cache policy, got %q", policy)
	}

	handler := runtimeapp.Handler{
		Root: fstest.MapFS{
			emittedRel: {Data: readBytes(t, artifact.Path)},
		},
		Assets: assets,
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/"+emittedRel, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected generated binary asset status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != immutableAssetCachePolicy {
		t.Fatalf("expected generated binary cache header, got %q", cache)
	}
}

func TestBuildEmitsScopedComponentStyleBlock(t *testing.T) {
	outputDir := t.TempDir()
	component := manifest.Component{
		Package: "components",
		Source:  "components/card.cmp.gwdk",
		Name:    "Card",
		Blocks: manifest.Blocks{
			View:      true,
			ViewBody:  `<section class="card">Card</section>`,
			Style:     true,
			StyleBody: `.card { color: red; }`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			Package: "components",
			ID:      "home",
			Route:   "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Card /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, inlineStyleAssetPath)
	scopeID := cssscope.ScopeID(hashKey)
	artifact := cssArtifactByLogicalPath(t, result.CSSArtifacts, componentCSSLogicalPath(irComponent(component), scopeID))
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, artifact.Path))
	if !strings.Contains(html, `<link rel="stylesheet" href="/`+emittedRel+`">`) {
		t.Fatalf("expected component inline style link:\n%s", html)
	}
	if !strings.Contains(html, `class="card" data-gowdk-scope="`+scopeID+`"`) {
		t.Fatalf("expected component inline style scope marker:\n%s", html)
	}
	css := readFile(t, artifact.Path)
	expected := `.card` + componentCSSScopeSelector(scopeID) + `{color:red;}`
	if css != expected {
		t.Fatalf("unexpected scoped inline component css:\nwant %q\n got %q", expected, css)
	}
}

func TestBuildLinksLayoutStyleBlockToUsingPages(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{
				ID:      "home",
				Route:   "/",
				Layouts: []string{"root"},
				Blocks:  manifest.Blocks{View: true, ViewBody: `<main>Home</main>`},
			},
			{
				ID:     "plain",
				Route:  "/plain",
				Blocks: manifest.Blocks{View: true, ViewBody: `<main>Plain</main>`},
			},
		},
		Layouts: []manifest.Layout{{
			ID: "root",
			Blocks: manifest.Blocks{
				View:      true,
				ViewBody:  `<section class="shell"><slot /></section>`,
				Style:     true,
				StyleBody: `.shell { padding: 1rem; }`,
			},
		}},
	}

	result, err := Build(gowdk.Config{CSS: gowdk.CSSConfig{Include: []string{DisableCSSDiscovery}}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CSSArtifacts) != 1 {
		t.Fatalf("expected one layout css artifact, got %#v", result.CSSArtifacts)
	}
	emittedRel := filepath.ToSlash(mustRelativePath(t, outputDir, result.CSSArtifacts[0].Path))
	home := readFile(t, filepath.Join(outputDir, "index.html"))
	if !strings.Contains(home, `<link rel="stylesheet" href="/`+emittedRel+`">`) {
		t.Fatalf("expected layout style link on using page:\n%s", home)
	}
	plain := readFile(t, filepath.Join(outputDir, "plain", "index.html"))
	if strings.Contains(plain, emittedRel) {
		t.Fatalf("did not expect layout style link on non-using page:\n%s", plain)
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
