package buildgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestSamePackageImportPathSurfacesGoListError(t *testing.T) {
	// A temp dir outside any Go module makes `go list` fail with a clear reason
	// (no go.mod), which must be surfaced rather than collapsed into a generic
	// "requires a buildable Go package" message.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "page.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := samePackageImportPath(filepath.Join(root, "home.page.gwdk"))
	if err == nil {
		t.Fatal("expected a same-package build data error outside a Go module")
	}
	if !strings.Contains(err.Error(), "buildable Go package") {
		t.Fatalf("expected the user-facing message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("expected the underlying go list error (go.mod not found) to be surfaced, got: %v", err)
	}
}

func TestBuildRendersLiteralBuildData(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "home",
			Route: "/",
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Portable Go web compiler", slug: "home" }`,
				View:      true,
				ViewBody:  `<main data-page="{slug}"><Hero title="{title}" /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Name: "Hero",
			Props: []gwdkir.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: gwdkir.Blocks{
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

func TestBuildEmitsLocalizedRoutesAndPassesLocaleToBuildParams(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join("..", "..", "examples", "go-interop", "localized-about.page.gwdk")
	config := gowdk.Config{I18N: gowdk.I18NConfig{
		Locales: []gowdk.LocaleConfig{
			{Code: "en"},
			{Code: "pt", PathPrefix: "/pt"},
		},
		DefaultLocale: "en",
	}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "about",
		Package: "pages",
		Source:  source,
		Route:   "/about",
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> AboutForBuild()`,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `type PageCopy struct {
	Title string ` + "`json:\"title\"`" + `
}

func AboutForBuild(params gowdkbuildparams.BuildParams) PageCopy {
	return PageCopy{Title: "Locale " + params.LocaleCode()}
}`,
			}},
			View:     true,
			ViewBody: `<main><h1>{title}</h1></main>`,
		},
	}}}

	result, err := Build(config, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	en := readFile(t, filepath.Join(outputDir, "en", "about", "index.html"))
	if !strings.Contains(en, `<html lang="en">`) || !strings.Contains(en, `<h1>Locale en</h1>`) {
		t.Fatalf("expected localized English output:\n%s", en)
	}
	pt := readFile(t, filepath.Join(outputDir, "pt", "about", "index.html"))
	if !strings.Contains(pt, `<html lang="pt">`) || !strings.Contains(pt, `<h1>Locale pt</h1>`) {
		t.Fatalf("expected localized Portuguese output:\n%s", pt)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("expected two localized artifacts, got %#v", result.Artifacts)
	}
	manifest := readRouteManifest(t, outputDir)
	if len(manifest.Routes) != 2 || manifest.Routes[0].Locale != "en" || manifest.Routes[1].Locale != "pt" {
		t.Fatalf("expected localized route manifest entries, got %#v", manifest.Routes)
	}
}

