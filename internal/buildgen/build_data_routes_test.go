package buildgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

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

func TestBuildMergesMultipleLiteralBuildDataDeclarations(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:    "home",
		Route: "/",
		Blocks: manifest.Blocks{
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

func TestBuildExpandsDynamicSPAPaths(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "blog.post",
			Route: "/blog/{slug}",
			Paths: true,
			Blocks: manifest.Blocks{
				PathsBody: `=> { slug: "hello-gowdk" }
=> { slug: "compile-first" }`,
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
