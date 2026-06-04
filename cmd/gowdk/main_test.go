package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildCommandWritesIndexHTML(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	if err := os.WriteFile(source, []byte(`@page home
@route "/"

view {
  <main>
    <h1>GOWDK & friends</h1>
  </main>
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"build", "--out", outputDir, source}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, "<main><h1>GOWDK &amp; friends</h1></main>") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestWatchCommandOnceRunsBuild(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, source, `@page home
@route "/"

view {
  <main>Watched</main>
}
`)

	if err := run([]string{"watch", "--once", "--out", outputDir, source}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "<main>Watched</main>") {
		t.Fatalf("unexpected watch build output:\n%s", payload)
	}
}

func TestInitCommandScaffoldsBuildableProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	if err := run([]string{"init", root}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"gowdk.config.go",
		"src/pages/home.page.gwdk",
		"src/components/hero.cmp.gwdk",
		"styles/global.css",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected scaffold file %s: %v", path, err)
		}
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()

	if err := run([]string{"build"}); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(root, "dist", "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "Hello from GOWDK") {
		t.Fatalf("unexpected initialized build output:\n%s", payload)
	}
}

func TestInitCommandRejectsExistingFilesUnlessForced(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, "package custom\n")

	err := run([]string{"init", root})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "package custom\n" {
		t.Fatalf("init without --force overwrote config:\n%s", payload)
	}

	if err := run([]string{"init", "--force", root}); err != nil {
		t.Fatal(err)
	}
	payload, err = os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "var Config = gowdk.Config") {
		t.Fatalf("init --force did not refresh config:\n%s", payload)
	}
}

func TestWatchRejectsInvalidInterval(t *testing.T) {
	err := run([]string{"watch", "--interval", "0s", "--out", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "watch interval must be positive") {
		t.Fatalf("expected invalid interval error, got %v", err)
	}
}

func TestBuildInputSnapshotDetectsFileChanges(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, source, `@page home
@route "/"

view {
  <main>Before</main>
}
`)

	first, err := buildInputSnapshot([]string{"--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	writeCLIFile(t, source, `@page home
@route "/"

view {
  <main>After</main>
}
`)
	second, err := buildInputSnapshot([]string{"--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	if second.same(first) {
		t.Fatalf("expected changed source snapshot: first=%#v second=%#v", first, second)
	}
}

func TestBuildCommandWritesComponentExpandedHTML(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	if err := os.WriteFile(page, []byte(`@page home
@route "/"

view {
  <main>
    <Hero title="GOWDK" />
  </main>
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(component, []byte(`@component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"build", "--out", outputDir, page, component}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, "<main><section><h1>GOWDK</h1></section></main>") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestBuildCommandDiscoversFilesWhenNoPathsArePassed(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>
    <Hero title="Discovered" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `@component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`)
	writeCLIFile(t, filepath.Join(root, "dist", "stale.page.gwdk"), `@page stale
@route "/stale"

view {
  <main>stale</main>
}
`)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()

	if err := run([]string{"build", "--out", "dist"}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	if !strings.Contains(output, "<main><section><h1>Discovered</h1></section></main>") {
		t.Fatalf("unexpected output:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "stale", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected output directory source file to be excluded, stat err: %v", err)
	}
	routeManifest, err := os.ReadFile(filepath.Join(root, "dist", "gowdk-routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routeManifest), `"path": "index.html"`) {
		t.Fatalf("unexpected route manifest: %s", routeManifest)
	}
}

func TestBuildCommandUsesConfigForDiscoveryAndOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
		Exclude: []string{"src/ignored.page.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "public",
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>
    <Hero title="Configured" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `@component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "ignored.page.gwdk"), `@page ignored
@route "/ignored"

view {
  <main>Ignored</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build"}); err != nil {
			t.Fatal(err)
		}
	})

	payload, err := os.ReadFile(filepath.Join(root, "public", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "<main><section><h1>Configured</h1></section></main>") {
		t.Fatalf("unexpected output:\n%s", payload)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "ignored", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected configured exclude to skip ignored page, stat err: %v", err)
	}
}

func TestBuildCommandUsesTailwindAddonFromConfig(t *testing.T) {
	root := t.TempDir()
	fakeTailwind := filepath.Join(root, "tailwindcss")
	writeCLIFile(t, fakeTailwind, `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o|--output)
			shift
			out="$1"
			;;
	esac
	shift
done
if [ "$out" = "" ]; then
	echo "missing output" >&2
	exit 2
fi
printf '/* fake tailwind */\n.font-bold { font-weight: 700; }\n' > "$out"
`)
	if err := os.Chmod(fakeTailwind, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{
			Input: "styles/app.css",
			Command: `+strconv.Quote(fakeTailwind)+`,
			OutputPath: "assets/tw.css",
			Href: "/assets/tw.css",
			Minify: true,
		}),
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "styles", "app.css"), `@import "tailwindcss" source(none);
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `@page home
@route "/"

view {
  <main class="font-bold">Home</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--out", "dist", "home.page.gwdk"}); err != nil {
			t.Fatal(err)
		}
	})

	html, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<link rel="stylesheet" href="/assets/tw.css">`) {
		t.Fatalf("expected tailwind stylesheet link:\n%s", html)
	}
	css, err := os.ReadFile(filepath.Join(root, "dist", "assets", "tw.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "fake tailwind") {
		t.Fatalf("expected fake tailwind output, got %q", css)
	}
}

func TestBuildCommandDiscoversConfiguredModules(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend", Type: "frontend"},
		{
			Name: "frontend2",
			Type: "marketing-ui",
			Source: gowdk.SourceConfig{
				Include: []string{"ui2/**/*.gwdk"},
			},
		},
		{
			Name: "backend",
			Type: "backendmicroservice",
			Source: gowdk.SourceConfig{
				Include: []string{"backend/**/*.gwdk"},
				Exclude: []string{"backend/ignored.page.gwdk"},
			},
		},
	},
	Build: gowdk.BuildConfig{
		Output: "dist",
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "ui2", "second.page.gwdk"), `@page second
@route "/second"

view {
  <main>Frontend Two</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `@page admin
@route "/admin"

view {
  <main>Backend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "ignored.page.gwdk"), `@page ignored
@route "/ignored"

view {
  <main>Ignored</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "other", "stray.page.gwdk"), `@page stray
@route "/stray"

view {
  <main>Stray</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build"}); err != nil {
			t.Fatal(err)
		}
	})

	for _, path := range []string{
		filepath.Join(root, "dist", "index.html"),
		filepath.Join(root, "dist", "second", "index.html"),
		filepath.Join(root, "dist", "admin", "index.html"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "dist", "ignored", "index.html"),
		filepath.Join(root, "dist", "stray", "index.html"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be skipped, stat err: %v", path, err)
		}
	}
}