func TestBuildUsesTypedPathAndBuildRecords(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "post",
			Route: "/blog/{slug}",
			Blocks: gwdkir.Blocks{
				Paths: true,
				PathsRecords: []gwdkir.LiteralRecord{{
					Fields: map[string]string{"slug": "typed"},
				}},
				Build: true,
				BuildRecords: []gwdkir.LiteralRecord{{
					Fields: map[string]string{
						"title": "Typed",
						"copy":  "Typed route",
					},
					Expressions: map[string]string{
						"title": `"Typed"`,
						"copy":  `title + " route"`,
					},
					FieldOrder: []string{"title", "copy"},
				}},
				View:     true,
				ViewBody: `<main data-slug="{slug}"><h1>{title}</h1><p>{copy}</p></main>`,
			},
		}},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "blog", "typed", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<main data-slug="typed"><h1>Typed</h1><p>Typed route</p></main>`) {
		t.Fatalf("expected typed path/build records in output:\n%s", output)
	}
}

func TestBuildRejectsNonStringTypedPathRecordValuesBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths: true,
			PathsRecords: []gwdkir.LiteralRecord{{
				Fields:      map[string]string{"slug": `field("title")`},
				Expressions: map[string]string{"slug": `field("title")`},
				FieldOrder:  []string{"slug"},
			}},
			View:     true,
			ViewBody: `<main>Post</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(err.Error(), `path param slug: value must be a string literal`) {
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
			name: "duplicate field across declarations",
			body: `=> { title: "Home" }
=> { title: "Second" }`,
			wantError: `duplicate build field "title"`,
		},
		{
			name:      "duplicate field",
			body:      `=> { title: "Home", title: "Again" }`,
			wantError: `duplicate build field "title"`,
		},
		{
			name:      "invalid expression",
			body:      `=> { count: 10 / 0 }`,
			wantError: `division by zero`,
		},
		{
			name:      "unsupported operand",
			body:      `=> { enabled: true + false }`,
			wantError: `operator + requires numbers`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir()
			app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
				ID:    "home",
				Route: "/",
				Blocks: gwdkir.Blocks{
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

func TestBuildMergesMultipleLiteralBuildDataDeclarations(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
			Build: true,
			BuildBody: `=> { title: "Home" }
=> { tagline: "Second declaration" }`,
			View:     true,
			ViewBody: `<main><h1>{title}</h1><p>{tagline}</p></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `<main><h1>Home</h1><p>Second declaration</p></main>`) {
		t.Fatalf("expected merged build data output:\n%s", payload)
	}
}

func TestBuildRendersExpandedBuildDataScalarsAndReferences(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "hello" }`,
			Build:     true,
			BuildBody: `=> { title: "Hello", count: 2, live: true }
=> { headline: "{title} {slug}", copy: field("headline") }
=> { total: (count + 3) * 2, inverse: -total, label: "Post " + param("slug"), current: field("label") == "Post hello", visible: live && count < 3 }`,
			View:     true,
			ViewBody: `<main data-count="{count}" data-live="{live}" data-total="{total}" data-inverse="{inverse}" data-current="{current}" data-visible="{visible}"><h1>{headline}</h1><p>{copy}</p><a>{label}</a></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "blog", "hello", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `<main data-count="2" data-live="true" data-total="10" data-inverse="-10" data-current="true" data-visible="true"><h1>Hello hello</h1><p>Hello hello</p><a>Post hello</a></main>`) {
		t.Fatalf("expected expanded build data output:\n%s", payload)
	}
}

func TestBuildRejectsBuildDataRouteParamConflictBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.imported",
		Route: "/go-imported",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
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

func TestBuildUsesImportedGoBuildDataWithEmptyBuildParams(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.static.params",
		Route: "/go-static-params",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> interop.StaticCopyWithParamsForBuild()`,
			View:      true,
			ViewBody:  `<main><h1>{title}</h1><p>{tagline}</p></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "go-static-params", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<h1>Static Go params</h1>`,
		`<p>Static pages receive empty BuildParams.</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in static BuildParams output:\n%s", expected, output)
		}
	}
}

func TestBuildUsesNoArgImportedGoBuildDataOnDynamicRoute(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.imported.dynamic",
		Route: "/go-imported/{slug}",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "legacy" }`,
			Build:     true,
			BuildBody: `=> interop.FeaturedCopyForBuild()`,
			View:      true,
			ViewBody:  `<main data-slug="{slug}"><h1>{title}</h1><p>{tagline}</p></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "go-imported", "legacy", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`data-slug="legacy"`,
		`<h1>Imported Go data</h1>`,
		`<p>This page rendered data from a Go package imported directly in .gwdk.</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in dynamic no-arg build data output:\n%s", expected, output)
		}
	}
}

func TestBuildUsesImportedGoBuildDataWhenFunctionWritesStderr(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.imported",
		Route: "/go-imported",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> interop.FeaturedCopyWithStderrForBuild()`,
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
		`<h1>Logged Go data</h1>`,
		`<p>Build helper stderr does not corrupt JSON build data.</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in imported build data output:\n%s", expected, output)
		}
	}
}

func TestBuildUsesImportedGoBuildDataReturningError(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.imported",
		Route: "/go-imported",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> interop.FeaturedCopyWithErrorForBuild()`,
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
		`<h1>Checked Go data</h1>`,
		`<p>Build helpers can return a value and error.</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in imported build data output:\n%s", expected, output)
		}
	}
}

