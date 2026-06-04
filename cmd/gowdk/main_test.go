package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

import "github.com/gowdk/gowdk"

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

func TestBuildCommandDiscoversConfiguredModules(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/gowdk/gowdk"

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

import "github.com/gowdk/gowdk"

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

import "github.com/gowdk/gowdk"

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

import "github.com/gowdk/gowdk"

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
