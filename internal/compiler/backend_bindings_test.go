package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/goblockgen"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBindBackendHandlersClassifiesSupportedActionSignatures(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26\n")
	writeCompilerTestFile(t, filepath.Join(root, "auth.go"), `package auth

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

type LoginInput struct {
	Email string `+"`form:\"email\"`"+`
	Tags []string `+"`form:\"tag\"`"+`
	Remember bool `+"`form:\"remember\"`"+`
	Age int `+"`form:\"age\"`"+`
	Score uint64 `+"`form:\"score\"`"+`
	Internal string `+"`form:\"-\"`"+`
	ignored string
}

type BrokenInput struct {
	Nested map[string]string `+"`form:\"nested\"`"+`
}

func Ping(context.Context) (response.Response, error) {
	return response.Response{}, nil
}

func Login(context.Context, LoginInput) (response.Response, error) {
	return response.Response{}, nil
}

func LoginPtr(context.Context, *LoginInput) (response.Response, error) {
	return response.Response{}, nil
}

func Raw(context.Context, form.Values) (response.Response, error) {
	return response.Response{}, nil
}

func Broken(context.Context, BrokenInput) (response.Response, error) {
	return response.Response{}, nil
}

func Session(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}

func Bad(LoginInput) (response.Response, error) {
	return response.Response{}, nil
}

func List(context.Context) (response.Response, error) {
	return response.FragmentFor("#patients", "<p>runtime</p>"), nil
}

func BrokenFragment(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}
`)

	app := BindBackendHandlers(manifest.Manifest{Pages: []manifest.Page{{
		ID:     "Login",
		Source: filepath.Join(root, "Login.page.gwdk"),
		Route:  "/Login",
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{
				{Name: "Ping"},
				{Name: "Login"},
				{Name: "LoginPtr"},
				{Name: "Raw"},
				{Name: "Broken"},
				{Name: "Bad"},
				{Name: "Missing"},
			},
			APIs: []manifest.API{{
				Name:   "Session",
				Method: "GET",
				Route:  "/api/Session",
			}},
			Fragments: []manifest.FragmentEndpoint{
				{Name: "List", Method: "GET", Route: "/patients/list", Target: "#patients"},
				{Name: "BrokenFragment", Method: "GET", Route: "/patients/broken", Target: "#patients"},
				{Name: "MissingFragment", Method: "GET", Route: "/patients/missing", Target: "#patients"},
			},
		},
	}}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["Ping"], manifest.BackendBindingBound, manifest.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Login"], manifest.BackendBindingBound, manifest.BackendSignatureActionForm, "LoginInput", false)
	assertBinding(t, bindings["LoginPtr"], manifest.BackendBindingBound, manifest.BackendSignatureActionFormPtr, "LoginInput", true)
	assertBinding(t, bindings["Raw"], manifest.BackendBindingBound, manifest.BackendSignatureActionValues, "", false)
	assertBinding(t, bindings["Session"], manifest.BackendBindingBound, manifest.BackendSignatureAPI, "", false)
	assertInputFields(t, bindings["Login"].InputFields, "Email:email:string,Tags:tag:[]string,Remember:remember:bool,Age:age:int,Score:score:uint64")
	if got := bindings["Broken"]; got.Status != manifest.BackendBindingUnsupportedSignature {
		t.Fatalf("expected Broken unsupported signature, got %#v", got)
	}
	if !strings.Contains(bindings["Broken"].Message, "unsupported field type") {
		t.Fatalf("expected Broken message to explain unsupported field type, got %q", bindings["Broken"].Message)
	}
	if got := bindings["Bad"]; got.Status != manifest.BackendBindingUnsupportedSignature {
		t.Fatalf("expected Bad unsupported signature, got %#v", got)
	}
	if got := bindings["Missing"]; got.Status != manifest.BackendBindingMissing {
		t.Fatalf("expected Missing binding, got %#v", got)
	}
	assertBinding(t, bindings["List"], manifest.BackendBindingBound, manifest.BackendSignatureFragment, "", false)
	if got := bindings["BrokenFragment"]; got.Status != manifest.BackendBindingUnsupportedSignature {
		t.Fatalf("expected BrokenFragment unsupported signature, got %#v", got)
	}
	if _, ok := bindings["MissingFragment"]; ok {
		t.Fatalf("did not expect missing static fragment fallback to create a backend binding")
	}
}

