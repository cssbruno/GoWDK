package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildSkipsRequestTimePagesAndKeepsSPAArtifacts(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main>Dashboard</main>`,
			},
		},
		{
			ID:    "blog.post",
			Route: "/blog/{slug}",
			Blocks: gwdkir.Blocks{
				Paths:     true,
				PathsBody: `=> { slug: "hello-gowdk" }`,
				View:      true,
				ViewBody:  `<main>Post</main>`,
			},
		},
	}}

	result, err := Build(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected only one app artifact, got %#v", result.Artifacts)
	}
	if result.Artifacts[0].PageID != "blog.post" {
		t.Fatalf("expected SSR page to be skipped, got %#v", result.Artifacts)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "dashboard", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no SSR build output, stat err: %v", err)
	}
}

func TestSSRArtifactsRenderConcreteSSRPage(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:         "dashboard",
			Route:      "/dashboard",
			Render:     gowdk.SSR,
			Cache:      "public, max-age=45",
			Revalidate: "15",
			ErrorPage:  "errors/dashboard.html",
			Blocks: gwdkir.Blocks{
				BuildBody: `=> { title: "Dashboard" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1><p>Live</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{
		Build: gowdk.BuildConfig{
			Scripts: []gowdk.Script{{Src: "/assets/app.js", Type: "module"}},
		},
		Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	if artifacts[0].PageID != "dashboard" || artifacts[0].Route != "/dashboard" || artifacts[0].Render != gowdk.SSR {
		t.Fatalf("unexpected SSR artifact metadata: %#v", artifacts[0])
	}
	if artifacts[0].Cache != "public, max-age=45, stale-while-revalidate=15" {
		t.Fatalf("unexpected SSR cache policy: %#v", artifacts[0])
	}
	if artifacts[0].ErrorPage != "errors/dashboard.html" {
		t.Fatalf("unexpected SSR error page: %#v", artifacts[0])
	}
	if !strings.Contains(artifacts[0].HTML, "<h1>Dashboard</h1>") {
		t.Fatalf("expected rendered SSR HTML, got %s", artifacts[0].HTML)
	}
	if !strings.Contains(artifacts[0].HTML, `<script type="module" src="/assets/app.js" defer></script>`) {
		t.Fatalf("expected configured script in SSR HTML, got %s", artifacts[0].HTML)
	}
}

func TestSSRArtifactsIncludeScopedJSScripts(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			Source:  "pages/dashboard.page.gwdk",
			Package: "pages",
			ID:      "dashboard",
			Route:   "/dashboard",
			Render:  gowdk.SSR,
			JS:      []string{"./dashboard.js"},
			Uses:    []gwdkir.Use{{Alias: "charts", Package: "components"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><charts.SignupsChart /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Source:  "components/signups-chart.cmp.gwdk",
			Package: "components",
			Name:    "SignupsChart",
			JS:      []string{"./chart.js"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<section></section>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	html := artifacts[0].HTML
	if !strings.Contains(html, `<script type="module" src="/assets/gowdk/pages/dashboard/dashboard.js" defer></script>`) {
		t.Fatalf("expected page scoped JS in SSR HTML, got %s", html)
	}
	if !strings.Contains(html, `<script type="module" src="/assets/gowdk/components/components/SignupsChart/chart.js" defer></script>`) {
		t.Fatalf("expected component scoped JS in SSR HTML, got %s", html)
	}
}

func TestSSRArtifactsRenderHybridPageWithoutLoad(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:     "marketing",
			Route:  "/marketing",
			Render: gowdk.Hybrid,
			Blocks: gwdkir.Blocks{
				BuildBody: `=> { title: "Marketing" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one hybrid artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if artifact.PageID != "marketing" || artifact.Route != "/marketing" || artifact.Render != gowdk.Hybrid {
		t.Fatalf("unexpected hybrid artifact metadata: %#v", artifact)
	}
	if artifact.HasLoad {
		t.Fatalf("expected hybrid route without load metadata, got %#v", artifact)
	}
	if !strings.Contains(artifact.HTML, "<h1>Marketing</h1>") {
		t.Fatalf("expected rendered hybrid HTML, got %s", artifact.HTML)
	}
}

func TestSSRArtifactsFromIRRenderConcreteSSRPage(t *testing.T) {
	outputDir := t.TempDir()
	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				BuildBody: `=> { title: "Dashboard" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1><p>Live</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifactsFromIR(config, gwdkanalysis.BuildProgram(config, app), outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	if artifacts[0].PageID != "dashboard" || artifacts[0].Render != gowdk.SSR || !strings.Contains(artifacts[0].HTML, "<h1>Dashboard</h1>") {
		t.Fatalf("unexpected SSR artifact: %#v", artifacts[0])
	}
}

func TestSSRArtifactsRenderDynamicSSRPageWithPlaceholders(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Guards: []string{"auth.required"},
			Blocks: gwdkir.Blocks{
				BuildBody: `=> { title: "Post {slug}" }`,
				View:      true,
				ViewBody:  `<main data-slug="{param(\"slug\")}"><h1>{title}</h1><p>{param("slug")}</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one SSR artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if artifact.Route != "/blog/{slug}" {
		t.Fatalf("unexpected dynamic route: %#v", artifact)
	}
	if len(artifact.DynamicParams) != 1 || artifact.DynamicParams[0] != "slug" || len(artifact.Guards) != 1 || artifact.Guards[0] != "auth.required" {
		t.Fatalf("unexpected route metadata: %#v", artifact)
	}
	if len(artifact.Replacements) != 1 || artifact.Replacements[0].Param != "slug" {
		t.Fatalf("unexpected replacements: %#v", artifact.Replacements)
	}
	if !strings.Contains(artifact.HTML, artifact.Replacements[0].Placeholder) {
		t.Fatalf("expected SSR HTML placeholder %q in %s", artifact.Replacements[0].Placeholder, artifact.HTML)
	}
}

func TestSSRArtifactsRejectRouteParamInDangerousAttribute(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<img src="x" onerror="{param(\"slug\")}" />`,
			},
		}},
	}

	_, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err == nil {
		t.Fatal("expected dangerous route param attribute error")
	}
	if !strings.Contains(err.Error(), `route param interpolation is not allowed in "onerror" attributes`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSRArtifactsRenderLoadPlaceholders(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { user.name, account.plan }`,
			View:     true,
			ViewBody: `<main><h1>{user.name}</h1><p>{account.plan}</p></main>`,
		},
	}}}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if !artifact.HasLoad {
		t.Fatalf("expected load metadata, got %#v", artifact)
	}
	if len(artifact.LoadReplacements) != 2 {
		t.Fatalf("expected load replacements, got %#v", artifact.LoadReplacements)
	}
	paths := map[string]bool{}
	for _, replacement := range artifact.LoadReplacements {
		paths[replacement.Path] = true
		if !strings.Contains(artifact.HTML, replacement.Placeholder) {
			t.Fatalf("expected placeholder %q in HTML %s", replacement.Placeholder, artifact.HTML)
		}
	}
	if !paths["user.name"] || !paths["account.plan"] {
		t.Fatalf("expected dotted load paths, got %#v", artifact.LoadReplacements)
	}
}

func TestSSRArtifactsComposePageLoadThroughLayouts(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "dashboard",
			Route:   "/dashboard",
			Render:  gowdk.SSR,
			Layouts: []string{"shell"},
			Blocks: gwdkir.Blocks{
				Load:     true,
				LoadBody: `=> { user.name }`,
				View:     true,
				ViewBody: `<main>{user.name}</main>`,
			},
		}},
		Layouts: []gwdkir.Layout{{
			ID: "shell",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<section><header>{user.name}</header><slot /></section>`,
			},
		}},
	}

	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if len(artifact.LoadReplacements) != 1 || artifact.LoadReplacements[0].Path != "user.name" {
		t.Fatalf("expected page load replacement to be shared with layout, got %#v", artifact.LoadReplacements)
	}
	placeholder := artifact.LoadReplacements[0].Placeholder
	if strings.Count(artifact.HTML, placeholder) != 2 {
		t.Fatalf("expected page load placeholder in layout and page body, got:\n%s", artifact.HTML)
	}
	if !strings.Contains(artifact.HTML, "<header>"+placeholder+"</header><main>"+placeholder+"</main>") {
		t.Fatalf("expected layout and page to compose around page load data, got:\n%s", artifact.HTML)
	}
}
