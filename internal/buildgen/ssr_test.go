package buildgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildSkipsRequestTimePagesAndKeepsSPAArtifacts(t *testing.T) {
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
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:         "dashboard",
			Route:      "/dashboard",
			Render:     gowdk.SSR,
			Cache:      "public, max-age=45",
			Revalidate: "15",
			ErrorPage:  "errors/dashboard.html",
			Blocks: manifest.Blocks{
				BuildBody: `=> { title: "Dashboard" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1><p>Live</p></main>`,
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
}

func TestSSRArtifactsFromIRRenderConcreteSSRPage(t *testing.T) {
	outputDir := t.TempDir()
	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				BuildBody: `=> { title: "Dashboard" }`,
				View:      true,
				ViewBody:  `<main><h1>{title}</h1><p>Live</p></main>`,
			},
		}},
	}

	artifacts, err := SSRArtifactsFromIR(config, gwdkanalysis.BuildIR(config, app), outputDir)
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
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Guard:  []string{"auth.required"},
			Blocks: manifest.Blocks{
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
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "blog.post",
			Route:  "/blog/{slug}",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
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
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
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
