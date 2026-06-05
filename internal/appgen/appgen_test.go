package appgen

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateWritesEmbeddedStaticApp(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(staticDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	result, err := Generate(staticDir, appDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		result.ModulePath,
		result.MainPath,
		result.PackagePath,
		filepath.Join(result.StaticDir, "index.html"),
		filepath.Join(result.StaticDir, "blog", "hello", "index.html"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(result.Files, ",") != "blog/hello/index.html,gowdk-assets.json,index.html" {
		t.Fatalf("unexpected copied files: %#v", result.Files)
	}
	mainPayload, err := os.ReadFile(result.MainPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"gowdk-generated-app/gowdkapp"`,
		"handler, err := gowdkapp.Handler()",
		"ReadHeaderTimeout: 5 * time.Second",
	} {
		if !strings.Contains(string(mainPayload), expected) {
			t.Fatalf("expected generated server main.go to contain %q:\n%s", expected, mainPayload)
		}
	}
	packagePayload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"package gowdkapp",
		"//go:embed static",
		"func Handler() (http.Handler, error)",
		"func ServeMux() (*http.ServeMux, error)",
		"mux.Handle(\"/\", staticHandler{",
		`response.Header().Set("Allow", "GET, HEAD")`,
		`request.URL.Path == "/_gowdk/health"`,
		`response.Header().Set("X-GOWDK-App", handler.identity.AppID)`,
		`response.Header().Set("X-GOWDK-Instance-ID", handler.identity.InstanceID)`,
		`assets:   loadAssetManifest(root),`,
		`"assets":      strconv.Itoa(len(handler.assets.Files))`,
		`instanceID = generatedInstanceID(moduleName)`,
		`rand.Read(token[:])`,
	} {
		if !strings.Contains(string(packagePayload), expected) {
			t.Fatalf("expected generated gowdkapp/app.go to contain %q:\n%s", expected, packagePayload)
		}
	}
}

func TestGeneratePreservesUnchangedFilesAndRemovesStaleStaticFiles(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, "old.html"), "<main>Old</main>")

	result, err := Generate(staticDir, appDir)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		result.MainPath,
		result.PackagePath,
		result.ModulePath,
		filepath.Join(result.StaticDir, "index.html"),
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
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	if err := os.Remove(filepath.Join(staticDir, "old.html")); err != nil {
		t.Fatal(err)
	}
	result, err = Generate(staticDir, appDir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Files, ",") != "index.html" {
		t.Fatalf("unexpected copied files: %#v", result.Files)
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
	if _, err := os.Stat(filepath.Join(result.StaticDir, "old.html")); !os.IsNotExist(err) {
		t.Fatalf("expected stale embedded static file to be removed, stat err: %v", err)
	}
}

func TestGenerateSkipsUnsafeEmbeddedOutputFiles(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, ".env"), "SECRET=value")
	writeTestFile(t, filepath.Join(staticDir, ".env.local"), "SECRET=value")
	writeTestFile(t, filepath.Join(staticDir, "assets", "app.css.map"), "{}")
	writeTestFile(t, filepath.Join(staticDir, "source", "home.page.gwdk"), "@page home")
	writeTestFile(t, filepath.Join(staticDir, "source", "main.go"), "package main")
	writeTestFile(t, filepath.Join(staticDir, "tmp", "asset.css"), "body{}")
	writeTestFile(t, filepath.Join(staticDir, "assets", "scratch.tmp"), "temporary")
	writeTestFile(t, filepath.Join(staticDir, "assets", "app.css"), "body{}")

	result, err := Generate(staticDir, appDir)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(result.Files, ",") != "assets/app.css,index.html" {
		t.Fatalf("unexpected embedded files: %#v", result.Files)
	}
	for _, path := range []string{
		filepath.Join(result.StaticDir, ".env"),
		filepath.Join(result.StaticDir, ".env.local"),
		filepath.Join(result.StaticDir, "assets", "app.css.map"),
		filepath.Join(result.StaticDir, "source", "home.page.gwdk"),
		filepath.Join(result.StaticDir, "source", "main.go"),
		filepath.Join(result.StaticDir, "tmp", "asset.css"),
		filepath.Join(result.StaticDir, "assets", "scratch.tmp"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected unsafe file %s to be skipped, stat err: %v", path, err)
		}
	}
}

func TestGenerateWritesActionRedirectHandler(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	result, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:         "newsletter",
		ActionName:     "subscribe",
		Route:          "/newsletter",
		InputName:      "input",
		InputType:      "SubscribeInput",
		InputFields:    []string{"email"},
		RequiredFields: []string{"email"},
		ValidatesInput: true,
		Redirect:       "/newsletter?ok=1",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`if request.Method == http.MethodPost && handler.action(response, request)`,
		`case "/newsletter":`,
		`const maxActionBodyBytes int64 = 1 << 20`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`if err := request.ParseForm(); err != nil`,
		`http.StatusRequestEntityTooLarge`,
		`writeActionError(response, http.StatusBadRequest, actionErrorInvalidForm)`,
		`writeActionError(response, http.StatusRequestEntityTooLarge, actionErrorRequestTooLarge)`,
		`writeActionError(response, http.StatusUnprocessableEntity, actionErrorValidationFailed)`,
		`response.Header().Set("Cache-Control", "no-store")`,
		`type SubscribeInput struct`,
		`func decodeNewsletterSubscribeInput(values formValues) (SubscribeInput, error)`,
		`decodeExpectedFields(values, []string{"email"})`,
		`validateRequiredFields(input.Values, []string{"email"})`,
		`http.StatusUnprocessableEntity`,
		`http.Redirect(response, request, "/newsletter?ok=1", http.StatusSeeOther)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesActionFragmentHandler(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "patients", "index.html"), "<main>Patients</main>")

	result, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:      "patients",
		ActionName:  "refresh",
		Route:       "/patients",
		InputName:   "input",
		InputType:   "PatientFilter",
		InputFields: []string{"query"},
		Redirect:    "/patients",
		Fragments: []ActionFragment{{
			Target: "#patients",
			HTML:   "<section><p>Updated patients</p></section>",
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`if isPartialRequest(request)`,
		`writeActionFragment(response, request, []actionFragment{{Target: "#patients", HTML: "<section><p>Updated patients</p></section>"}})`,
		`response.Header().Set("X-GOWDK-Fragment-Target", fragment.Target)`,
		`response.Header().Set("X-GOWDK-Fragment-Swap", swap)`,
		`func partialSwapMode(value string) string`,
		`actionErrorFragmentNotFound = "partial fragment not found"`,
		`http.Redirect(response, request, "/patients", http.StatusSeeOther)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesSSRHandler(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Static</main>")

	result, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "dashboard",
		Route:  "/dashboard",
		HTML:   "<main><h1>Dashboard</h1></main>",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`if handler.ssrExact(response, request)`,
		`if handler.ssrDynamic(response, request)`,
		`func (handler staticHandler) ssrExact(response http.ResponseWriter, request *http.Request) bool`,
		`func (handler staticHandler) ssrDynamic(response http.ResponseWriter, request *http.Request) bool`,
		`case "/dashboard":`,
		`response.Header().Set("Content-Type", "text/html; charset=utf-8")`,
		`response.Header().Set("Cache-Control", "no-store")`,
		`writeSSRHTML(response, request, "<main><h1>Dashboard</h1></main>")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesDynamicSSRHandler(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Static</main>")

	result, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		HTML:   `<main data-slug="__SLUG__">__SLUG__</main>`,
		Replacements: []SSRReplacement{{
			Param:       "slug",
			Placeholder: "__SLUG__",
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`matchSSRRoute("/blog/{slug}", request.URL.Path)`,
		`strings.ReplaceAll(html, "__SLUG__", escapeSSRValue(params["slug"]))`,
		`func escapeSSRValue(value string) string`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesDynamicSSRHandlerWithoutReplacements(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Static</main>")

	result, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		HTML:   `<main>Post</main>`,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	if !strings.Contains(source, `matchSSRRoute("/blog/{slug}", request.URL.Path)`) {
		t.Fatalf("expected generated main.go to match dynamic route:\n%s", source)
	}
	if strings.Contains(source, `case "/blog/{slug}":`) {
		t.Fatalf("expected generated main.go not to use exact literal match for dynamic route:\n%s", source)
	}
}

func TestActionRoutesInfersInputFieldsFromGPostForm(t *testing.T) {
	routes, err := ActionRoutes(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={subscribe}><input name="email" required /><textarea name="note"></textarea></form>`,
			Actions: []manifest.Action{{
				Name:           "subscribe",
				InputName:      "input",
				InputType:      "SubscribeInput",
				ValidatesInput: true,
				Redirect:       "/newsletter?ok=1",
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected one route, got %#v", routes)
	}
	if strings.Join(routes[0].InputFields, ",") != "email,note" {
		t.Fatalf("unexpected fields: %#v", routes[0].InputFields)
	}
	if strings.Join(routes[0].RequiredFields, ",") != "email" {
		t.Fatalf("unexpected required fields: %#v", routes[0].RequiredFields)
	}
	if !routes[0].ValidatesInput {
		t.Fatalf("expected validation metadata: %#v", routes[0])
	}
}

func TestActionRoutesRendersActionFragments(t *testing.T) {
	routes, err := ActionRoutes(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={refresh} g:target="#patients"><input name="query" /></form><section id="patients"></section>`,
			Actions: []manifest.Action{{
				Name:      "refresh",
				InputName: "input",
				InputType: "PatientFilter",
				Fragments: []manifest.Fragment{{
					Target: "#patients",
					Body:   `<p>Updated & safe</p>`,
				}},
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected one route, got %#v", routes)
	}
	if routes[0].Redirect != "" {
		t.Fatalf("did not expect redirect for fragment-only action: %#v", routes[0])
	}
	if len(routes[0].Fragments) != 1 {
		t.Fatalf("expected one fragment, got %#v", routes[0].Fragments)
	}
	if routes[0].Fragments[0].Target != "#patients" || routes[0].Fragments[0].HTML != "<p>Updated &amp; safe</p>" {
		t.Fatalf("unexpected fragment route: %#v", routes[0].Fragments[0])
	}
}

func TestActionRoutesRejectsFileInputsWithPageContext(t *testing.T) {
	_, err := ActionRoutes(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "profile",
		Route: "/profile",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={save}><input name="avatar" type="file" /></form>`,
			Actions: []manifest.Action{{
				Name:     "save",
				Redirect: "/profile?ok=1",
			}},
		},
	}}})
	if err == nil {
		t.Fatal("expected file input error")
	}
	if !strings.Contains(err.Error(), `profile: file input "avatar" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsAppDirInsideStaticOutput(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	_, err := Generate(staticDir, filepath.Join(staticDir, "app"))
	if err == nil {
		t.Fatal("expected app directory validation error")
	}
	if !strings.Contains(err.Error(), "must be outside static output directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsStaticOutputInsideGeneratedStaticDir(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	staticDir := filepath.Join(appDir, "gowdkapp", "static")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	_, err := Generate(staticDir, appDir)
	if err == nil {
		t.Fatal("expected generated static directory validation error")
	}
	if !strings.Contains(err.Error(), "must not be inside generated app static directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsUnsafeActionRedirect(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	_, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:     "newsletter",
		ActionName: "subscribe",
		Route:      "/newsletter",
		Redirect:   "https://example.com",
	}}})
	if err == nil {
		t.Fatal("expected unsafe redirect error")
	}
	if !strings.Contains(err.Error(), `redirect "https://example.com" must be a local absolute path`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsDynamicActionRoute(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "blog", "hello", "index.html"), "<main>Post</main>")

	_, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:     "blog.post",
		ActionName: "save",
		Route:      "/blog/{slug}",
		Redirect:   "/blog/hello",
	}}})
	if err == nil {
		t.Fatal("expected dynamic route error")
	}
	if !strings.Contains(err.Error(), `route "/blog/{slug}" must be a concrete path`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsSSRReplacementForUndeclaredParam(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		HTML:   "<main>Post</main>",
		Replacements: []SSRReplacement{{
			Param:       "missing",
			Placeholder: "__MISSING__",
		}},
	}}})
	if err == nil {
		t.Fatal("expected undeclared replacement error")
	}
	if !strings.Contains(err.Error(), `replacement param "missing" is not declared by route "/blog/{slug}"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsAmbiguousDynamicSSRRoutes(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{
		{
			PageID: "blog.category",
			Route:  "/blog/{category}/{slug}",
			HTML:   "<main>Category</main>",
		},
		{
			PageID: "blog.edit",
			Route:  "/blog/{slug}/edit",
			HTML:   "<main>Edit</main>",
		},
	}})
	if err == nil {
		t.Fatal("expected ambiguous generated SSR route error")
	}
	if !strings.Contains(err.Error(), `route "/blog/{slug}/edit" overlaps dynamic SSR page blog.category route "/blog/{category}/{slug}"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateAllowsConcreteSSRRouteBesideDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{
		{
			PageID: "blog.about",
			Route:  "/blog/about",
			HTML:   "<main>About</main>",
		},
		{
			PageID: "blog.post",
			Route:  "/blog/{slug}",
			HTML:   "<main>Post</main>",
		},
	}}); err != nil {
		t.Fatalf("expected concrete SSR route beside dynamic SSR route to be valid, got %v", err)
	}
}

func TestBuildBinaryCompilesGeneratedApp(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(staticDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	if _, err := Generate(staticDir, appDir); err != nil {
		t.Fatal(err)
	}
	built, err := BuildBinary(appDir, binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if built != binaryPath {
		t.Fatalf("unexpected binary path: %q", built)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWASMCompilesGeneratedApp(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	wasmPath := filepath.Join(root, "site.wasm")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, "gowdk-assets.json"), `{"version":1,"files":{}}`)

	if _, err := Generate(staticDir, appDir); err != nil {
		t.Fatal(err)
	}
	built, err := BuildWASM(appDir, wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if built != wasmPath {
		t.Fatalf("unexpected wasm path: %q", built)
	}
	info, err := os.Stat(wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected wasm artifact to be non-empty")
	}
}

func TestGeneratedBinaryServesEmbeddedStaticHTML(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(staticDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(staticDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	if _, err := Generate(staticDir, appDir); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(),
		"GOWDK_ADDR="+addr,
		"GOWDK_APP_ID=clinic",
		"GOWDK_MODULE_NAME=backend",
		"GOWDK_INSTANCE_ID=backend-2",
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, headers, err := waitForHTTPResponse("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<main>Home</main>") {
		t.Fatalf("unexpected response body: %s", body)
	}
	if headers.Get("X-GOWDK-App") != "clinic" {
		t.Fatalf("unexpected app header: %q", headers.Get("X-GOWDK-App"))
	}
	if headers.Get("X-GOWDK-Module") != "backend" {
		t.Fatalf("unexpected module header: %q", headers.Get("X-GOWDK-Module"))
	}
	if headers.Get("X-GOWDK-Instance-ID") != "backend-2" {
		t.Fatalf("unexpected instance ID header: %q", headers.Get("X-GOWDK-Instance-ID"))
	}

	body, err = waitForHTTP("http://" + addr + "/blog/hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<main>Post</main>") {
		t.Fatalf("unexpected nested response body: %s", body)
	}

	body, err = waitForHTTP("http://" + addr + "/_gowdk/health")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"status":"ok"`,
		`"app":"clinic"`,
		`"module":"backend"`,
		`"instance_id":"backend-2"`,
		`"assets":"1"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected health response to contain %q, got %s", expected, body)
		}
	}
}

func TestGeneratedBinaryServesSSRRouteBeforeStaticFallback(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "dashboard", "index.html"), "<main>Stale static dashboard</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "dashboard",
		Route:  "/dashboard",
		HTML:   "<main><h1>Request Dashboard</h1></main>",
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, headers, err := waitForHTTPResponse("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "<main><h1>Request Dashboard</h1></main>" {
		t.Fatalf("unexpected SSR response body: %s", body)
	}
	if strings.Contains(body, "Stale static dashboard") {
		t.Fatalf("expected SSR route to win over static fallback, got %s", body)
	}
	if contentType := headers.Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if cacheControl := headers.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryServesDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		HTML:   `<main data-slug="__SLUG__"><h1>__SLUG__</h1></main>`,
		Replacements: []SSRReplacement{{
			Param:       "slug",
			Placeholder: "__SLUG__",
		}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, _, err := waitForHTTPResponse("http://" + addr + "/blog/%3Cscript%3E")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != `<main data-slug="&lt;script&gt;"><h1>&lt;script&gt;</h1></main>` {
		t.Fatalf("unexpected dynamic SSR response body: %s", body)
	}
}

func TestGeneratedBinaryServesStaticPageBeforeDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "blog", "about", "index.html"), "<main>Static about</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		HTML:   `<main>Dynamic __SLUG__</main>`,
		Replacements: []SSRReplacement{{
			Param:       "slug",
			Placeholder: "__SLUG__",
		}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, headers, err := waitForHTTPResponse("http://" + addr + "/blog/about")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "<main>Static about</main>" {
		t.Fatalf("expected static page to win over dynamic SSR route, got: %s", body)
	}
	if headers.Get("Cache-Control") == "no-store" {
		t.Fatalf("expected static response headers, got SSR cache header")
	}
}

func TestGeneratedBinaryServesStaticAssetBeforeRootDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "favicon.ico"), "ICON")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "catch.all",
		Route:  "/{slug}",
		HTML:   `<main>Dynamic __SLUG__</main>`,
		Replacements: []SSRReplacement{{
			Param:       "slug",
			Placeholder: "__SLUG__",
		}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, headers, err := waitForHTTPResponse("http://" + addr + "/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "ICON" {
		t.Fatalf("expected static asset to win over root dynamic SSR route, got: %s", body)
	}
	if headers.Get("Cache-Control") == "no-store" {
		t.Fatalf("expected static response headers, got SSR cache header")
	}
}

func TestGeneratedBinaryAutoGeneratesInstanceID(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "index.html"), "<main>Home</main>")

	if _, err := Generate(staticDir, appDir); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(withoutEnv(os.Environ(), "GOWDK_INSTANCE_ID"),
		"GOWDK_ADDR="+addr,
		"GOWDK_APP_ID=clinic",
		"GOWDK_MODULE_NAME=frontend",
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	_, headers, err := waitForHTTPResponse("http://" + addr + "/_gowdk/health")
	if err != nil {
		t.Fatal(err)
	}
	if headers.Get("X-GOWDK-App") != "clinic" {
		t.Fatalf("unexpected app header: %q", headers.Get("X-GOWDK-App"))
	}
	if headers.Get("X-GOWDK-Module") != "frontend" {
		t.Fatalf("unexpected module header: %q", headers.Get("X-GOWDK-Module"))
	}
	instanceID := headers.Get("X-GOWDK-Instance-ID")
	if !strings.HasPrefix(instanceID, "frontend-") {
		t.Fatalf("expected autogenerated frontend instance ID, got %q", instanceID)
	}
}

func TestGeneratedBinaryRedirectsActionPOST(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:         "newsletter",
		ActionName:     "subscribe",
		Route:          "/newsletter",
		InputName:      "input",
		InputType:      "SubscribeInput",
		InputFields:    []string{"email"},
		RequiredFields: []string{"email"},
		ValidatesInput: true,
		Redirect:       "/newsletter?ok=1",
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	response, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com")
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

	response, err = waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com&role=admin")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unexpected field to return 400, got %d", response.StatusCode)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on action error, got %q", cacheControl)
	}

	response, err = waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected missing required field to return 422, got %d", response.StatusCode)
	}

	response, err = waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email="+strings.Repeat("a", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected oversized form to return 413, got %d", response.StatusCode)
	}
}