func TestBindBackendHandlersClassifiesSSRLoadSignatures(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26\n")
	writeCompilerTestFile(t, filepath.Join(root, "dashboard.go"), `package dashboard

import "github.com/cssbruno/gowdk/addons/ssr"

func LoadDashboard(ssr.LoadContext) (map[string]any, error) {
	return map[string]any{"user": "Ada"}, nil
}

func LoadProfile(ssr.LoadContext) map[string]any {
	return map[string]any{"user": "Ada"}
}

func LoadBroken() map[string]any {
	return nil
}
`)

	app := BindBackendHandlers(manifest.Manifest{Pages: []manifest.Page{
		{
			ID:     "dashboard",
			Source: filepath.Join(root, "dashboard.page.gwdk"),
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				Load: true,
			},
		},
		{
			ID:     "profile",
			Source: filepath.Join(root, "profile.page.gwdk"),
			Route:  "/profile",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				Load: true,
			},
		},
		{
			ID:     "broken",
			Source: filepath.Join(root, "broken.page.gwdk"),
			Route:  "/broken",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				Load: true,
			},
		},
		{
			ID:     "missing",
			Source: filepath.Join(root, "missing.page.gwdk"),
			Route:  "/missing",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				Load: true,
			},
		},
	}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["LoadDashboard"], manifest.BackendBindingBound, manifest.BackendSignatureLoadError, "", false)
	assertBinding(t, bindings["LoadProfile"], manifest.BackendBindingBound, manifest.BackendSignatureLoad, "", false)
	if got := bindings["LoadBroken"]; got.Status != manifest.BackendBindingUnsupportedSignature {
		t.Fatalf("expected LoadBroken unsupported signature, got %#v", got)
	}
	if got := bindings["LoadMissing"]; got.Status != manifest.BackendBindingMissing {
		t.Fatalf("expected LoadMissing missing binding, got %#v", got)
	}
	if app.Pages[0].LoadBinding.FunctionName != "LoadDashboard" || app.Pages[0].LoadBinding.Status != manifest.BackendBindingBound {
		t.Fatalf("expected page load binding to be attached, got %#v", app.Pages[0].LoadBinding)
	}
}

func TestBindBackendHandlersBindsInlineSSRScriptLoad(t *testing.T) {
	root := t.TempDir()
	page := manifest.Page{
		ID:      "dashboard",
		Package: "pages",
		Source:  filepath.Join(root, "dashboard.page.gwdk"),
		Route:   "/dashboard",
		Render:  gowdk.SSR,
		Imports: []manifest.Import{{
			Alias: "ssr",
			Path:  ssrImportPath,
		}},
		Blocks: manifest.Blocks{
			Load: true,
			GoBlocks: []manifest.GoBlock{{
				Target: "ssr",
				Body: `func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return map[string]any{"user": "Ada"}, nil
}`,
			}},
		},
	}

	app := BindBackendHandlers(manifest.Manifest{Pages: []manifest.Page{page}})
	bindings := compilerBindingsByBlock(app.BackendBindings)
	binding := bindings["LoadDashboard"]
	if binding.Status != manifest.BackendBindingBound || binding.Signature != manifest.BackendSignatureLoadError {
		t.Fatalf("expected inline SSR load binding, got %#v", binding)
	}
	if binding.ImportPath != goblockgen.GeneratedImportPath("pages") || binding.PackageName != "pages" {
		t.Fatalf("unexpected inline go block import metadata: %#v", binding)
	}
	if app.Pages[0].LoadBinding.ImportPath != goblockgen.GeneratedImportPath("pages") {
		t.Fatalf("expected page load binding to use generated go block package, got %#v", app.Pages[0].LoadBinding)
	}
}

func TestBindBackendHandlersBindsDefaultInlineGoBlockEndpoints(t *testing.T) {
	root := t.TempDir()
	page := manifest.Page{
		ID:      "home",
		Package: "pages",
		Source:  filepath.Join(root, "home.page.gwdk"),
		Route:   "/",
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{{
				Name:   "Subscribe",
				Method: "POST",
				Route:  "/newsletter",
			}},
			APIs: []manifest.API{{
				Name:   "Session",
				Method: "GET",
				Route:  "/api/session",
			}},
			Fragments: []manifest.FragmentEndpoint{{
				Name:   "List",
				Method: "GET",
				Route:  "/items",
				Target: "#items",
			}},
			GoBlocks: []manifest.GoBlock{{
				Body: `import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Subscribe(context.Context) (response.Response, error) {
	return response.RedirectTo("/?subscribed=1"), nil
}

func Session(context.Context, *http.Request) (response.Response, error) {
	return response.JSONValue(http.StatusOK, map[string]bool{"authenticated": true})
}

func List(context.Context) (response.Response, error) {
	return response.FragmentFor("#items", "<ul><li>One</li></ul>"), nil
}`,
			}},
		},
	}

	app := BindBackendHandlers(manifest.Manifest{Pages: []manifest.Page{page}})
	bindings := compilerBindingsByBlock(app.BackendBindings)
	for _, name := range []string{"Subscribe", "Session", "List"} {
		if bindings[name].ImportPath != goblockgen.GeneratedImportPath("pages") || bindings[name].PackageName != "pages" {
			t.Fatalf("expected %s to bind generated inline go package, got %#v", name, bindings[name])
		}
	}
	assertBinding(t, bindings["Subscribe"], manifest.BackendBindingBound, manifest.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Session"], manifest.BackendBindingBound, manifest.BackendSignatureAPI, "", false)
	assertBinding(t, bindings["List"], manifest.BackendBindingBound, manifest.BackendSignatureFragment, "", false)
}

