package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/seo"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildDoesNotEmitSEOArtifactsWithoutAddon(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{seoHomePage()}}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.SitemapPath != "" || result.RobotsPath != "" {
		t.Fatalf("did not expect SEO paths without addon: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(outputDir, sitemapFile)); !os.IsNotExist(err) {
		t.Fatalf("sitemap must be disabled by default, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, robotsFile)); !os.IsNotExist(err) {
		t.Fatalf("robots must be disabled by default, stat err=%v", err)
	}
}

func TestBuildEmitsSEOArtifactsWhenAddonEnabled(t *testing.T) {
	outputDir := t.TempDir()
	config := gowdk.Config{Addons: []gowdk.Addon{seo.Addon(seo.Options{
		BaseURL:  "https://example.com/docs/",
		Disallow: []string{"/admin", "/drafts", "/admin"},
		ExtraURLs: []seo.URL{{
			Loc:        "/rss.xml",
			LastMod:    "2026-06-14",
			ChangeFreq: "daily",
			Priority:   "0.8",
		}},
	})}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		seoHomePage(),
		{
			ID:    "blog",
			Route: "/blog/{slug}",
			Blocks: gwdkir.Blocks{
				Paths:     true,
				PathsBody: `=> { slug: "hello-gowdk" }`,
				View:      true,
				ViewBody:  `<main>Blog</main>`,
			},
		},
	}}

	result, err := Build(config, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.SitemapPath != filepath.Join(outputDir, sitemapFile) || result.RobotsPath != filepath.Join(outputDir, robotsFile) {
		t.Fatalf("unexpected SEO paths: sitemap=%q robots=%q", result.SitemapPath, result.RobotsPath)
	}

	sitemap := readFile(t, result.SitemapPath)
	for _, expected := range []string{
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		`<loc>https://example.com/docs/</loc>`,
		`<loc>https://example.com/docs/blog/hello-gowdk</loc>`,
		`<loc>https://example.com/docs/rss.xml</loc>`,
		`<lastmod>2026-06-14</lastmod>`,
		`<changefreq>daily</changefreq>`,
		`<priority>0.8</priority>`,
	} {
		if !strings.Contains(sitemap, expected) {
			t.Fatalf("expected sitemap to contain %q:\n%s", expected, sitemap)
		}
	}

	robots := readFile(t, result.RobotsPath)
	for _, expected := range []string{
		"User-agent: *",
		"Disallow: /admin",
		"Disallow: /drafts",
		"Sitemap: https://example.com/docs/sitemap.xml",
	} {
		if !strings.Contains(robots, expected) {
			t.Fatalf("expected robots to contain %q:\n%s", expected, robots)
		}
	}

	requireBuildReportEvent(t, result.Report, "seo", "sitemap_written")
	requireBuildReportEvent(t, result.Report, "seo", "robots_written")
}

func TestBuildReportListsSEORouteExclusions(t *testing.T) {
	outputDir := t.TempDir()
	config := gowdk.Config{Addons: []gowdk.Addon{
		seo.Addon(seo.Options{BaseURL: "https://example.com"}),
		ssr.Addon(),
	}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		seoHomePage(),
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Dashboard</main>`,
			},
		},
	}}

	result, err := Build(config, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	event := findBuildReportEvent(result.Report, "seo", "seo_route_excluded")
	if event == nil {
		t.Fatalf("expected SEO exclusion event in %#v", result.Report.Events)
	}
	if event.PageID != "dashboard" || event.Route != "/dashboard" || event.Data["reason"] != "request_time_rendering" || event.Data["mode"] != "ssr" {
		t.Fatalf("unexpected SEO exclusion event: %#v", event)
	}
}

func TestBuildMemoryCollectsSEOArtifacts(t *testing.T) {
	config := gowdk.Config{Addons: []gowdk.Addon{seo.Addon(seo.Options{BaseURL: "https://example.com"})}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{seoHomePage()}}

	result, err := BuildMemory(config, app, ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Files[sitemapFile]; !ok {
		t.Fatalf("expected in-memory sitemap, got files %#v", result.Files)
	}
	if _, ok := result.Files[robotsFile]; !ok {
		t.Fatalf("expected in-memory robots, got files %#v", result.Files)
	}
	requireBuildReportEvent(t, result.Report, "seo", "sitemap_collected")
	requireBuildReportEvent(t, result.Report, "seo", "robots_collected")
}

func TestBuildRejectsSEOAddonWithoutBaseURL(t *testing.T) {
	outputDir := t.TempDir()
	config := gowdk.Config{Addons: []gowdk.Addon{seo.Addon(seo.Options{})}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{seoHomePage()}}

	_, err := Build(config, app, outputDir)
	if err == nil {
		t.Fatal("expected missing BaseURL error")
	}
	if !strings.Contains(err.Error(), "seo BaseURL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func seoHomePage() gwdkir.Page {
	return gwdkir.Page{
		ID:    "home",
		Route: "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
		},
	}
}
