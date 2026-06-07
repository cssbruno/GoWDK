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
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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
		`Assets: gowdkruntime.LoadAssetManifest(root),`,
		`ErrorPages: gowdkruntime.LoadErrorPages(root),`,
		`Backend: backend,`,
		`SSRExact: ssrExact,`,
		`SSRDynamic: ssrDynamic}`,
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
		PageID:           "newsletter",
		ActionName:       "Subscribe",
		Route:            "/newsletter",
		InputName:        "input",
		InputType:        "SubscribeInput",
		InputFields:      []string{"email"},
		RequiredFields:   []string{"email"},
		RequiredMessages: map[string]string{"email": "Email is required"},
		ValidationRules: []ActionValidationRule{{
			Field:          "email",
			MinLength:      5,
			MaxLength:      80,
			Pattern:        `[a-z]+@[a-z]+[.][a-z]{2,4}`,
			PatternMessage: "Use a real email address",
		}},
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
		`backendRouter, err := newBackendRouter()`,
		`Backend: backendRouter.HandlerFunc(),`,
		`gowdkform "github.com/cssbruno/gowdk/runtime/form"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`gowdkvalidation "github.com/cssbruno/gowdk/runtime/validation"`,
		`"regexp"`,
		`"unicode/utf8"`,
		`func newBackendRouter() (*gowdkruntime.BackendRouter, error)`,
		`gowdkruntime.BackendRoute{Method: http.MethodPost, Path: "/newsletter", Kind: "action", Handler: action}`,
		`func action(response http.ResponseWriter, request *http.Request) bool`,
		`case "/newsletter":`,
		`const maxActionBodyBytes int64 = 1 << 20`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`if err := request.ParseForm(); err != nil`,
		`http.StatusRequestEntityTooLarge`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, "invalid form")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusRequestEntityTooLarge, "request body too large")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusUnprocessableEntity, "validation failed")`,
		`validationTarget := strings.TrimSpace(request.Header.Get("X-GOWDK-Target"))`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.ValidationFragment(validationTarget, validation))`,
		`requestPath := actionRequestPath(request.URL.Path)`,
		`func actionRequestPath(value string) string`,
		`type SubscribeInput struct`,
		`func decodeNewsletterSubscribeInput(values gowdkform.Values) (SubscribeInput, error)`,
		`gowdkform.DecodeExpected(values, gowdkform.Schema{Fields: []gowdkform.Field{{Name: "email"}}})`,
		`validation := gowdkvalidation.Result{}`,
		`values.HasSubmitted("email")`,
		`validation.Add("email", "Email is required")`,
		`utf8.RuneCountInString(value) < 5`,
		`utf8.RuneCountInString(value) > 80`,
		`regexp.MatchString("^(?:[a-z]+@[a-z]+[.][a-z]{2,4})$", value)`,
		`validation.Add("email", "Use a real email address")`,
		`http.StatusUnprocessableEntity`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.RedirectTo("/newsletter?ok=1"))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesBoundContractBackendRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{
		{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []manifest.BackendInputField{
				{FieldName: "Name", FormName: "name", Type: "string"},
				{FieldName: "Tags", FormName: "tag", Type: "[]string"},
				{FieldName: "Age", FormName: "age", Type: "int"},
				{FieldName: "Remember", FormName: "remember", Type: "bool"},
			},
			Method:    "POST",
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "HandleCreatePatient",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
		},
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			InputFields: []manifest.BackendInputField{
				{FieldName: "Filter", FormName: "filter", Type: "string"},
			},
			Method:    "GET",
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "LoadPatientPage",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
		},
	}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{IR: program})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`gowdkcontracts "github.com/cssbruno/gowdk/runtime/contracts"`,
		`patients "example.com/app/contracts/patients"`,
		`contractRegistry := gowdkcontracts.NewRegistry()`,
		`patients.Register(contractRegistry)`,
		`Kind: "command", Handler: commandPatientsCreatePatientPOSTPatients(contractRegistry)`,
		`Kind: "query", Handler: queryPatientsGetPatientPageGETPatients(contractRegistry)`,
		`func commandPatientsCreatePatientPOSTPatients(contractRegistry *gowdkcontracts.Registry) gowdkruntime.BackendHandler`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`values := gowdkform.FromURLValues(request.PostForm)`,
		`input, err := decodeContractPatientsCreatePatientInput(values)`,
		`gowdkcontracts.ExecuteCommandForRole[patients.CreatePatient, patients.CreatePatientResult]`,
		`func decodeContractPatientsCreatePatientInput(values gowdkform.Values) (patients.CreatePatient, error)`,
		`gowdkform.DecodeExpected(values, gowdkform.Schema{Fields: []gowdkform.Field{{Name: "name"}, {Name: "tag"}, {Name: "age"}, {Name: "remember"}}})`,
		`input.Name = field0`,
		`input.Tags = field1`,
		`input.Age = int(field2)`,
		`input.Remember = field3`,
		`func queryPatientsGetPatientPageGETPatients(contractRegistry *gowdkcontracts.Registry) gowdkruntime.BackendHandler`,
		`values := gowdkform.FromURLValues(request.URL.Query())`,
		`input, err := decodeContractPatientsGetPatientPageInput(values)`,
		`gowdkcontracts.ExecuteQueryForRole[patients.GetPatientPage, patients.PatientPageData]`,
		`func decodeContractPatientsGetPatientPageInput(values gowdkform.Values) (patients.GetPatientPage, error)`,
		`input.Filter = field0`,
		`gowdkresponse.JSONValue(http.StatusOK, result)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated contract app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateBackendAppRegistersBackendRoutes(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "generated-backend")

	result, err := GenerateBackendWithOptions(appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Route:      "/newsletter",
			Redirect:   "/newsletter?ok=1",
		}},
		Fragments: []FragmentEndpoint{{
			PageID:       "patients",
			FragmentName: "List",
			Method:       "GET",
			Route:        "/patients/list",
			Target:       "#patients",
			HTML:         "<section>Patients</section>",
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
		`gowdkruntime "github.com/cssbruno/gowdk/runtime/app"`,
		`backendRouter, err := newBackendRouter()`,
		`mux.Handle("/", backendRouter)`,
		`func newBackendRouter() (*gowdkruntime.BackendRouter, error)`,
		`gowdkruntime.BackendRoute{Method: http.MethodPost, Path: "/newsletter", Kind: "action", Handler: action}`,
		`gowdkruntime.BackendRoute{Method: http.MethodGet, Path: "/patients/list", Kind: "fragment", Handler: fragment}`,
		`func fragment(response http.ResponseWriter, request *http.Request) bool`,
		`gowdkresponse.FragmentFor("#patients", "<section>Patients</section>")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated backend app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `func backend(response http.ResponseWriter, request *http.Request) bool`) {
		t.Fatalf("expected backend-only app to use BackendRouter instead of generated backend dispatcher:\n%s", source)
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
		`CSRF: csrfTokenSource,`,
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
			Redirect:    "/dashboard",
			Fragments:   []ActionFragment{{Target: "#login", HTML: "<p>ignored</p>"}},
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
		`ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "action", PageID: "Login", Name: "Login", Method: "POST", Path: "/Login"})`,
		`result, err := auth.Login(ctx, input)`,
		`result, err := auth.Save(ctx, &input)`,
		`result, err := auth.Ping(ctx)`,
		`gowdkresponse.WriteNoStoreHTTP(response, result)`,
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
	if strings.Contains(source, `gowdkresponse.RedirectTo("/dashboard")`) || strings.Contains(source, `<p>ignored</p>`) {
		t.Fatalf("bound action must keep redirect and fragment policy in the user Go response:\n%s", source)
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

func TestContractSourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_contracts.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("contract source emitter must use go/ast, found %q in source_contracts.go", forbidden)
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

func TestRateLimitSourceEmitterDoesNotUseStringLineWriting(t *testing.T) {
	payload, err := os.ReadFile("source_rate_limit.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, forbidden := range []string{"WriteString", "strings.Builder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("rate-limit source emitter must use go/ast, found %q in source_rate_limit.go", forbidden)
		}
	}
}

func TestAppShellSourceEmitterDoesNotUseRawTemplates(t *testing.T) {
	for _, path := range []string{"source.go", "source_backend_app.go", "template.go"} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(payload)
		for _, forbidden := range []string{
			"appPackageSourceTemplate",
			"backendAppPackageSourceTemplate",
			"strings.ReplaceAll",
			"Temporary generated-Go template exception",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("app shell source emitter must use go/ast, found %q in %s", forbidden, path)
			}
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
		`ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "api", PageID: "status", Name: "Health", Method: "GET", Path: "/api/health"})`,
		`result, err := status.Health(ctx, request)`,
		`gowdkresponse.WriteNoStoreError(response, gowdkresponse.HandlerStatus(err, http.StatusInternalServerError), err.Error())`,
		`gowdkresponse.WriteNoStoreHTTP(response, result)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesEndpointErrorPages(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "subscribe.html"), "<main>Subscribe Error</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "health.html"), "<main>Health Error</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Route:      "/newsletter",
			ErrorPage:  "/errors/subscribe.html",
		}},
		APIs: []APIEndpoint{{
			PageID:    "status",
			APIName:   "Health",
			Method:    http.MethodGet,
			Route:     "/api/health",
			ErrorPage: "/errors/health.html",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/status",
				PackageName:  "status",
				FunctionName: "Health",
				Signature:    manifest.BackendSignatureAPI,
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
	for _, expected := range []string{
		`ErrorPages: gowdkruntime.LoadErrorPagesWith(root, gowdkruntime.ErrorPage{Path: "errors/health.html"}, gowdkruntime.ErrorPage{Path: "errors/subscribe.html"})`,
		`func action(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`func api(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`gowdkruntime.EndpointMetadata{Kind: "action", PageID: "newsletter", Name: "Subscribe", Method: "POST", Path: "/newsletter", ErrorPage: "errors/subscribe.html"}`,
		`gowdkruntime.EndpointMetadata{Kind: "api", PageID: "status", Name: "Health", Method: "GET", Path: "/api/health", ErrorPage: "errors/health.html"}`,
		`gowdkruntime.RecoverEndpointPanic(response, request, recovered)`,
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
		`SSRExact: ssrExact,`,
		`SSRDynamic: ssrDynamic}`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`func ssrExact(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`func ssrDynamic(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`case "/dashboard":`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr"})`,
		`request = request.WithContext(ctx)`,
		`if recovered := recover(); recovered != nil {`,
		`handled = true`,
		`gowdkruntime.RecoverSSRRoutePanic(response, request, recovered)`,
		`html := "<main><h1>Dashboard</h1></main>"`,
		`gowdkresponse.WriteNoStoreHTML(response, request, html)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesSSRCachePolicy(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "docs",
		Route:  "/docs",
		Cache:  "public, max-age=60",
		HTML:   "<main><h1>Docs</h1></main>",
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
		`Cache: "public, max-age=60"`,
		`gowdkresponse.WriteHTML(response, request, html, "public, max-age=60")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesSSRLoadHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "dashboard.html"), "<main>Dashboard Error</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:    "dashboard",
		Route:     "/dashboard",
		ErrorPage: "/errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "example.com/app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    manifest.BackendSignatureLoadError,
		},
		HTML: `<main><h1>__USER__</h1></main>`,
		LoadReplacements: []SSRLoadReplacement{{
			Path:        "user.name",
			Placeholder: "__USER__",
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
		`dashboard "example.com/app/dashboard"`,
		`gowdkssr "github.com/cssbruno/gowdk/addons/ssr"`,
		`"fmt"`,
		`ErrorPages: gowdkruntime.LoadErrorPagesWith(root, gowdkruntime.ErrorPage{Path: "errors/dashboard.html"})`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr", ErrorPage: "errors/dashboard.html", HasLoad: true})`,
		`gowdkruntime.RecoverSSRRoutePanic(response, request, recovered)`,
		`loadContext := gowdkssr.NewLoadContext(request, nil)`,
		`loadData, err := dashboard.LoadDashboard(loadContext)`,
		`redirectURL, redirectStatus, ok := gowdkssr.RedirectTarget(err)`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.Response{Kind: gowdkresponse.Redirect, Status: redirectStatus, URL: redirectURL})`,
		`gowdkruntime.WriteErrorPage(response, request, http.StatusInternalServerError, err.Error())`,
		`loadValue0, loadOK0 := gowdkssr.LoadPath(loadData, "user.name")`,
		`gowdkruntime.WriteErrorPage(response, request, http.StatusInternalServerError, "missing load field user.name")`,
		`strings.ReplaceAll(html, "__USER__", gowdkhtml.Escape(fmt.Sprint(loadValue0)))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateKeepsActionHandlersIndependentFromSSRLoad(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		SSR: []SSRRoute{{
			PageID:  "dashboard",
			Route:   "/dashboard",
			HasLoad: true,
			LoadBinding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "LoadDashboard",
				Signature:    manifest.BackendSignatureLoadError,
			},
			HTML: `<main><h1>__USER__</h1></main>`,
			LoadReplacements: []SSRLoadReplacement{{
				Path:        "user.name",
				Placeholder: "__USER__",
			}},
		}},
		Actions: []ActionEndpoint{{
			PageID:     "dashboard",
			ActionName: "Save",
			Method:     http.MethodPost,
			Route:      "/dashboard",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "Save",
				Signature:    manifest.BackendSignatureAction0,
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
	if count := strings.Count(source, "LoadDashboard(loadContext)"); count != 1 {
		t.Fatalf("expected SSR load to be called only by the SSR handler, got %d calls:\n%s", count, source)
	}
	actionSource := generatedFunctionSource(t, source, "action")
	for _, forbidden := range []string{"LoadDashboard(loadContext)", "gowdkssr.NewLoadContext", "LoadPath(loadData"} {
		if strings.Contains(actionSource, forbidden) {
			t.Fatalf("action handler must not rerun SSR load via %q:\n%s", forbidden, actionSource)
		}
	}
	if !strings.Contains(actionSource, "Save(ctx)") {
		t.Fatalf("expected action handler to call the action binding:\n%s", actionSource)
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
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "blog.post", Method: "GET", Path: "/blog/{slug}", Render: "ssr", DynamicParams: []string{"slug"}})`,
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

func TestGenerateWritesTypedSSRRouteParamBindings(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:        "patients.show",
		Route:         "/patients/{id}",
		DynamicParams: []string{"id"},
		RouteParams:   []manifest.RouteParam{{Name: "id", Type: "int"}},
		HTML:          `<main>Patient</main>`,
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
		`RouteParams: []gowdkruntime.RouteParamMetadata{gowdkruntime.RouteParamMetadata{Name: "id", Type: "int"}}`,
		`typedParams := map[string]any{}`,
		`paramValue0, paramOK0, paramErr0 := gowdkroute.Int(params, "id")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, "invalid route parameter id")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusNotFound, "missing route parameter id")`,
		`typedParams["id"] = paramValue0`,
		`ctx = gowdkruntime.WithTypedParams(ctx, typedParams)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
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
				Fragments: []manifest.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/newsletter/list",
					Target: "#newsletter",
					Body:   "<section>Newsletter list</section>",
				}},
			},
		},
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Guard:  []string{"auth.required"},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><h1>Dashboard</h1></main>`,
			},
		},
	}}

	config := gowdk.Config{
		Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
	}
	ir := gwdkanalysis.BuildIR(config, app)
	result, err := GenerateWithOptions(outputDir, appDir, Options{
		AutoRoutes: true,
		Config:     config,
		IR:         &ir,
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
		`case request.Method == "GET" && requestPath == "/newsletter/list":`,
		`gowdkresponse.FragmentFor("#newsletter", "<section>Newsletter list</section>")`,
		`case "/dashboard":`,
		`gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr", Guards: []string{"auth.required"}}`,
		`<main><h1>Dashboard</h1></main>`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected auto-detected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesGuardRegistryAndGuardChecks(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
			Guards:     []string{"auth.required"},
			Redirect:   "/newsletter?ok=1",
		}},
		APIs: []APIEndpoint{{
			PageID:  "session",
			APIName: "Session",
			Method:  "GET",
			Route:   "/api/session",
			Guards:  []string{"auth.required"},
		}},
		SSR: []SSRRoute{{
			PageID: "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required"},
			HTML:   "<main>Dashboard</main>",
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
		`gowdkssr "github.com/cssbruno/gowdk/addons/ssr"`,
		`var guardRegistry gowdkssr.GuardRegistry`,
		`func RegisterGuards(registry gowdkssr.GuardRegistry)`,
		`gowdkssr.RunGuards(loadContext, guards, guardRegistry)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected guard generated source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWiresRateLimiterWhenEnabled(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ratelimit", gowdk.FeatureRateLimit)}},
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
			Guards:     []string{"auth.required"},
			Redirect:   "/newsletter?ok=1",
		}},
		APIs: []APIEndpoint{{
			PageID:    "session",
			APIName:   "Session",
			Method:    "GET",
			Route:     "/api/session",
			Guards:    []string{"auth.required"},
			ErrorPage: "/errors/api.html",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/session",
				PackageName:  "session",
				FunctionName: "Session",
				Signature:    manifest.BackendSignatureAPI,
			},
		}},
		Fragments: []FragmentEndpoint{{
			PageID:       "patients",
			FragmentName: "List",
			Method:       "GET",
			Route:        "/patients/list",
			Target:       "#patients",
			HTML:         "<section>Patients</section>",
			Guards:       []string{"auth.required"},
		}},
		SSR: []SSRRoute{{
			PageID:    "dashboard",
			Route:     "/dashboard",
			Guards:    []string{"auth.required"},
			HasLoad:   true,
			ErrorPage: "/errors/dashboard.html",
			LoadBinding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "LoadDashboard",
				Signature:    manifest.BackendSignatureLoadError,
			},
			HTML: "<main>Dashboard</main>",
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
		`gowdkratelimit "github.com/cssbruno/gowdk/addons/ratelimit"`,
		`var rateLimiter *gowdkratelimit.Limiter`,
		`func RegisterRateLimiter(limiter *gowdkratelimit.Limiter)`,
		`result, err := rateLimiter.AllowRequest(request)`,
		`gowdkratelimit.WriteHeaders(response, result)`,
		`gowdkratelimit.DefaultLimitHandler(response, request, result)`,
		`if runRateLimit(response, request)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected rate-limit generated source to contain %q:\n%s", expected, source)
		}
	}
	assertSourceOrder(t, source,
		`ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "action"`,
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
	)
	apiIndex := strings.Index(source, `ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "api"`)
	if apiIndex < 0 {
		t.Fatalf("expected generated source to contain API endpoint context:\n%s", source)
	}
	assertSourceOrder(t, source[apiIndex:],
		`defer func()`,
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
		`result, err := session.Session(ctx, request)`,
	)
	fragmentIndex := strings.Index(source, `ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "fragment"`)
	if fragmentIndex < 0 {
		t.Fatalf("expected generated source to contain fragment endpoint context:\n%s", source)
	}
	assertSourceOrder(t, source[fragmentIndex:],
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
		`fragment := gowdkresponse.FragmentFor("#patients", "<section>Patients</section>")`,
	)
	ssrIndex := strings.Index(source, `ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr"`)
	if ssrIndex < 0 {
		t.Fatalf("expected generated source to contain SSR route context:\n%s", source)
	}
	assertSourceOrder(t, source[ssrIndex:],
		`defer func()`,
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
		`html := "<main>Dashboard</main>"`,
		`loadContext := gowdkssr.NewLoadContext(request, nil)`,
		`loadData, err := dashboard.LoadDashboard(loadContext)`,
	)
}