func TestDiscoverGoEndpointCommentsBindsStandaloneEndpoints(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26\n")
	writeCompilerTestFile(t, filepath.Join(root, "handlers.go"), `package api

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

//gowdk:act POST /login
func Login(context.Context) (response.Response, error) {
	return response.Response{}, nil
}

//gowdk:api GET /api/session
func Session(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}
`)
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "home",
		Source: filepath.Join(root, "home.page.gwdk"),
		Route:  "/",
		Blocks: manifest.Blocks{View: true, ViewBody: "<main>Home</main>"},
	}}}

	app, err := DiscoverGoEndpointComments(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(app.Endpoints) != 2 {
		t.Fatalf("expected two Go comment endpoints, got %#v", app.Endpoints)
	}
	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatal(err)
	}
	app = BindBackendHandlers(app)
	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["Login"], manifest.BackendBindingBound, manifest.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Session"], manifest.BackendBindingBound, manifest.BackendSignatureAPI, "", false)
}

func TestValidateManifestRejectsGoEndpointConflictWithGOWDKEndpoint(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "home",
			Source: source,
			Route:  "/",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: "<main>Home</main>",
				APIs:     []manifest.API{{Name: "Session", Method: "GET", Route: "/api/session"}},
			},
		}},
		Endpoints: []manifest.EndpointDeclaration{{
			Kind:       "api",
			SourceKind: manifest.EndpointSourceGo,
			Package:    "api",
			Source:     filepath.Join(root, "handlers.go"),
			Name:       "Session",
			Method:     "GET",
			Route:      "/api/session",
		}},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected route conflict diagnostic")
	}
	if !hasDiagnosticCode(err.(ValidationErrors), "route_method_conflict") {
		t.Fatalf("missing route_method_conflict diagnostic: %#v", err)
	}
}

func TestValidateBackendBindingPolicyFailsProductionMissingHandler(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "login",
		Source: filepath.Join(t.TempDir(), "login.page.gwdk"),
		Route:  "/login",
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{{Name: "Login", Method: "POST"}},
		},
	}}}

	err := ValidateBackendBindingPolicy(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, app)
	if err == nil {
		t.Fatal("expected production missing handler diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "backend_binding_required") {
		t.Fatalf("missing backend_binding_required diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(err.Error(), "--allow-missing-backend") {
		t.Fatalf("expected diagnostic to mention explicit stub flag, got %v", err)
	}
}

func TestValidateBackendBindingPolicyAllowsDevelopmentMissingHandler(t *testing.T) {
	app := manifest.Manifest{BackendBindings: []manifest.BackendBinding{{
		Kind:         actionHandlerKind,
		PageID:       "login",
		BlockName:    "Login",
		Method:       "POST",
		Route:        "/login",
		FunctionName: "Login",
		Status:       manifest.BackendBindingMissing,
	}}}

	if err := ValidateBackendBindingPolicy(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected development missing handler to remain non-fatal, got %v", err)
	}
}

func TestValidateBackendBindingPolicyAllowsExplicitProductionStubMode(t *testing.T) {
	app := manifest.Manifest{BackendBindings: []manifest.BackendBinding{{
		Kind:         apiHandlerKind,
		PageID:       "session",
		BlockName:    "Session",
		Method:       "GET",
		Route:        "/api/session",
		FunctionName: "Session",
		Status:       manifest.BackendBindingUnsupportedSignature,
	}}}

	config := gowdk.Config{Build: gowdk.BuildConfig{
		Mode:                gowdk.Production,
		AllowMissingBackend: true,
	}}
	if err := ValidateBackendBindingPolicy(config, app); err != nil {
		t.Fatalf("expected explicit production stub mode to allow missing backend, got %v", err)
	}
}

func assertBinding(t *testing.T, binding manifest.BackendBinding, status manifest.BackendBindingStatus, signature manifest.BackendSignatureKind, inputType string, inputPointer bool) {
	t.Helper()
	if binding.Status != status || binding.Signature != signature || binding.InputType != inputType || binding.InputPointer != inputPointer {
		t.Fatalf("unexpected binding: %#v", binding)
	}
}

func assertInputFields(t *testing.T, fields []manifest.BackendInputField, expected string) {
	t.Helper()
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.FieldName+":"+field.FormName+":"+field.Type)
	}
	if strings.Join(parts, ",") != expected {
		t.Fatalf("unexpected input fields: %s", strings.Join(parts, ","))
	}
}

func compilerBindingsByBlock(bindings []manifest.BackendBinding) map[string]manifest.BackendBinding {
	out := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		out[binding.BlockName] = binding
	}
	return out
}

func writeCompilerTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