func TestBuildPassesRouteParamsToImportedGoBuildData(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.post",
		Route: "/go-post/{slug:int}",
		Imports: []gwdkir.Import{{
			Alias: "interop",
			Path:  "github.com/cssbruno/gowdk/examples/go-interop",
		}},
		Blocks: gwdkir.Blocks{
			Paths: true,
			PathsRecords: []gwdkir.LiteralRecord{
				{Fields: map[string]string{"slug": "123"}},
				{Fields: map[string]string{"slug": "456"}},
			},
			Build:     true,
			BuildBody: `=> interop.PostCopyForBuild()`,
			View:      true,
			ViewBody:  `<main data-slug="{slug}" data-canonical="{canonical}"><h1>{title}</h1></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"123", "456"} {
		payload, err := os.ReadFile(filepath.Join(outputDir, "go-post", slug, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf(`<main data-slug="%s" data-canonical="/go-post/%s"><h1>Post %s</h1></main>`, slug, slug, slug)
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected route params in imported build data output for %s:\n%s", slug, payload)
		}
	}
}

func TestBuildRejectsUnsupportedRouteParamGoBuildSignature(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join("..", "..", "examples", "go-interop", "bad-signature.page.gwdk")
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "go.bad",
		Source: source,
		Route:  "/bad/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "nope" }`,
			Build:     true,
			BuildBody: `=> BadForBuild()`,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `func BadForBuild(params int) map[string]string {
	return map[string]string{"title": "bad"}
}`,
			}},
			View:     true,
			ViewBody: `<main>{title}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unsupported Go build signature error")
	}
	message := err.Error()
	if !strings.Contains(message, "BadForBuild") ||
		(!strings.Contains(message, "not enough arguments in call") && !strings.Contains(message, "cannot use gowdkbuildparams.BuildParams")) {
		t.Fatalf("unexpected unsupported signature error: %v", err)
	}
}

func TestBuildUsesInlineGoBlockGoBuildData(t *testing.T) {
	outputDir := t.TempDir()
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "home.page.gwdk")
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "go.inline",
		Package: "pages",
		Source:  source,
		Route:   "/go-inline",
		Imports: []gwdkir.Import{{
			Alias: "strings",
			Path:  "strings",
		}},
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> HomePageForBuild()`,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `type PageCopy struct {
	Title string ` + "`json:\"title\"`" + `
	Slug string ` + "`json:\"slug\"`" + `
}

func HomePageForBuild() PageCopy {
	title := "GOWDK ships apps"
	return PageCopy{
		Title: title,
		Slug: strings.ToLower(strings.ReplaceAll(title, " ", "-")),
	}
}`,
			}},
			View:     true,
			ViewBody: `<main data-slug="{slug}"><h1>{title}</h1></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "go-inline", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<main data-slug="gowdk-ships-apps"><h1>GOWDK ships apps</h1></main>`) {
		t.Fatalf("expected inline go block build data output:\n%s", output)
	}
}

func TestBuildUsesDefaultGoBlockGoBuildData(t *testing.T) {
	outputDir := t.TempDir()
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "home.page.gwdk")
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "go.default",
		Package: "pages",
		Source:  source,
		Route:   "/go-default",
		Blocks: gwdkir.Blocks{
			Build:     true,
			BuildBody: `=> StaticPageForBuild()`,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `type PageCopy struct {
	Title string ` + "`json:\"title\"`" + `
	Slug string ` + "`json:\"slug\"`" + `
}

func StaticPageForBuild() PageCopy {
	return PageCopy{
		Title: "Static-first script",
		Slug: "static-first-script",
	}
}`,
			}},
			View:     true,
			ViewBody: `<main data-slug="{slug}"><h1>{title}</h1></main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "go-default", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, `<main data-slug="static-first-script"><h1>Static-first script</h1></main>`) {
		t.Fatalf("expected default go block build data output:\n%s", output)
	}
}

func TestBuildRejectsMissingGoBuildDataImport(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "go.interop",
		Route: "/go-interop",
		Blocks: gwdkir.Blocks{
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "hello-gowdk" }`,
			Build:     true,
			BuildBody: `=> { title: "Post {missing}" }`,
			View:      true,
			ViewBody:  `<main>{title}</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown interpolation reference error")
	}
	if !strings.Contains(err.Error(), `build field title: unknown build data field or route param "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInterpolateBuildValueReportsAccurateUnknownReference(t *testing.T) {
	data := map[string]string{"title": "Hi"}
	params := map[string]string{"slug": "x"}
	cases := []struct {
		value string
		want  string
	}{
		{`{field("missing")}`, `unknown build data field "missing"`},
		{`{missing}`, `unknown build data field or route param "missing"`},
		{`{param("missing")}`, `unknown route param "missing"`},
	}
	for _, tc := range cases {
		_, err := interpolateBuildValue(tc.value, params, data)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("interpolateBuildValue(%q): want error containing %q, got %v", tc.value, tc.want, err)
		}
	}
}

