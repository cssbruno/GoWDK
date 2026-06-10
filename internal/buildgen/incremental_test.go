package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildPreservesUnchangedArtifactModTimes(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
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
	initial := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		{
			Source:  homeSource,
			Package: "app",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Home before</main>`,
			},
		},
		{
			Source:  aboutSource,
			Package: "app",
			ID:      "about",
			Route:   "/about",
			Blocks: gwdkir.Blocks{
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
	changed := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		{
			Source:  homeSource,
			Package: "app",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Home after</main>`,
			},
		},
		{
			Source:  aboutSource,
			Package: "app",
			ID:      "about",
			Route:   "/about",
			Blocks: gwdkir.Blocks{
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

func TestBuildIncrementalFromIRRendersChangedPageSources(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "home.page.gwdk")
	initial := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  source,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home before</main>`,
		},
	}}}
	if _, err := Build(gowdk.Config{}, initial, outputDir); err != nil {
		t.Fatal(err)
	}

	changed := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  source,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home after</main>`,
		},
	}}}
	result, err := BuildIncrementalFromIR(gowdk.Config{}, gwdkanalysis.BuildProgram(gowdk.Config{}, changed), outputDir, []string{source})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].PageID != "home" {
		t.Fatalf("unexpected artifacts: %#v", result.Artifacts)
	}
	if html := readFile(t, filepath.Join(outputDir, "index.html")); !strings.Contains(html, "Home after") {
		t.Fatalf("expected changed home output, got:\n%s", html)
	}
}

func TestBuildIncrementalRemovesStaleChangedPageRouteOutput(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "home.page.gwdk")
	initial := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  source,
		Package: "app",
		ID:      "home",
		Route:   "/old",
		Blocks: gwdkir.Blocks{
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

	changed := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source:  source,
		Package: "app",
		ID:      "home",
		Route:   "/new",
		Blocks: gwdkir.Blocks{
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