func TestGeneratedBinaryServesPartialActionFragment(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:      "patients",
		ActionName:  "refresh",
		Route:       "/patients",
		InputName:   "input",
		InputType:   "PatientFilter",
		InputFields: []string{"query"},
		Redirect:    "/patients",
		Fragments: []ActionFragment{{
			Target: "#patients",
			HTML:   "<section><p>Updated patients</p></section>",
		}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	response, err := waitForHTTPStatusWithHeaders("http://"+addr+"/patients", http.MethodPost, "query=active", map[string]string{
		"X-GOWDK-Partial": "1",
		"X-GOWDK-Target":  "#patients",
		"X-GOWDK-Swap":    "outerHTML",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected fragment response status 200, got %d: %s", response.StatusCode, payload)
	}
	if strings.TrimSpace(string(payload)) != "<section><p>Updated patients</p></section>" {
		t.Fatalf("unexpected fragment body: %s", payload)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if response.Header.Get("X-GOWDK-Fragment-Target") != "#patients" {
		t.Fatalf("unexpected fragment target: %q", response.Header.Get("X-GOWDK-Fragment-Target"))
	}
	if response.Header.Get("X-GOWDK-Fragment-Swap") != "outerHTML" {
		t.Fatalf("unexpected fragment swap: %q", response.Header.Get("X-GOWDK-Fragment-Swap"))
	}
	if response.Header.Get("Location") != "" {
		t.Fatalf("did not expect redirect location on partial response: %q", response.Header.Get("Location"))
	}
}

func TestGeneratedBinaryDoesNotValidateRequiredFieldsWithoutValidMetadata(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(staticDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	if _, err := GenerateWithOptions(staticDir, appDir, Options{Actions: []ActionRoute{{
		PageID:         "newsletter",
		ActionName:     "subscribe",
		Route:          "/newsletter",
		InputName:      "input",
		InputType:      "SubscribeInput",
		InputFields:    []string{"email"},
		RequiredFields: []string{"email"},
		Redirect:       "/newsletter?ok=1",
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	response, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect without validation metadata, got %d", response.StatusCode)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func freeAddr(t *testing.T) string {
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

func withoutEnv(env []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[name] = true
	}

	var filtered []string
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok && blocked[name] {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func waitForHTTP(url string) (string, error) {
	body, _, err := waitForHTTPResponse(url)
	return body, err
}

func waitForHTTPResponse(url string) (string, http.Header, error) {
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
		payload, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = nil
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return string(payload), response.Header.Clone(), nil
	}
	if lastErr != nil {
		return "", nil, lastErr
	}
	return "", nil, os.ErrDeadlineExceeded
}

func waitForHTTPStatus(url, method, body string) (*http.Response, error) {
	return waitForHTTPStatusWithHeaders(url, method, body, nil)
}

func waitForHTTPStatusWithHeaders(url, method, body string, headers map[string]string) (*http.Response, error) {
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
		for name, value := range headers {
			request.Header.Set(name, value)
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
