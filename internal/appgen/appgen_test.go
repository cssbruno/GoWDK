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

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateWritesEmbeddedSPAApp(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	result, err := Generate(outputDir, appDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		result.ModulePath,
		result.MainPath,
		result.PackagePath,
		filepath.Join(result.OutputDir, "index.html"),
		filepath.Join(result.OutputDir, "blog", "hello", "index.html"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(result.Files, ",") != "blog/hello/index.html,gowdk-assets.json,index.html" {
		t.Fatalf("unexpected copied files: %#v", result.Files)
	}
	modulePayload, err := os.ReadFile(result.ModulePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modulePayload), "require github.com/cssbruno/gowdk") {
		t.Fatalf("expected generated app to depend on GOWDK runtime module:\n%s", modulePayload)
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
		"//go:embed app",
		"func Handler() (http.Handler, error)",
		"func ServeMux() (*http.ServeMux, error)",
		`gowdkruntime "github.com/cssbruno/gowdk/runtime/app"`,
		`mux.Handle("/", gowdkruntime.Handler{`,
		`Identity: gowdkruntime.InstanceIdentity(),`,
		`Assets:   gowdkruntime.LoadAssetManifest(root),`,
		`Backend:  backend,`,
		`SSRExact:   ssrExact,`,
		`SSRDynamic: ssrDynamic,`,
	} {
		if !strings.Contains(string(packagePayload), expected) {
			t.Fatalf("expected generated gowdkapp/app.go to contain %q:\n%s", expected, packagePayload)
		}
	}
	for _, copiedRuntime := range []string{
		"type SPAHandler struct",
		"func loadAssetManifest",
		"func generatedInstanceID",
		"rand.Read(token[:])",
		`request.URL.Path == "/_gowdk/health"`,
	} {
		if strings.Contains(string(packagePayload), copiedRuntime) {
			t.Fatalf("expected generated gowdkapp/app.go not to copy runtime helper %q:\n%s", copiedRuntime, packagePayload)
		}
	}
}

func TestGeneratePreservesUnchangedFilesAndRemovesStaleSPAFiles(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "old.html"), "<main>Old</main>")

	result, err := Generate(outputDir, appDir)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		result.MainPath,
		result.PackagePath,
		result.ModulePath,
		filepath.Join(result.OutputDir, "index.html"),
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
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	if err := os.Remove(filepath.Join(outputDir, "old.html")); err != nil {
		t.Fatal(err)
	}
	result, err = Generate(outputDir, appDir)
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
	if _, err := os.Stat(filepath.Join(result.OutputDir, "old.html")); !os.IsNotExist(err) {
		t.Fatalf("expected stale embedded app file to be removed, stat err: %v", err)
	}
}

func TestGenerateSkipsUnsafeEmbeddedOutputFiles(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, ".env"), "SECRET=value")
	writeTestFile(t, filepath.Join(outputDir, ".env.local"), "SECRET=value")
	writeTestFile(t, filepath.Join(outputDir, "assets", "app.css.map"), "{}")
	writeTestFile(t, filepath.Join(outputDir, "source", "home.page.gwdk"), "@page home")
	writeTestFile(t, filepath.Join(outputDir, "source", "main.go"), "package main")
	writeTestFile(t, filepath.Join(outputDir, "tmp", "asset.css"), "body{}")
	writeTestFile(t, filepath.Join(outputDir, "assets", "scratch.tmp"), "temporary")
	writeTestFile(t, filepath.Join(outputDir, "assets", "app.css"), "body{}")

	result, err := Generate(outputDir, appDir)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(result.Files, ",") != "assets/app.css,index.html" {
		t.Fatalf("unexpected embedded files: %#v", result.Files)
	}
	for _, path := range []string{
		filepath.Join(result.OutputDir, ".env"),
		filepath.Join(result.OutputDir, ".env.local"),
		filepath.Join(result.OutputDir, "assets", "app.css.map"),
		filepath.Join(result.OutputDir, "source", "home.page.gwdk"),
		filepath.Join(result.OutputDir, "source", "main.go"),
		filepath.Join(result.OutputDir, "tmp", "asset.css"),
		filepath.Join(result.OutputDir, "assets", "scratch.tmp"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected unsafe file %s to be skipped, stat err: %v", path, err)
		}
	}
}

func TestGenerateWritesActionRedirectHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:         "newsletter",
		ActionName:     "Subscribe",
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
	modulePayload, err := os.ReadFile(result.ModulePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modulePayload), "require github.com/cssbruno/gowdk") {
		t.Fatalf("expected generated action app to depend on GOWDK runtime module:\n%s", modulePayload)
	}
	source := string(payload)
	for _, expected := range []string{
		`Backend:  backend,`,
		`gowdkform "github.com/cssbruno/gowdk/runtime/form"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`gowdkvalidation "github.com/cssbruno/gowdk/runtime/validation"`,
		`func backend(response http.ResponseWriter, request *http.Request) bool`,
		`if request.Method == http.MethodPost && action(response, request)`,
		`func action(response http.ResponseWriter, request *http.Request) bool`,
		`case "/newsletter":`,
		`const maxActionBodyBytes int64 = 1 << 20`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`if err := request.ParseForm(); err != nil`,
		`http.StatusRequestEntityTooLarge`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, "invalid form")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusRequestEntityTooLarge, "request body too large")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusUnprocessableEntity, "validation failed")`,
		`requestPath := actionRequestPath(request.URL.Path)`,
		`func actionRequestPath(value string) string`,
		`type SubscribeInput struct`,
		`func decodeNewsletterSubscribeInput(values gowdkform.Values) (SubscribeInput, error)`,
		`gowdkform.DecodeExpected(values, gowdkform.Schema{Fields: []gowdkform.Field{{Name: "email"}}})`,
		`validation := gowdkvalidation.Result{}`,
		`values.HasSubmitted(field)`,
		`http.StatusUnprocessableEntity`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.RedirectTo("/newsletter?ok=1"))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWiresCSRFWhenEnabled(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), `<main><form method="post" action="/newsletter"><input name="email"></form></main>`)

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			Enabled:    true,
			SecretEnv:  "GOWDK_TEST_CSRF_SECRET",
			CookieName: "csrf",
			FieldName:  "_csrf",
			HeaderName: "X-CSRF",
			Insecure:   true,
		}}},
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Route:       "/newsletter",
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`errors`,
		`gowdkactions "github.com/cssbruno/gowdk/addons/actions"`,
		`CSRF:     csrfTokenSource,`,
		`csrfTokenSource, err := newCSRF()`,
		`csrfValidator = csrfTokenSource`,
		`var csrfValidator gowdkactions.CSRFValidator`,
		`func newCSRF() (*gowdkactions.CSRF, error)`,
		`secret := strings.TrimSpace(os.Getenv("GOWDK_TEST_CSRF_SECRET"))`,
		`return nil, errors.New("GOWDK_TEST_CSRF_SECRET is required when generated CSRF is enabled")`,
		`CookieName: "csrf"`,
		`FieldName: "_csrf"`,
		`HeaderName: "X-CSRF"`,
		`Insecure: true`,
		`if csrfValidator != nil {`,
		`err := csrfValidator.Validate(request)`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusForbidden, "invalid csrf token")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesTypedBoundActionHandlers(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "Login", "index.html"), "<main>Login</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{
		{
			PageID:      "Login",
			ActionName:  "Login",
			Route:       "/Login",
			InputFields: []string{"email"},
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Login",
				Signature:    manifest.BackendSignatureActionForm,
				InputType:    "LoginInput",
				InputFields: []manifest.BackendInputField{
					{FieldName: "Email", FormName: "email", Type: "string"},
					{FieldName: "Tags", FormName: "tag", Type: "[]string"},
					{FieldName: "Age", FormName: "age", Type: "int"},
					{FieldName: "Remember", FormName: "remember", Type: "bool"},
				},
			},
		},
		{
			PageID:      "Login",
			ActionName:  "save",
			Route:       "/Login/save",
			InputFields: []string{"email"},
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Save",
				Signature:    manifest.BackendSignatureActionFormPtr,
				InputType:    "LoginInput",
				InputPointer: true,
				InputFields: []manifest.BackendInputField{
					{FieldName: "Email", FormName: "email", Type: "string"},
				},
			},
		},
		{
			PageID:     "Login",
			ActionName: "Ping",
			Route:      "/Login/Ping",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Ping",
				Signature:    manifest.BackendSignatureAction0,
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`auth "example.com/app/auth"`,
		`func decodeLoginLoginBoundInput(values gowdkform.Values) (auth.LoginInput, error)`,
		`field0, ok, err := gowdkform.String(values, "email")`,
		`input.Email = field0`,
		`field1 := gowdkform.Strings(values, "tag")`,
		`input.Tags = field1`,
		`field2, ok, err := gowdkform.Int(values, "age", 0)`,
		`input.Age = int(field2)`,
		`field3, ok, err := gowdkform.Bool(values, "remember")`,
		`input.Remember = field3`,
		`input, err := decodeLoginLoginBoundInput(values)`,
		`ctx := gowdkruntime.WithEndpoint(request.Context(), gowdkruntime.EndpointMetadata{Kind: "action", PageID: "Login", Name: "Login", Method: "POST", Path: "/Login"})`,
		`result, err := auth.Login(ctx, input)`,
		`result, err := auth.Save(ctx, &input)`,
		`result, err := auth.Ping(ctx)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, "type LoginInput struct") {
		t.Fatalf("did not expect generated app to create a fake user input struct:\n%s", source)
	}
	if strings.Contains(source, "DecodeStruct") {
		t.Fatalf("did not expect generated app to use runtime reflection struct decoding:\n%s", source)
	}
}

func TestGenerateDoesNotImportMissingOrUnsupportedBackendPackages(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "home",
			ActionName: "Submit",
			Route:      "/",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingMissing,
				ImportPath:   "example.com/app/missing",
				PackageName:  "missing",
				FunctionName: "Submit",
				Message:      "GOWDK action handler missing.Submit is not implemented",
			},
		}},
		APIs: []APIEndpoint{{
			PageID:  "home",
			APIName: "Status",
			Method:  "GET",
			Route:   "/api/status",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingUnsupportedSignature,
				ImportPath:   "example.com/app/status",
				PackageName:  "status",
				FunctionName: "Status",
				Message:      "GOWDK API handler status.Status must have signature func(context.Context, *http.Request) (response.Response, error)",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{
		`"example.com/app/missing"`,
		`"example.com/app/status"`,
		`missing.Submit(`,
		`status.Status(`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("did not expect generated missing-handler source to contain %q:\n%s", forbidden, source)
		}
	}
	for _, expected := range []string{
		`GOWDK action handler missing.Submit is not implemented`,
		`GOWDK API handler status.Status must have signature`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated 501 source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateSortsImportsAndBackendDispatchDeterministically(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{
			{
				PageID:     "z",
				ActionName: "Zed",
				Route:      "/z",
				Binding: manifest.BackendBinding{
					Status:       manifest.BackendBindingBound,
					ImportPath:   "example.com/app/beta",
					PackageName:  "beta",
					FunctionName: "Zed",
					Signature:    manifest.BackendSignatureAction0,
				},
			},
			{
				PageID:     "a",
				ActionName: "Alpha",
				Route:      "/a",
				Binding: manifest.BackendBinding{
					Status:       manifest.BackendBindingBound,
					ImportPath:   "example.com/app/alpha",
					PackageName:  "alpha",
					FunctionName: "Alpha",
					Signature:    manifest.BackendSignatureAction0,
				},
			},
		},
		APIs: []APIEndpoint{
			{
				PageID:  "z",
				APIName: "ZedAPI",
				Method:  http.MethodGet,
				Route:   "/api/z",
				Binding: manifest.BackendBinding{
					Status:       manifest.BackendBindingBound,
					ImportPath:   "example.com/app/beta",
					PackageName:  "beta",
					FunctionName: "ZedAPI",
					Signature:    manifest.BackendSignatureAPI,
				},
			},
			{
				PageID:  "a",
				APIName: "AlphaAPI",
				Method:  http.MethodGet,
				Route:   "/api/a",
				Binding: manifest.BackendBinding{
					Status:       manifest.BackendBindingBound,
					ImportPath:   "example.com/app/alpha",
					PackageName:  "alpha",
					FunctionName: "AlphaAPI",
					Signature:    manifest.BackendSignatureAPI,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	assertSourceOrder(t, source,
		`alpha "example.com/app/alpha"`,
		`beta "example.com/app/beta"`,
		`gowdkruntime "github.com/cssbruno/gowdk/runtime/app"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`"path"`,
	)
	assertSourceOrder(t, source, `case "/a":`, `case "/z":`)
	assertSourceOrder(t, source, `requestPath == "/api/a"`, `requestPath == "/api/z"`)
}

func TestActionSourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_actions.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("action source emitter must use go/ast, found %q in source_actions.go", forbidden)
		}
	}
}

func TestAPISourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_api.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("API source emitter must use go/ast, found %q in source_api.go", forbidden)
		}
	}
}

func TestBackendSourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_backend.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("backend source emitter must use go/ast, found %q in source_backend.go", forbidden)
		}
	}
}

func TestSSRSourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_ssr.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("SSR source emitter must use go/ast, found %q in source_ssr.go", forbidden)
		}
	}
}

func TestGenerateWritesBoundAPIHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "status", "index.html"), "<main>Status</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{APIs: []APIEndpoint{{
		PageID:  "status",
		APIName: "Health",
		Method:  http.MethodGet,
		Route:   "/api/health",
		Binding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "example.com/app/status",
			PackageName:  "status",
			FunctionName: "Health",
			Signature:    manifest.BackendSignatureAPI,
		},
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
		`status "example.com/app/status"`,
		`func api(response http.ResponseWriter, request *http.Request) bool`,
		`requestPath := path.Clean("/" + request.URL.Path)`,
		`case request.Method == "GET" && requestPath == "/api/health":`,
		`ctx := gowdkruntime.WithEndpoint(request.Context(), gowdkruntime.EndpointMetadata{Kind: "api", PageID: "status", Name: "Health", Method: "GET", Path: "/api/health"})`,
		`result, err := status.Health(ctx, request)`,
		`gowdkresponse.WriteNoStoreError(response, gowdkresponse.HandlerStatus(err, http.StatusInternalServerError), err.Error())`,
		`gowdkresponse.WriteNoStoreHTTP(response, result)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesActionFragmentHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:      "patients",
		ActionName:  "Refresh",
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
		`gowdkform "github.com/cssbruno/gowdk/runtime/form"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`partial := strings.TrimSpace(request.Header.Get("X-GOWDK-Partial"))`,
		`fragment := gowdkresponse.Response{Kind: gowdkresponse.Fragment, Status: http.StatusOK, Target: "#patients", Body: "<section><p>Updated patients</p></section>"}`,
		`gowdkresponse.FragmentSwap(fragment.Target, gowdkresponse.SwapMode(swap), fragment.Body)`,
		`gowdkresponse.WriteNoStoreHTTP(response, fragment)`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusNotFound, "partial fragment not found")`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.RedirectTo("/patients"))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesSSRHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
		`SSRExact:   ssrExact,`,
		`SSRDynamic: ssrDynamic,`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`func ssrExact(response http.ResponseWriter, request *http.Request) bool`,
		`func ssrDynamic(response http.ResponseWriter, request *http.Request) bool`,
		`case "/dashboard":`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard"})`,
		`request = request.WithContext(ctx)`,
		`gowdkresponse.WriteNoStoreHTML(response, request, "<main><h1>Dashboard</h1></main>")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesDynamicSSRHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
		`gowdkhtml "github.com/cssbruno/gowdk/runtime/html"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`gowdkroute "github.com/cssbruno/gowdk/runtime/route"`,
		`gowdkroute.Match("/blog/{slug}", request.URL.Path)`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "blog.post", Method: "GET", Path: "/blog/{slug}"})`,
		`ctx = gowdkruntime.WithParams(ctx, params)`,
		`request = request.WithContext(ctx)`,
		`strings.ReplaceAll(html, "__SLUG__", gowdkhtml.Escape(params["slug"]))`,
		`gowdkresponse.WriteNoStoreHTML(response, request, html)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesDynamicSSRHandlerWithoutReplacements(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
	if !strings.Contains(source, `gowdkroute.Match("/blog/{slug}", request.URL.Path)`) {
		t.Fatalf("expected generated main.go to match dynamic route:\n%s", source)
	}
	if !strings.Contains(source, `params, ok := gowdkroute.Match`) {
		t.Fatalf("expected generated main.go to keep route params in request context:\n%s", source)
	}
	if !strings.Contains(source, `ctx = gowdkruntime.WithParams(ctx, params)`) {
		t.Fatalf("expected generated main.go to attach dynamic route params:\n%s", source)
	}
	if strings.Contains(source, `case "/blog/{slug}":`) {
		t.Fatalf("expected generated main.go not to use exact literal match for dynamic route:\n%s", source)
	}
}

func TestGenerateAutoDetectsActionAndSSRRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	app := manifest.Manifest{Pages: []manifest.Page{
		{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<form g:post={Subscribe}><input name="email" required /></form>`,
				Actions: []manifest.Action{{
					Name:           "Subscribe",
					InputName:      "input",
					InputType:      "SubscribeInput",
					ValidatesInput: true,
					Redirect:       "/newsletter?ok=1",
				}},
			},
		},
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><h1>Dashboard</h1></main>`,
			},
		},
	}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		AutoRoutes: true,
		Config: gowdk.Config{
			Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
		},
		Manifest: &app,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`case "/newsletter":`,
		`func decodeNewsletterSubscribeInput(values gowdkform.Values) (SubscribeInput, error)`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.RedirectTo("/newsletter?ok=1"))`,
		`case "/dashboard":`,
		`<main><h1>Dashboard</h1></main>`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected auto-detected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateAutoRoutesRequiresManifest(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{AutoRoutes: true})
	if err == nil || !strings.Contains(err.Error(), "auto route detection requires a parsed manifest") {
		t.Fatalf("expected auto route manifest error, got %v", err)
	}
}

func TestActionEndpointsInfersInputFieldsFromGPostForm(t *testing.T) {
	routes, err := ActionEndpoints(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={Subscribe}><input name="email" required /><textarea name="note"></textarea></form>`,
			Actions: []manifest.Action{{
				Name:           "Subscribe",
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

func TestActionEndpointsInfersSubmitIntentFieldsFromGPostForm(t *testing.T) {
	routes, err := ActionEndpoints(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={Subscribe}><input name="email" /><button name="intent" value="save">Save</button><button type="button" name="local">Local</button></form>`,
			Actions: []manifest.Action{{
				Name:   "Subscribe",
				Method: "POST",
				Route:  "/newsletter",
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected one action endpoint, got %#v", routes)
	}
	if strings.Join(routes[0].InputFields, ",") != "email,intent" {
		t.Fatalf("unexpected fields: %#v", routes[0].InputFields)
	}
}

func TestActionEndpointsRendersActionFragments(t *testing.T) {
	routes, err := ActionEndpoints(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={Refresh} g:target="#patients"><input name="query" /></form><section id="patients"></section>`,
			Actions: []manifest.Action{{
				Name:      "Refresh",
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

func TestActionEndpointsRejectsFileInputsWithPageContext(t *testing.T) {
	_, err := ActionEndpoints(manifest.Manifest{Pages: []manifest.Page{{
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

func TestGenerateRejectsAppDirInsideSPAOutput(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := Generate(outputDir, filepath.Join(outputDir, "app"))
	if err == nil {
		t.Fatal("expected app directory validation error")
	}
	if !strings.Contains(err.Error(), "must be outside build output directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsSPAOutputInsideGeneratedSPADir(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	outputDir := filepath.Join(appDir, "gowdkapp", "app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := Generate(outputDir, appDir)
	if err == nil {
		t.Fatal("expected generated app output directory validation error")
	}
	if !strings.Contains(err.Error(), "must not be inside generated app output directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsUnsafeActionRedirect(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:     "newsletter",
		ActionName: "Subscribe",
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

func TestGenerateRejectsDynamicActionEndpoint(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "blog", "hello", "index.html"), "<main>Post</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:     "blog.post",
		ActionName: "save",
		Route:      "/blog/{slug}",
		Redirect:   "/blog/hello",
	}}})
	if err == nil {
		t.Fatal("expected dynamic route error")
	}
	if !strings.Contains(err.Error(), `endpoint path "/blog/{slug}" must be a concrete path`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateRejectsSSRReplacementForUndeclaredParam(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	if _, err := Generate(outputDir, appDir); err != nil {
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	wasmPath := filepath.Join(root, "site.wasm")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{"version":1,"files":{}}`)

	if _, err := Generate(outputDir, appDir); err != nil {
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

func TestGeneratedBinaryServesEmbeddedSPAHTML(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "blog", "hello", "index.html"), "<main>Post</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)

	if _, err := Generate(outputDir, appDir); err != nil {
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

func TestGeneratedBinaryServesSSRRouteBeforeSPAFallback(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "dashboard", "index.html"), "<main>Stale app dashboard</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
	if strings.Contains(body, "Stale app dashboard") {
		t.Fatalf("expected SSR route to win over app fallback, got %s", body)
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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

func TestGeneratedBinaryServesAppPageBeforeDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "blog", "about", "index.html"), "<main>App about</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
	if strings.TrimSpace(body) != "<main>App about</main>" {
		t.Fatalf("expected app page to win over dynamic SSR route, got: %s", body)
	}
	if headers.Get("Cache-Control") == "no-store" {
		t.Fatalf("expected app response headers, got SSR cache header")
	}
}

func TestGeneratedBinaryServesSPAAssetBeforeRootDynamicSSRRoute(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "favicon.ico"), "ICON")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
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
		t.Fatalf("expected app asset to win over root dynamic SSR route, got: %s", body)
	}
	if headers.Get("Cache-Control") == "no-store" {
		t.Fatalf("expected SPA response headers, got SSR cache header")
	}
}

func TestGeneratedBinaryAutoGeneratesInstanceID(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := Generate(outputDir, appDir); err != nil {
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
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:         "newsletter",
		ActionName:     "Subscribe",
		Route:          "/newsletter",
		InputName:      "input",
		InputType:      "SubscribeInput",
		InputFields:    []string{"email"},
		RequiredFields: []string{"email"},
		ValidatesInput: true,
		Redirect:       "/newsletter?ok=1",
	}, {
		PageID:     "newsletter",
		ActionName: "Ping",
		Route:      "/newsletter/ping",
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
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on action redirect, got %q", cacheControl)
	}

	response, err = waitForHTTPStatus("http://"+addr+"/newsletter/", http.MethodPost, "email=reader%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected trailing slash POST to return 303, got %d", response.StatusCode)
	}
	if response.Header.Get("Location") != "/newsletter?ok=1" {
		t.Fatalf("unexpected redirect location for trailing slash POST: %q", response.Header.Get("Location"))
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on trailing slash action redirect, got %q", cacheControl)
	}

	response, err = waitForHTTPStatus("http://"+addr+"/newsletter/ping", http.MethodPost, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("expected no-content action to return 204, got %d", response.StatusCode)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on no-content action response, got %q", cacheControl)
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

func TestGeneratedBinaryValidatesCSRFWhenEnabled(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), `<main><form method="post" action="/newsletter"><input name="email"></form></main>`)

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			Enabled:   true,
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Route:       "/newsletter",
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(),
		"GOWDK_ADDR="+addr,
		"GOWDK_TEST_CSRF_SECRET="+strings.Repeat("s", 32),
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, headers, err := waitForHTTPResponse("http://" + addr + "/newsletter")
	if err != nil {
		t.Fatal(err)
	}
	token := hiddenInputValue(body, "_gowdk_csrf")
	if token == "" {
		t.Fatalf("expected generated form csrf token, got %s", body)
	}
	cookie := cookieHeader(headers.Get("Set-Cookie"))
	if !strings.Contains(cookie, "__Host-gowdk-csrf=") {
		t.Fatalf("expected csrf cookie, got %q", headers.Get("Set-Cookie"))
	}

	response, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected missing csrf token to return 403, got %d: %s", response.StatusCode, payload)
	}
	if !strings.Contains(string(payload), "invalid csrf token") {
		t.Fatalf("expected invalid csrf body, got %s", payload)
	}
	if cache := response.Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store on invalid csrf response, got %q", cache)
	}

	response, err = waitForHTTPStatusWithHeaders("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com", map[string]string{
		"Cookie":       cookie,
		"X-GOWDK-CSRF": token,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected valid csrf POST to return 303, got %d", response.StatusCode)
	}
	if response.Header.Get("Location") != "/newsletter?ok=1" {
		t.Fatalf("unexpected redirect location: %q", response.Header.Get("Location"))
	}
}

func TestGeneratedBinaryServesPartialActionFragment(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:      "patients",
		ActionName:  "Refresh",
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
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on partial fragment response, got %q", cacheControl)
	}
}

func hiddenInputValue(markup string, name string) string {
	marker := `name="` + name + `" value="`
	start := strings.Index(markup, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(markup[start:], `"`)
	if end < 0 {
		return ""
	}
	return markup[start : start+end]
}

func cookieHeader(setCookie string) string {
	if index := strings.Index(setCookie, ";"); index >= 0 {
		return setCookie[:index]
	}
	return setCookie
}

func TestGeneratedBinaryAcknowledgesCookieNotice(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), `<main>Home</main><form data-cookie-notice method="post" action="/_gowdk/cookie-ack"></form>`)

	if _, err := GenerateWithOptions(outputDir, appDir, Options{}); err != nil {
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

	response, err := waitForHTTPStatusWithHeaders("http://"+addr+"/_gowdk/cookie-ack", http.MethodPost, "", map[string]string{
		"Referer": "http://" + addr + "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", response.StatusCode)
	}
	if response.Header.Get("Location") != "/" {
		t.Fatalf("unexpected redirect location: %q", response.Header.Get("Location"))
	}
	if setCookie := response.Header.Get("Set-Cookie"); !strings.Contains(setCookie, "gowdk_cookie_ack=accepted") {
		t.Fatalf("expected acknowledgement cookie, got %q", setCookie)
	}

	response, err = waitForHTTPStatusWithHeaders("http://"+addr+"/", http.MethodGet, "", map[string]string{
		"Cookie": "gowdk_cookie_ack=accepted",
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
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if !strings.Contains(string(payload), "data-cookie-notice hidden") {
		t.Fatalf("expected hidden cookie notice, got %s", payload)
	}
}

func TestGeneratedBinaryDoesNotValidateRequiredFieldsWithoutValidMetadata(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:         "newsletter",
		ActionName:     "Subscribe",
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

func assertSourceOrder(t *testing.T, source string, snippets ...string) {
	t.Helper()
	previous := -1
	for _, snippet := range snippets {
		index := strings.Index(source, snippet)
		if index < 0 {
			t.Fatalf("expected generated source to contain %q:\n%s", snippet, source)
		}
		if index <= previous {
			t.Fatalf("expected %q to appear after previous snippets:\n%s", snippet, source)
		}
		previous = index
	}
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
