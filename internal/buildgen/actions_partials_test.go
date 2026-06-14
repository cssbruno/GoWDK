package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildLowersGPostDirectiveForActionPage(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "signup",
		Route:  "/signup",
		Render: gowdk.Action,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<form g:post={Submit}><input name="email" /></form>`,
			Actions: []gwdkir.Action{{
				Name:     "Submit",
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
	if !strings.Contains(output, `<form method="post" action="/signup"><input name="email"></form>`) {
		t.Fatalf("expected lowered g:post form in output:\n%s", output)
	}
}

func TestBuildSynthesizesActionInputAttrsFromBindingFields(t *testing.T) {
	outputDir := t.TempDir()
	ir := gwdkir.Program{Version: gwdkir.Version, Pages: []gwdkir.Page{{
		ID:     "signup",
		Route:  "/signup",
		Render: gowdk.Action,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<form g:post={Submit}><input name="age" /><input name="score" /></form>`,
			Actions:  []gwdkir.Action{{Name: "Submit"}},
		},
	}}, Endpoints: []gwdkir.Endpoint{{
		Kind:   gwdkir.EndpointAction,
		Source: gwdkir.EndpointSourceGOWDK,
		PageID: "signup",
		Symbol: "Submit",
		Method: "POST",
		Path:   "/signup",
		Binding: gwdkir.Binding{InputFields: []source.BackendInputField{
			{FieldName: "Age", FormName: "age", Type: "uint8"},
			{FieldName: "Score", FormName: "score", Type: "int16"},
		}},
	}}}

	_, err := BuildFromIR(gowdk.Config{}, ir, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "signup", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, want := range []string{
		`<input name="age" type="number" inputmode="numeric" min="0" max="255">`,
		`<input name="score" type="number" inputmode="numeric" min="-32768" max="32767">`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestBuildProductionRequiresBoundBackendHandlers(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "signup",
		Package: "app",
		Source:  filepath.Join(t.TempDir(), "signup.page.gwdk"),
		Route:   "/signup",
		Render:  gowdk.Action,
		Guards:  []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<form g:post={Submit}><input name="email" /></form>`,
			Actions:  []gwdkir.Action{{Name: "Submit"}},
		},
	}}}

	_, err := Build(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, app, outputDir)
	if err == nil {
		t.Fatal("expected production build to reject missing backend handler")
	}
	if !strings.Contains(err.Error(), "production build requires a bound action handler Submit") {
		t.Fatalf("unexpected production backend binding error: %v", err)
	}
}

func TestBuildProductionAllowsExplicitMissingBackendStubs(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "signup",
		Package: "app",
		Source:  filepath.Join(t.TempDir(), "signup.page.gwdk"),
		Route:   "/signup",
		Render:  gowdk.Action,
		Guards:  []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<form g:post={Submit}><input name="email" /></form>`,
			Actions:  []gwdkir.Action{{Name: "Submit"}},
		},
	}}}

	_, err := Build(gowdk.Config{Build: gowdk.BuildConfig{
		Mode:                gowdk.Production,
		AllowMissingBackend: true,
	}}, app, outputDir)
	if err != nil {
		t.Fatalf("expected explicit missing backend stubs to build, got %v", err)
	}
}

func TestBuildAllowsGPostWithLocalValueBinding(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<form g:post={Submit}><input name="query" g:bind:value={Query} /></form>`
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:     "search",
			Route:  "/search",
			Render: gowdk.Action,
			Guards: []string{"public"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
				Actions: []gwdkir.Action{{
					Name:     "Submit",
					Redirect: "/search",
				}},
			},
		}},
		Components: []gwdkir.Component{component},
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
	if strings.Contains(html, `data-gowdk-on-Submit`) || strings.Contains(html, `data-gowdk-event-Submit`) {
		t.Fatalf("did not expect local value binding to add Submit event interception:\n%s", html)
	}
}

func TestBuildEmitsPartialRuntimeForFragmentForms(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "patients",
		Route:  "/patients",
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View: true,
			ViewBody: `<main>
  <form g:post={Refresh} g:target="#patients" g:swap="innerHTML"><input name="query" /></form>
  <section id="patients">Initial</section>
</main>`,
			Actions: []gwdkir.Action{{
				Name: "Refresh",
				Fragments: []gwdkir.Fragment{{
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
	if result.AssetArtifacts[0].CachePolicy != noCacheAssetCachePolicy {
		t.Fatalf("expected no-cache policy for unhashed runtime asset, got %q", result.AssetArtifacts[0].CachePolicy)
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
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "signup",
		Route:  "/signup",
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<form g:post={Missing}></form>`,
			Actions: []gwdkir.Action{{
				Name:     "Submit",
				Redirect: "/signup?ok=1",
			}},
		},
	}}}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unknown g:post action error")
	}
	if !strings.Contains(err.Error(), `signup: unknown action "Missing" for g:post`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected no partial output, got %#v", entries)
	}
}
