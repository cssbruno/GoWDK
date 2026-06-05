package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

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