func TestBuildCommandDiscoversSelectedModuleOnly(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend", Type: "frontend"},
		{Name: "backend", Type: "backendmicroservice"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist",
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `@page admin
@route "/admin"

view {
  <main>Backend</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--module", "backend"}); err != nil {
			t.Fatal(err)
		}
	})

	if _, err := os.Stat(filepath.Join(root, "dist", "admin", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected unselected frontend module to be skipped, stat err: %v", err)
	}
}

func TestBuildCommandOutFlagOverridesConfigOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "configured-dist",
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Override</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--out", "custom-dist"}); err != nil {
			t.Fatal(err)
		}
	})

	if _, err := os.Stat(filepath.Join(root, "custom-dist", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "configured-dist", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected --out to override config output, stat err: %v", err)
	}
}

func TestBuildCommandLoadsExplicitConfigPath(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "site.gowdk.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"pages/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist",
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Custom config</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--config", "site.gowdk.go"}); err != nil {
			t.Fatal(err)
		}
	})

	payload, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "<main>Custom config</main>") {
		t.Fatalf("unexpected output:\n%s", payload)
	}
}

func TestRunWithoutArgsPrintsUsage(t *testing.T) {
	output, err := captureCLIStdout(t, func() error {
		return run(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Commands:") || !strings.Contains(output, "check [--config <file>]") {
		t.Fatalf("expected usage output, got:\n%s", output)
	}
}

func TestCLIRejectsUnknownCommandAndProjectFlag(t *testing.T) {
	_, err := captureCLIStdout(t, func() error {
		return run([]string{"unknown"})
	})
	if err == nil || !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("expected unknown command error, got %v", err)
	}

	err = run([]string{"check", "--wat"})
	if err == nil || !strings.Contains(err.Error(), `unknown check flag "--wat"`) {
		t.Fatalf("expected unknown check flag error, got %v", err)
	}

	err = run([]string{"manifest", "--json"})
	if err == nil || !strings.Contains(err.Error(), `unknown manifest flag "--json"`) {
		t.Fatalf("expected unknown manifest flag error, got %v", err)
	}
}

func TestCheckCommandJSONReportsDiagnostics(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "bad.page.gwdk")
	writeCLIFile(t, source, `@page bad
@route "/bad"
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"check", "--json", source})
	})
	if err == nil {
		t.Fatal("expected check to fail")
	}
	if !strings.Contains(output, `"diagnostics"`) || !strings.Contains(output, "missing view") {
		t.Fatalf("expected JSON diagnostics, got:\n%s", output)
	}
}

func TestManifestCommandHandlesMultipleFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	writeCLIFile(t, home, `@page home
@route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, about, `@page about
@route "/about"

view {
  <main>About</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"manifest", home, about})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"home"`) || !strings.Contains(output, `"about"`) {
		t.Fatalf("expected multi-file manifest, got:\n%s", output)
	}
}

func TestCheckCommandUsesConfigForDiscovery(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"pages/**/*.gwdk"},
		Exclude: []string{"pages/ignored.page.gwdk"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Configured check</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `@page ignored
@route "/"

view {
  <main>Ignored duplicate</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"check"}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestManifestCommandLoadsExplicitConfigPathForDiscovery(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "site.gowdk.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"pages/**/*.gwdk"},
		Exclude: []string{"pages/ignored.page.gwdk"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Manifest discovery</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `@page ignored
@route "/ignored"

view {
  <main>Ignored</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"manifest", "--config", "site.gowdk.go"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	if !strings.Contains(output, `"route": "/"`) {
		t.Fatalf("expected discovered home route in manifest: %s", output)
	}
	if strings.Contains(output, "ignored") {
		t.Fatalf("expected configured exclude to skip ignored page: %s", output)
	}
}

func TestSitemapCommandDiscoversSelectedModuleOnly(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend"},
		{Name: "backend"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `@page admin
@route "/admin"

view {
  <main>Backend</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"sitemap", "--module", "backend"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	if !strings.Contains(output, `"id": "admin"`) {
		t.Fatalf("expected selected backend page in sitemap: %s", output)
	}
	if strings.Contains(output, `"id": "home"`) {
		t.Fatalf("expected unselected frontend module to be skipped: %s", output)
	}
}

func TestRoutesCommandPrintsGeneratedRouteBindings(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	writeCLIFile(t, page, `@page newsletter
@route "/newsletter"
@render action

act subscribe {
  input := form SubscribeInput
  valid(input)?
  -> "/newsletter?ok=1"
}

view {
  <form g:post={subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"routes", page})
	})
	if err != nil {
		t.Fatal(err)
	}

	var report routeBindingsReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("unexpected routes version: %d", report.Version)
	}
	if len(report.Routes) != 2 {
		t.Fatalf("expected static and action routes, got %#v", report.Routes)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "static",
		Method:  "GET",
		Route:   "/newsletter",
		PageID:  "newsletter",
		Handler: `embedded.Static("pages/newsletter.html")`,
	})
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "action",
		Method:  "POST",
		Route:   "/newsletter",
		PageID:  "newsletter",
		Handler: "actions.NewsletterSubscribe",
	})
}

func TestRoutesCommandDiscoversSelectedModuleOnly(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend"},
		{Name: "backend"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `@page admin
@route "/admin"

view {
  <main>Backend</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"routes", "--module", "backend"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	var report routeBindingsReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("expected selected backend route only, got %#v", report.Routes)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "static",
		Method:  "GET",
		Route:   "/admin",
		PageID:  "admin",
		Handler: `embedded.Static("pages/admin.html")`,
	})
}

func TestRoutesCommandPrintsAPIBinding(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "status.page.gwdk")
	writeCLIFile(t, page, `@page status
@route "/status"

api health {
  GET "/api/health"
}

view {
  <main>Status</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"routes", page})
	})
	if err != nil {
		t.Fatal(err)
	}

	var report routeBindingsReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "api",
		Method:  "GET",
		Route:   "/api/health",
		PageID:  "status",
		Handler: "api.StatusHealth",
	})
}

func TestBuildCommandWritesGeneratedEmbeddedApp(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	writeCLIFile(t, page, `@page home
@route "/"

view {
  <main>Generated app</main>
}
`)

	if err := run([]string{"build", "--out", outputDir, "--app", appDir, page}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(appDir, "go.mod"),
		filepath.Join(appDir, "main.go"),
		filepath.Join(appDir, "static", "index.html"),
		filepath.Join(appDir, "static", "gowdk-routes.json"),
		filepath.Join(appDir, "static", "gowdk-assets.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildCommandBinRequiresGeneratedApp(t *testing.T) {
	err := run([]string{"build", "--out", t.TempDir(), "--bin", filepath.Join(t.TempDir(), "site")})
	if err == nil {
		t.Fatal("expected --bin without --app to fail")
	}
	if !strings.Contains(err.Error(), "--bin requires --app") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := run([]string{"build", "--out", t.TempDir(), "--app="}); err == nil {
		t.Fatal("expected empty --app to fail")
	}
	if err := run([]string{"build", "--out", t.TempDir(), "--app", filepath.Join(t.TempDir(), "app"), "--bin="}); err == nil {
		t.Fatal("expected empty --bin to fail")
	}
}

func TestBuildCommandBuildsRunnableEmbeddedBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	writeCLIFile(t, page, `@page home
@route "/"

view {
  <main>One binary</main>
}
`)

	if err := run([]string{"build", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, err := waitForCLIHTTP("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<main>One binary</main>") {
		t.Fatalf("unexpected response body: %s", body)
	}
}

func TestBuildCommandEmbedsSelectedModuleOnlyInBinary(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend"},
		{Name: "admin"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `@page dashboard
@route "/admin"

view {
  <main>Admin module</main>
}
`)

	outputDir := filepath.Join(root, "dist-admin")
	appDir := filepath.Join(root, "app-admin")
	binaryPath := filepath.Join(root, "admin-site")
	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--module", "admin", "--out", outputDir, "--app", appDir, "--bin", binaryPath}); err != nil {
			t.Fatal(err)
		}
	})

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, err := waitForCLIHTTP("http://" + addr + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<main>Admin module</main>") {
		t.Fatalf("unexpected selected module response: %s", body)
	}

	response, err := waitForCLIStatus("http://"+addr+"/", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unselected frontend route to be absent, got %d", response.StatusCode)
	}
}

func TestBuildCommandEmbedsMultipleSelectedModulesInBinary(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend"},
		{Name: "admin"},
		{Name: "docs"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `@page home
@route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `@page dashboard
@route "/admin"

view {
  <main>Admin module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "docs", "guide.page.gwdk"), `@page guide
@route "/docs"

view {
  <main>Docs module</main>
}
`)

	outputDir := filepath.Join(root, "dist-combined")
	appDir := filepath.Join(root, "app-combined")
	binaryPath := filepath.Join(root, "combined-site")
	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--module", "frontend,admin", "--out", outputDir, "--app", appDir, "--bin", binaryPath}); err != nil {
			t.Fatal(err)
		}
	})

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	home, err := waitForCLIHTTP("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home, "<main>Frontend module</main>") {
		t.Fatalf("unexpected frontend module response: %s", home)
	}
	admin, err := waitForCLIHTTP("http://" + addr + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(admin, "<main>Admin module</main>") {
		t.Fatalf("unexpected admin module response: %s", admin)
	}

	response, err := waitForCLIStatus("http://"+addr+"/docs", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unselected docs route to be absent, got %d", response.StatusCode)
	}
}

func TestBuildCommandBuildsSSRBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "dashboard.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	writeCLIFile(t, page, `@page dashboard
@route "/dashboard"
@render ssr

build {
  => { title: "Dashboard" }
}

view {
  <main>
    <h1>{title}</h1>
  </main>
}
`)

	if err := run([]string{"build", "--ssr", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "dashboard", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no static SSR HTML artifact, stat err: %v", err)
	}

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, err := waitForCLIHTTP("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<h1>Dashboard</h1>") {
		t.Fatalf("unexpected SSR response body: %s", body)
	}
}

func TestBuildCommandBuildsDynamicSSRBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "post.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	writeCLIFile(t, page, `@page blog.post
@route "/blog/{slug}"
@render ssr

build {
  => { title: "Post {slug}" }
}

view {
  <main data-slug={param("slug")}>
    <h1>{title}</h1>
    <p>{param("slug")}</p>
  </main>
}
`)

	if err := run([]string{"build", "--ssr", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
		t.Fatal(err)
	}

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, err := waitForCLIHTTP("http://" + addr + "/blog/hello-gowdk")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `<main data-slug="hello-gowdk"><h1>Post hello-gowdk</h1><p>hello-gowdk</p></main>`) {
		t.Fatalf("unexpected dynamic SSR response body: %s", body)
	}

	body, err = waitForCLIHTTP("http://" + addr + "/blog/%3Cscript%3E")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected escaped dynamic SSR param, got: %s", body)
	}
}

func TestBuildCommandBuildsActionRedirectBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	writeCLIFile(t, page, `@page newsletter
@route "/newsletter"
@render static

act subscribe {
  input := form SubscribeInput
  valid(input)?
  -> "/newsletter?ok=1"
}

view {
  <form g:post={subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	if err := run([]string{"build", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
		t.Fatal(err)
	}

	addr := freeCLIAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	response, err := waitForCLIStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", response.StatusCode)
	}
	if response.Header.Get("Location") != "/newsletter?ok=1" {
		t.Fatalf("unexpected redirect location: %q", response.Header.Get("Location"))
	}
}

func TestStaticFileHandlerServesRootIndex(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), `<main>Home</main>`)

	response := httptest.NewRecorder()
	staticFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "<main>Home</main>") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestStaticFileHandlerServesExtensionlessNestedIndex(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "blog", "hello-gowdk", "index.html"), `<main>Post</main>`)

	response := httptest.NewRecorder()
	staticFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/blog/hello-gowdk", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "<main>Post</main>") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestStaticFileHandlerRejectsUnsupportedMethods(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), `<main>Home</main>`)

	response := httptest.NewRecorder()
	staticFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("unexpected Allow header: %q", response.Header().Get("Allow"))
	}
}

func TestStaticFileHandlerDoesNotListDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	staticFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", response.Code, response.Body.String())
	}
}

func TestServeCommandRejectsMissingDirectory(t *testing.T) {
	err := serve([]string{"--dir", filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected missing directory error")
	}
	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "cannot find") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseServeOptions(t *testing.T) {
	dir, addr, err := parseServeOptions([]string{"--dir=dist", "--addr=127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if dir != "dist" || addr != "127.0.0.1:0" {
		t.Fatalf("unexpected serve options: dir=%q addr=%q", dir, addr)
	}

	_, _, err = parseServeOptions(nil)
	if err == nil {
		t.Fatal("expected missing dir error")
	}
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func captureCLIStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
	}()

	runErr := fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	payload, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(payload), runErr
}

func assertRouteBinding(t *testing.T, routes []routeBindingJSON, expected routeBindingJSON) {
	t.Helper()
	for _, route := range routes {
		if route == expected {
			return
		}
	}
	t.Fatalf("missing route binding %#v in %#v", expected, routes)
}

func freeCLIAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitForCLIHTTP(url string) (string, error) {
	deadline := time.Now().Add(10 * time.Second)
	client := http.Client{Timeout: 500 * time.Millisecond}
	var lastErr error
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		payload, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if response.StatusCode == http.StatusOK {
			return string(payload), nil
		}
		lastErr = nil
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", os.ErrDeadlineExceeded
}

func waitForCLIStatus(url, method, body string) (*http.Response, error) {
	deadline := time.Now().Add(10 * time.Second)
	client := http.Client{
		Timeout: 500 * time.Millisecond,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var lastErr error
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		if body != "" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return response, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, os.ErrDeadlineExceeded
}