func TestGenerateAutoRoutesRequiresIR(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{AutoRoutes: true})
	if err == nil || !strings.Contains(err.Error(), "auto route detection requires compiler IR") {
		t.Fatalf("expected auto route IR error, got %v", err)
	}
}

func TestActionEndpointsInfersInputFieldsFromGPostForm(t *testing.T) {
	routes, err := ActionEndpoints(manifest.Manifest{Pages: []manifest.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: manifest.Blocks{
			ViewBody: `<form g:post={Subscribe}><input name="email" required minlength="5" maxlength="80" pattern="[a-z]+@[a-z]+[.][a-z]{2,4}" g:message:required="Email is required" g:message:pattern="Use a real email" /><textarea name="note"></textarea></form>`,
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
	if routes[0].RequiredMessages["email"] != "Email is required" {
		t.Fatalf("unexpected required messages: %#v", routes[0].RequiredMessages)
	}
	if len(routes[0].ValidationRules) != 1 ||
		routes[0].ValidationRules[0].Field != "email" ||
		routes[0].ValidationRules[0].MinLength != 5 ||
		routes[0].ValidationRules[0].MaxLength != 80 ||
		routes[0].ValidationRules[0].Pattern != `[a-z]+@[a-z]+[.][a-z]{2,4}` ||
		routes[0].ValidationRules[0].PatternMessage != "Use a real email" {
		t.Fatalf("unexpected validation rules: %#v", routes[0].ValidationRules)
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

func TestFragmentEndpointsRenderComponents(t *testing.T) {
	routes, err := FragmentEndpoints(manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "patients",
			Route:   "/patients",
			Package: "pages",
			Uses:    []manifest.Use{{Alias: "ui", Package: "components"}},
			Blocks: manifest.Blocks{
				Fragments: []manifest.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/patients/list",
					Target: "#patients",
					Body:   `<section><ui.PatientCard name="Updated & safe" /></section>`,
				}},
			},
		}},
		Components: []manifest.Component{{
			Name:    "PatientCard",
			Package: "components",
			Props:   []manifest.Prop{{Name: "name", Type: "string"}},
			Blocks:  manifest.Blocks{View: true, ViewBody: `<article>{name}</article>`},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected one fragment endpoint, got %#v", routes)
	}
	if routes[0].HTML != "<section><article>Updated &amp; safe</article></section>" {
		t.Fatalf("unexpected fragment HTML: %q", routes[0].HTML)
	}
	if routes[0].Package != "pages" || routes[0].Uses["ui"] != "components" {
		t.Fatalf("expected fragment render context, got %#v", routes[0])
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

func TestGenerateRejectsEmptyActionValidationRule(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:          "newsletter",
		ActionName:      "Subscribe",
		Route:           "/newsletter",
		InputFields:     []string{"email"},
		ValidationRules: []ActionValidationRule{{Field: "email"}},
		ValidatesInput:  true,
		Redirect:        "/newsletter?ok=1",
	}}})
	if err == nil {
		t.Fatal("expected empty validation rule error")
	}
	if !strings.Contains(err.Error(), `validation field "email" has no constraints`) {
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

func TestGeneratedBinaryAppliesSSRCachePolicy(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "docs", "index.html"), "<main>Stale docs</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "docs",
		Route:  "/docs",
		Cache:  "public, max-age=60",
		HTML:   "<main><h1>Request Docs</h1></main>",
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

	body, headers, err := waitForHTTPResponse("http://" + addr + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "<main><h1>Request Docs</h1></main>" {
		t.Fatalf("unexpected SSR response body: %s", body)
	}
	if cacheControl := headers.Get("Cache-Control"); cacheControl != "public, max-age=60" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryAppliesSPAHTMLCachePolicy(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{
  "version": 1,
  "files": {},
  "cache": {
    "index.html": "public, max-age=120, stale-while-revalidate=30"
  }
}
`)

	if _, err := Generate(outputDir, appDir); err != nil {
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

	body, headers, err := waitForHTTPResponse("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "<main>Home</main>" {
		t.Fatalf("unexpected SPA response body: %s", body)
	}
	if cacheControl := headers.Get("Cache-Control"); cacheControl != "public, max-age=120, stale-while-revalidate=30" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryExecutesSSRLoadUserLogic(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "dashboard",
		Route:   "/dashboard",
		HasLoad: true,
		LoadBinding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    manifest.BackendSignatureLoadError,
		},
		HTML: `<main><h1>__USER__</h1><p>__PATH__</p></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.name", Placeholder: "__USER__"},
			{Path: "request.path", Placeholder: "__PATH__"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "dashboard", "dashboard.go"), `package dashboard

import "github.com/cssbruno/gowdk/addons/ssr"

func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	type requestData struct {
		Path string
	}
	return map[string]any{
		"user": map[string]any{"name": "Ada <admin>"},
		"request": requestData{Path: ctx.Request.URL.Path},
	}, nil
}
`)
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
	for _, expected := range []string{"Ada &lt;admin&gt;", "<p>/dashboard</p>"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected SSR load response to contain %q, got %s", expected, body)
		}
	}
	if cacheControl := headers.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryUsesCustomSSRLoadErrorPage(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "dashboard.html"), "<main>Dashboard Error</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:    "dashboard",
		Route:     "/dashboard",
		ErrorPage: "errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    manifest.BackendSignatureLoadError,
		},
		HTML: `<main><h1>__USER__</h1></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.name", Placeholder: "__USER__"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "dashboard", "dashboard.go"), `package dashboard

import (
	"errors"

	"github.com/cssbruno/gowdk/addons/ssr"
)

func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return nil, errors.New("secret database detail")
}
`)
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

	response, err := waitForHTTPStatus("http://"+addr+"/dashboard", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d with body %s", response.StatusCode, body)
	}
	if strings.TrimSpace(body) != "<main>Dashboard Error</main>" {
		t.Fatalf("unexpected custom error body: %s", body)
	}
	if strings.Contains(body, "secret database detail") {
		t.Fatalf("custom error page leaked load error detail: %s", body)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryUsesCustomSSRPanicErrorPage(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "dashboard.html"), "<main>Dashboard Error</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:    "dashboard",
		Route:     "/dashboard",
		ErrorPage: "errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    manifest.BackendSignatureLoad,
		},
		HTML: `<main><h1>__USER__</h1></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.name", Placeholder: "__USER__"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "dashboard", "dashboard.go"), `package dashboard

import "github.com/cssbruno/gowdk/addons/ssr"

func LoadDashboard(ctx ssr.LoadContext) map[string]any {
	panic("secret database detail")
}
`)
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

	response, err := waitForHTTPStatus("http://"+addr+"/dashboard", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d with body %s", response.StatusCode, body)
	}
	if strings.TrimSpace(body) != "<main>Dashboard Error</main>" {
		t.Fatalf("unexpected custom error body: %s", body)
	}
	if strings.Contains(body, "secret database detail") {
		t.Fatalf("custom error page leaked panic detail: %s", body)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryUsesCustomAPIErrorPage(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "errors", "health.html"), "<main>Health Error</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{APIs: []APIEndpoint{{
		PageID:    "status",
		APIName:   "Health",
		Method:    http.MethodGet,
		Route:     "/api/health",
		ErrorPage: "errors/health.html",
		Binding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/status",
			PackageName:  "status",
			FunctionName: "Health",
			Signature:    manifest.BackendSignatureAPI,
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "status", "status.go"), `package status

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Health(ctx context.Context, request *http.Request) (response.Response, error) {
	panic("secret database detail")
}
`)
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

	response, err := waitForHTTPStatus("http://"+addr+"/api/health", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d with body %s", response.StatusCode, body)
	}
	if strings.TrimSpace(body) != "<main>Health Error</main>" {
		t.Fatalf("unexpected custom error body: %s", body)
	}
	if strings.Contains(body, "secret database detail") {
		t.Fatalf("custom error page leaked panic detail: %s", body)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestGeneratedBinaryHandlesEndpointErrorsAndMissingErrorPage(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "500.html"), "<main>Fallback 500</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     http.MethodPost,
			Route:      "/newsletter",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Subscribe",
				Signature:    manifest.BackendSignatureAction0,
			},
		}, {
			PageID:     "newsletter",
			ActionName: "Explode",
			Method:     http.MethodPost,
			Route:      "/explode",
			ErrorPage:  "errors/missing.html",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Explode",
				Signature:    manifest.BackendSignatureAction0,
			},
		}},
		APIs: []APIEndpoint{{
			PageID:  "session",
			APIName: "Session",
			Method:  http.MethodGet,
			Route:   "/api/session",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Session",
				Signature:    manifest.BackendSignatureAPI,
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "backend", "backend.go"), `package backend

import (
	"context"
	"net/http"

	gowdkresponse "github.com/cssbruno/gowdk/runtime/response"
)

func Subscribe(ctx context.Context) (gowdkresponse.Response, error) {
	return gowdkresponse.Response{}, gowdkresponse.NewHandlerError(http.StatusConflict, "duplicate subscription", nil)
}

func Explode(ctx context.Context) (gowdkresponse.Response, error) {
	panic("secret action detail")
}

func Session(ctx context.Context, request *http.Request) (gowdkresponse.Response, error) {
	return gowdkresponse.Response{}, gowdkresponse.NewHandlerError(http.StatusForbidden, "session expired", nil)
}
`)
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

	actionResponse, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "")
	if err != nil {
		t.Fatal(err)
	}
	actionPayload, err := io.ReadAll(actionResponse.Body)
	_ = actionResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if actionResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected action handler error to return 409, got %d: %s", actionResponse.StatusCode, actionPayload)
	}
	if !strings.Contains(string(actionPayload), "duplicate subscription") {
		t.Fatalf("expected action handler error body, got %s", actionPayload)
	}
	if cacheControl := actionResponse.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on action handler error, got %q", cacheControl)
	}

	apiResponse, err := waitForHTTPStatus("http://"+addr+"/api/session", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	apiPayload, err := io.ReadAll(apiResponse.Body)
	_ = apiResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if apiResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected API handler error to return 403, got %d: %s", apiResponse.StatusCode, apiPayload)
	}
	if !strings.Contains(string(apiPayload), "session expired") {
		t.Fatalf("expected API handler error body, got %s", apiPayload)
	}
	if cacheControl := apiResponse.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on API handler error, got %q", cacheControl)
	}

	panicResponse, err := waitForHTTPStatus("http://"+addr+"/explode", http.MethodPost, "")
	if err != nil {
		t.Fatal(err)
	}
	panicPayload, err := io.ReadAll(panicResponse.Body)
	_ = panicResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	panicBody := string(panicPayload)
	if panicResponse.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected action panic to return 500, got %d: %s", panicResponse.StatusCode, panicBody)
	}
	if strings.TrimSpace(panicBody) != "<main>Fallback 500</main>" {
		t.Fatalf("expected missing custom error document to fall back to default 500 page, got %s", panicBody)
	}
	if strings.Contains(panicBody, "secret action detail") {
		t.Fatalf("fallback error page leaked panic detail: %s", panicBody)
	}
	if cacheControl := panicResponse.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on action panic boundary, got %q", cacheControl)
	}
}

func TestGeneratedBinarySSRGuardFailsClosedWithoutRegistry(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "dashboard",
		Route:  "/dashboard",
		Guards: []string{"auth.required"},
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

	response, err := waitForHTTPStatus("http://"+addr+"/dashboard", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected guarded SSR route to fail closed with 403, got %d: %s", response.StatusCode, payload)
	}
	if !strings.Contains(string(payload), `SSR guard "auth.required" is not registered`) {
		t.Fatalf("expected missing guard registry error, got %s", payload)
	}
}

func TestGeneratedBinaryBackendGuardsFailClosedWithoutRegistry(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
			Guards:     []string{"auth.required"},
			Redirect:   "/newsletter?ok=1",
		}},
		APIs: []APIEndpoint{{
			PageID:  "session",
			APIName: "Session",
			Method:  "GET",
			Route:   "/api/session",
			Guards:  []string{"auth.required"},
		}},
	}); err != nil {
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

	actionResponse, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=a@example.com")
	if err != nil {
		t.Fatal(err)
	}
	actionPayload, err := io.ReadAll(actionResponse.Body)
	_ = actionResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if actionResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected guarded action to fail closed with 403, got %d: %s", actionResponse.StatusCode, actionPayload)
	}

	apiResponse, err := waitForHTTPStatus("http://"+addr+"/api/session", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	apiPayload, err := io.ReadAll(apiResponse.Body)
	_ = apiResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if apiResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected guarded API to fail closed with 403, got %d: %s", apiResponse.StatusCode, apiPayload)
	}
}

func TestGeneratedBinaryRegisteredGuardsAllowRequestTimeRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
			Guards:     []string{"auth.required"},
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Subscribe",
				Signature:    manifest.BackendSignatureAction0,
			},
		}},
		APIs: []APIEndpoint{{
			PageID:  "session",
			APIName: "Session",
			Method:  "GET",
			Route:   "/api/session",
			Guards:  []string{"auth.required"},
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Session",
				Signature:    manifest.BackendSignatureAPI,
			},
		}},
		SSR: []SSRRoute{{
			PageID: "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required"},
			HTML:   "<main><h1>Request Dashboard</h1></main>",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, appPackageDirName, "guards_register.go"), `package gowdkapp

import gowdkssr "github.com/cssbruno/gowdk/addons/ssr"

func init() {
	RegisterGuards(gowdkssr.GuardRegistry{
		"auth.required": func(ctx gowdkssr.LoadContext) error {
			return nil
		},
	})
}
`)
	writeTestFile(t, filepath.Join(appDir, "backend", "backend.go"), `package backend

import (
	"context"
	"net/http"

	gowdkresponse "github.com/cssbruno/gowdk/runtime/response"
)

func Subscribe(context.Context) (gowdkresponse.Response, error) {
	return gowdkresponse.RedirectTo("/newsletter?ok=1"), nil
}

func Session(context.Context, *http.Request) (gowdkresponse.Response, error) {
	return gowdkresponse.JSONBody(http.StatusOK, `+"`"+`{"ok":true}`+"`"+`), nil
}
`)
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

	ssrBody, _, err := waitForHTTPResponse("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ssrBody, "Request Dashboard") {
		t.Fatalf("expected registered guard to allow SSR route, got %s", ssrBody)
	}

	actionResponse, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = actionResponse.Body.Close()
	if actionResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected registered guard to allow action redirect, got %d", actionResponse.StatusCode)
	}

	apiResponse, err := waitForHTTPStatus("http://"+addr+"/api/session", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	apiPayload, err := io.ReadAll(apiResponse.Body)
	_ = apiResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if apiResponse.StatusCode != http.StatusOK || !strings.Contains(string(apiPayload), `"ok":true`) {
		t.Fatalf("expected registered guard to allow API response, got %d: %s", apiResponse.StatusCode, apiPayload)
	}
}

func TestGeneratedBinaryAppliesRegisteredRateLimiter(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ratelimit", gowdk.FeatureRateLimit)}},
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Method:      "POST",
			Route:       "/newsletter",
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, appPackageDirName, "ratelimit_register.go"), `package gowdkapp

import (
	"time"

	gowdkratelimit "github.com/cssbruno/gowdk/addons/ratelimit"
)

func init() {
	store := gowdkratelimit.NewInMemoryStore(gowdkratelimit.InMemoryOptions{})
	limiter, err := gowdkratelimit.New(gowdkratelimit.Options{
		Limit: 1,
		Window: time.Hour,
		Store: store,
	})
	if err != nil {
		panic(err)
	}
	RegisterRateLimiter(limiter)
}
`)
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

	first, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Body.Close()
	if first.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected first request to reach generated action redirect, got %d", first.StatusCode)
	}
	if first.Header.Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("expected rate-limit headers on allowed response, got %#v", first.Header)
	}

	second, err := waitForHTTPStatus("http://"+addr+"/newsletter", http.MethodPost, "email=reader%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(second.Body)
	_ = second.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate-limited with 429, got %d: %s", second.StatusCode, payload)
	}
	if second.Header.Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on limited response, got %#v", second.Header)
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
		PageID:           "newsletter",
		ActionName:       "Subscribe",
		Route:            "/newsletter",
		InputName:        "input",
		InputType:        "SubscribeInput",
		InputFields:      []string{"email"},
		RequiredFields:   []string{"email"},
		RequiredMessages: map[string]string{"email": "Email is required"},
		ValidationRules: []ActionValidationRule{{
			Field:          "email",
			MinLength:      5,
			MaxLength:      80,
			Pattern:        `[a-z]+@[a-z]+[.][a-z]{2,4}`,
			PatternMessage: "Use a real email address",
		}},
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

	response, err = waitForHTTPStatusWithHeaders("http://"+addr+"/newsletter", http.MethodPost, "email=", map[string]string{
		"X-GOWDK-Partial": "1",
		"X-GOWDK-Target":  "#errors",
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
		t.Fatalf("expected partial validation fragment to return 200, got %d: %s", response.StatusCode, payload)
	}
	for _, expected := range []string{`<div data-gowdk-validation>`, `data-gowdk-field="email"`, `Email is required`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected validation fragment to contain %q, got %s", expected, payload)
		}
	}
	if response.Header.Get("X-GOWDK-Fragment-Target") != "#errors" {
		t.Fatalf("unexpected validation fragment target: %q", response.Header.Get("X-GOWDK-Fragment-Target"))
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on validation fragment response, got %q", cacheControl)
	}

	response, err = waitForHTTPStatusWithHeaders("http://"+addr+"/newsletter", http.MethodPost, "email=invalid", map[string]string{
		"X-GOWDK-Partial": "1",
		"X-GOWDK-Target":  "#errors",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected partial pattern validation fragment to return 200, got %d: %s", response.StatusCode, payload)
	}
	for _, expected := range []string{`<div data-gowdk-validation>`, `data-gowdk-field="email"`, `Use a real email address`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected pattern validation fragment to contain %q, got %s", expected, payload)
		}
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

func TestGeneratedBinaryServesStandaloneFragmentRoute(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Fragments: []FragmentEndpoint{{
		PageID:       "patients",
		FragmentName: "List",
		Method:       "GET",
		Route:        "/patients/list",
		Target:       "#patients",
		HTML:         "<section><p>Updated patients</p></section>",
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients/list", http.MethodGet, "")
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
	if response.Header.Get("X-GOWDK-Fragment-Swap") != "innerHTML" {
		t.Fatalf("unexpected fragment swap: %q", response.Header.Get("X-GOWDK-Fragment-Swap"))
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on standalone fragment response, got %q", cacheControl)
	}
}

func TestGeneratedBinaryExecutesFragmentUserHook(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Fragments: []FragmentEndpoint{{
		PageID:       "patients",
		FragmentName: "List",
		Method:       "GET",
		Route:        "/patients/list",
		Target:       "#patients",
		HTML:         "<section><p>Static fallback</p></section>",
		Binding: manifest.BackendBinding{
			Status:       manifest.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/patients",
			PackageName:  "patients",
			FunctionName: "List",
			Signature:    manifest.BackendSignatureFragment,
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	gowdkapp "github.com/cssbruno/gowdk/runtime/app"
	"github.com/cssbruno/gowdk/runtime/response"
)

func List(ctx context.Context) (response.Response, error) {
	request, ok := gowdkapp.Request(ctx)
	if !ok {
		return response.HTMLBody(500, "missing request"), nil
	}
	return response.FragmentFor("#patients", "<section><p>Runtime "+request.URL.Query().Get("q")+"</p></section>"), nil
}
`)
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients/list?q=hook", http.MethodGet, "")
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
	if strings.TrimSpace(string(payload)) != "<section><p>Runtime hook</p></section>" {
		t.Fatalf("expected runtime fragment hook response, got %s", payload)
	}
	if strings.Contains(string(payload), "Static fallback") {
		t.Fatalf("expected runtime hook to replace static fallback, got %s", payload)
	}
	if response.Header.Get("X-GOWDK-Fragment-Target") != "#patients" {
		t.Fatalf("unexpected fragment target: %q", response.Header.Get("X-GOWDK-Fragment-Target"))
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

func generatedFunctionSource(t *testing.T, source string, name string) string {
	t.Helper()
	start := strings.Index(source, "\nfunc "+name+"(")
	if start < 0 {
		start = strings.Index(source, "func "+name+"(")
	}
	if start < 0 {
		t.Fatalf("expected generated source to contain func %s:\n%s", name, source)
	}
	end := strings.Index(source[start+1:], "\nfunc ")
	if end < 0 {
		return source[start:]
	}
	return source[start : start+1+end]
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