func TestBuildRendersExplicitRouteParamReferences(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "hello-gowdk" }`,
			View:      true,
			ViewBody:  `<main data-slug="{param(\"slug\")}"><h1>{param("slug")}</h1><a href="/blog/{param(\"slug\")}">Post</a></main>`,
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
	if !strings.Contains(string(payload), `<main data-slug="hello-gowdk"><h1>hello-gowdk</h1><a href="/blog/hello-gowdk">Post</a></main>`) {
		t.Fatalf("expected route param reference in output:\n%s", payload)
	}
}

func TestBuildRejectsUndeclaredRouteParamReferenceBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { slug: "alert(1)" }`,
			View:      true,
			ViewBody:  `<a href="{param(\"slug\")}">Post</a>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected dangerous route param attribute error")
	}
	if !strings.Contains(err.Error(), `is not allowed in "href" attributes`) {
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: gwdkir.Blocks{
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

func TestBuildExpandsDynamicSPAPaths(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:    "blog.post",
			Route: "/blog/{slug}",
			Blocks: gwdkir.Blocks{
				Paths: true,
				PathsBody: `=> { slug: "hello-gowdk" }
=> { slug: "compile-first" }`,
				View:     true,
				ViewBody: `<main data-slug="{slug}"><h1>{slug}</h1><PostTitle title="{slug}" /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Name: "PostTitle",
			Props: []gwdkir.Prop{
				{Name: "title", Type: "string"},
			},
			Blocks: gwdkir.Blocks{
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
	for _, slug := range []string{"hello-gowdk", "compile-first"} {
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
	if seen["/blog/compile-first"] != "blog/compile-first/index.html" {
		t.Fatalf("missing app route in manifest: %#v", seen)
	}
}

func TestBuildExpandsTypedDynamicSPAPathsAndInheritedActionRoutes(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "patients.show",
		Route:  "/patients/{id:int}",
		Render: gowdk.SPA,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			Paths:     true,
			PathsBody: `=> { id: "123" }`,
			View:      true,
			ViewBody:  `<main><p>{id}</p><form g:post={Save}><input name="name" /></form></main>`,
			Actions: []gwdkir.Action{{
				Name: "Save",
			}},
		},
	}}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(outputDir, "patients", "123", "index.html")
	if len(result.Artifacts) != 1 || result.Artifacts[0].Route != "/patients/123" || result.Artifacts[0].Path != wantPath {
		t.Fatalf("unexpected typed dynamic artifact: %#v", result.Artifacts)
	}
	html := readFile(t, wantPath)
	for _, expected := range []string{
		`<main><p>123</p><form method="post" action="/patients/123"><input name="name"></form></main>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in typed dynamic action page:\n%s", expected, html)
		}
	}

	routes := readRouteManifest(t, outputDir)
	if len(routes.Routes) != 1 || routes.Routes[0].Route != "/patients/123" || routes.Routes[0].Path != "patients/123/index.html" {
		t.Fatalf("unexpected typed dynamic route manifest: %#v", routes.Routes)
	}
}

func TestBuildLocalizesInheritedActionRoutes(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "contact",
		Route:  "/contact",
		Render: gowdk.SPA,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><form g:post={Submit}><input name="email" /></form></main>`,
			Actions: []gwdkir.Action{{
				Name: "Submit",
			}},
		},
	}}}

	result, err := Build(gowdk.Config{I18N: gowdk.I18NConfig{
		Locales: []gowdk.LocaleConfig{{Code: "en"}, {Code: "pt-BR", PathPrefix: "/br"}},
	}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("expected localized artifacts, got %#v", result.Artifacts)
	}
	for _, item := range []struct {
		path string
		want string
	}{
		{path: filepath.Join(outputDir, "en", "contact", "index.html"), want: `action="/en/contact"`},
		{path: filepath.Join(outputDir, "br", "contact", "index.html"), want: `action="/br/contact"`},
	} {
		html := readFile(t, item.path)
		if !strings.Contains(html, item.want) {
			t.Fatalf("expected localized inherited action %s in %s:\n%s", item.want, item.path, html)
		}
	}
}

func TestBuildRejectsUnknownDynamicInterpolationBeforeWriting(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "blog.post",
		Route: "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths:     true,
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
			wantError: `blog.post (src/pages/blog.post.page.gwdk): page route "/blog/hello-gowdk" duplicates page blog.post`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir()
			sourcePath := ""
			if tt.name == "duplicate output" {
				sourcePath = "src/pages/blog.post.page.gwdk"
			}
			app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
				ID:     "blog.post",
				Route:  "/blog/{slug}",
				Source: sourcePath,
				Blocks: gwdkir.Blocks{
					Paths:     true,
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
