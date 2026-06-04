package staticgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/manifest"
	runtimeasset "github.com/gowdk/gowdk/runtime/asset"
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
				ViewBody: `<main>Home</main>`,
			},
		}},
		Components: []manifest.Component{{
			Source: "components/hero.cmp.gwdk",
			Name:   "Hero",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section>Hero</section>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{Addons: []gowdk.Addon{processor}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if processor.calls != 1 {
		t.Fatalf("expected one css processor call, got %d", processor.calls)
	}
	if len(processor.sources) != 2 || processor.sources[0].Kind != "page" || processor.sources[1].Kind != "component" {
		t.Fatalf("unexpected css sources: %#v", processor.sources)
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
			body:      `=> { slug: "../secret" }`,
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
			if entries, err := os.ReadDir(outputDir); err != nil {
				t.Fatal(err)
			} else if len(entries) != 0 {
				t.Fatalf("expected no partial output, got %#v", entries)
			}
		})
	}
}

func TestBuildRejectsRequestTimeAndDynamicPagesBeforeWriting(t *testing.T) {
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

	_, err := Build(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err == nil {
		t.Fatal("expected build error")
	}
	message := err.Error()
	if !strings.Contains(message, "cannot emit @render ssr") {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
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
