package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildEmitsScopedJSOnlyForPagesThatUseIt(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "pages", "dashboard.js"), "import { chart } from './chart-data.js'; chart();\n")
	writeFile(t, filepath.Join(root, "pages", "metrics.ts"), "const label: string = 'ready'; document.documentElement.dataset.metrics = label;\n")
	writeFile(t, filepath.Join(root, "components", "chart.js"), "document.documentElement.dataset.chart = 'ready';\n")
	writeFile(t, filepath.Join(root, "components", "panel.ts"), "type Mode = 'ready'; const mode: Mode = 'ready'; document.body.dataset.panel = mode;\n")

	app := manifest.Manifest{
		Pages: []manifest.Page{
			{
				Source:  "pages/dashboard.page.gwdk",
				Package: "pages",
				ID:      "dashboard",
				Route:   "/dashboard",
				JS:      []string{"./dashboard.js", "./metrics.ts"},
				InlineJS: []manifest.InlineScript{{
					Name: "inline-gowdk.js",
					Body: "document.documentElement.dataset.inlineDashboard = 'ready';",
				}},
				Blocks: manifest.Blocks{
					View:     true,
					ViewBody: `<main><charts.SignupsChart /></main>`,
				},
				Uses: []manifest.Use{{Alias: "charts", Package: "components"}},
			},
			{
				Source:  "pages/about.page.gwdk",
				Package: "pages",
				ID:      "about",
				Route:   "/about",
				Blocks: manifest.Blocks{
					View:     true,
					ViewBody: `<main>About</main>`,
				},
			},
		},
		Components: []manifest.Component{{
			Source:  "components/signups-chart.cmp.gwdk",
			Package: "components",
			Name:    "SignupsChart",
			JS:      []string{"./chart.js", "./panel.ts"},
			InlineJS: []manifest.InlineScript{{
				Name: "inline-gowdk.js",
				Body: "document.documentElement.dataset.inlineChart = 'ready';",
			}},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<section data-signups-chart></section>`,
			},
		}},
	}
	outputDir := filepath.Join(root, "dist")

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	dashboardHTML := readFile(t, filepath.Join(outputDir, "dashboard", "index.html"))
	for _, expected := range []string{
		`<script type="module" src="/assets/gowdk/pages/dashboard/dashboard.js" defer></script>`,
		`<script type="module" src="/assets/gowdk/pages/dashboard/metrics.js" defer></script>`,
		`<script type="module" src="/assets/gowdk/pages/dashboard/inline-gowdk.js" defer></script>`,
		`<script type="module" src="/assets/gowdk/components/components/SignupsChart/chart.js" defer></script>`,
		`<script type="module" src="/assets/gowdk/components/components/SignupsChart/panel.js" defer></script>`,
		`<script type="module" src="/assets/gowdk/components/components/SignupsChart/inline-gowdk.js" defer></script>`,
	} {
		if !strings.Contains(dashboardHTML, expected) {
			t.Fatalf("expected dashboard HTML to contain %q:\n%s", expected, dashboardHTML)
		}
	}

	aboutHTML := readFile(t, filepath.Join(outputDir, "about", "index.html"))
	for _, unexpected := range []string{"dashboard.js", "metrics.js", "inline-gowdk.js", "SignupsChart/chart.js", "SignupsChart/panel.js"} {
		if strings.Contains(aboutHTML, unexpected) {
			t.Fatalf("did not expect %q in about HTML:\n%s", unexpected, aboutHTML)
		}
	}

	expectedAssets := map[string]string{
		"assets/gowdk/pages/dashboard/dashboard.js":                       "chart();",
		"assets/gowdk/pages/dashboard/metrics.js":                         "dataset.metrics = label",
		"assets/gowdk/pages/dashboard/inline-gowdk.js":                    "inlineDashboard",
		"assets/gowdk/components/components/SignupsChart/chart.js":        "dataset.chart",
		"assets/gowdk/components/components/SignupsChart/panel.js":        "dataset.panel = mode",
		"assets/gowdk/components/components/SignupsChart/inline-gowdk.js": "inlineChart",
	}
	for logicalPath, expectedContent := range expectedAssets {
		artifact := assetArtifactByLogicalPath(t, result.AssetArtifacts, logicalPath)
		if !strings.Contains(readFile(t, artifact.Path), expectedContent) {
			t.Fatalf("expected copied JS asset %s to contain %q", logicalPath, expectedContent)
		}
		if artifact.CachePolicy != noCacheAssetCachePolicy {
			t.Fatalf("expected no-cache policy for unhashed scoped script %s, got %q", logicalPath, artifact.CachePolicy)
		}
	}
}

func TestBuildRejectsInvalidScopedJSPath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	app := manifest.Manifest{Pages: []manifest.Page{{
		Source: "pages/home.page.gwdk",
		ID:     "home",
		Route:  "/",
		JS:     []string{"/absolute.js"},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}}}

	_, err := Build(gowdk.Config{}, app, filepath.Join(root, "dist"))
	if err == nil {
		t.Fatal("expected invalid scoped JS path error")
	}
	if !strings.Contains(err.Error(), `page home js "/absolute.js": path must be relative`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
