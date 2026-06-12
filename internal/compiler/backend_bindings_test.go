package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/goblockgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBindBackendHandlersClassifiesSupportedActionSignatures(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
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

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{{
		ID:     "Login",
		Source: filepath.Join(root, "Login.page.gwdk"),
		Route:  "/Login",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{
				{Name: "Ping"},
				{Name: "Login"},
				{Name: "LoginPtr"},
				{Name: "Raw"},
				{Name: "Broken"},
				{Name: "Bad"},
				{Name: "Missing"},
			},
			APIs: []gwdkir.API{{
				Name:   "Session",
				Method: "GET",
				Route:  "/api/Session",
			}},
			Fragments: []gwdkir.FragmentEndpoint{
				{Name: "List", Method: "GET", Route: "/patients/list", Target: "#patients"},
				{Name: "BrokenFragment", Method: "GET", Route: "/patients/broken", Target: "#patients"},
				{Name: "MissingFragment", Method: "GET", Route: "/patients/missing", Target: "#patients"},
			},
		},
	}}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["Ping"], source.BackendBindingBound, source.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Login"], source.BackendBindingBound, source.BackendSignatureActionForm, "LoginInput", false)
	assertBinding(t, bindings["LoginPtr"], source.BackendBindingBound, source.BackendSignatureActionFormPtr, "LoginInput", true)
	assertBinding(t, bindings["Raw"], source.BackendBindingBound, source.BackendSignatureActionValues, "", false)
	assertBinding(t, bindings["Session"], source.BackendBindingBound, source.BackendSignatureAPI, "", false)
	assertInputFields(t, bindings["Login"].InputFields, "Email:email:string,Tags:tag:[]string,Remember:remember:bool,Age:age:int,Score:score:uint64")
	if got := bindings["Broken"]; got.Status != source.BackendBindingUnsupportedSignature {
		t.Fatalf("expected Broken unsupported signature, got %#v", got)
	}
	if !strings.Contains(bindings["Broken"].Message, "unsupported field type") {
		t.Fatalf("expected Broken message to explain unsupported field type, got %q", bindings["Broken"].Message)
	}
	if got := bindings["Bad"]; got.Status != source.BackendBindingUnsupportedSignature {
		t.Fatalf("expected Bad unsupported signature, got %#v", got)
	}
	if got := bindings["Missing"]; got.Status != source.BackendBindingMissing {
		t.Fatalf("expected Missing binding, got %#v", got)
	}
	assertBinding(t, bindings["List"], source.BackendBindingBound, source.BackendSignatureFragment, "", false)
	if got := bindings["BrokenFragment"]; got.Status != source.BackendBindingUnsupportedSignature {
		t.Fatalf("expected BrokenFragment unsupported signature, got %#v", got)
	}
	if _, ok := bindings["MissingFragment"]; ok {
		t.Fatalf("did not expect missing static fragment fallback to create a backend binding")
	}
}

func TestBindBackendHandlersUsesTypedGoPackageResolution(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	writeCompilerTestFile(t, filepath.Join(root, "auth.go"), `package auth

import (
	ctx "context"
	nethttp "net/http"

	resp "github.com/cssbruno/gowdk/runtime/response"
)

type ActionResult = resp.Response

type AliasInput struct {
	Email string `+"`form:\"email\"`"+`
}

func Login(ctx.Context, AliasInput) (ActionResult, error) {
	return resp.Response{}, nil
}

func Session(ctx.Context, *nethttp.Request) (ActionResult, error) {
	return resp.Response{}, nil
}

func hidden(ctx.Context) (ActionResult, error) {
	return resp.Response{}, nil
}
`)

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{{
		ID:     "login",
		Source: filepath.Join(root, "login.page.gwdk"),
		Route:  "/login",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{
				{Name: "Login"},
				{Name: "hidden"},
			},
			APIs: []gwdkir.API{{
				Name:   "Session",
				Method: "GET",
				Route:  "/api/session",
			}},
		},
	}}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["Login"], source.BackendBindingBound, source.BackendSignatureActionForm, "AliasInput", false)
	assertInputFields(t, bindings["Login"].InputFields, "Email:email:string")
	assertBinding(t, bindings["Session"], source.BackendBindingBound, source.BackendSignatureAPI, "", false)
	if got := bindings["hidden"]; got.Status != source.BackendBindingMissing {
		t.Fatalf("expected unexported handler to remain missing, got %#v", got)
	}
}

func TestBindBackendHandlersHonorsBuildTags(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	writeCompilerTestFile(t, filepath.Join(root, "base.go"), `package auth
`)
	writeCompilerTestFile(t, filepath.Join(root, "tagged.go"), `//go:build gowdkextra

package auth

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Tagged(context.Context) (response.Response, error) {
	return response.Response{}, nil
}
`)

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{{
		ID:     "tagged",
		Source: filepath.Join(root, "tagged.page.gwdk"),
		Route:  "/tagged",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{{Name: "Tagged"}},
		},
	}}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	if got := bindings["Tagged"]; got.Status != source.BackendBindingMissing {
		t.Fatalf("expected build-tagged handler to be missing without active tag, got %#v", got)
	}
}

func TestBindBackendHandlersReportsPackageLoadErrors(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	writeCompilerTestFile(t, filepath.Join(root, "broken.go"), `package broken

func Broken(
`)

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{{
		ID:     "broken",
		Source: filepath.Join(root, "broken.page.gwdk"),
		Route:  "/broken",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{{Name: "Broken"}},
		},
	}}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	if got := bindings["Broken"]; got.Status != source.BackendBindingMissing || !strings.Contains(got.Message, "could not be inspected") {
		t.Fatalf("expected package load error metadata, got %#v", got)
	}
}

func TestBindBackendHandlersClassifiesSSRLoadSignatures(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
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

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{
		{
			ID:     "dashboard",
			Source: filepath.Join(root, "dashboard.page.gwdk"),
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				Load: true,
			},
		},
		{
			ID:     "profile",
			Source: filepath.Join(root, "profile.page.gwdk"),
			Route:  "/profile",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				Load: true,
			},
		},
		{
			ID:     "broken",
			Source: filepath.Join(root, "broken.page.gwdk"),
			Route:  "/broken",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				Load: true,
			},
		},
		{
			ID:     "missing",
			Source: filepath.Join(root, "missing.page.gwdk"),
			Route:  "/missing",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				Load: true,
			},
		},
	}})

	bindings := compilerBindingsByBlock(app.BackendBindings)
	assertBinding(t, bindings["LoadDashboard"], source.BackendBindingBound, source.BackendSignatureLoadError, "", false)
	assertBinding(t, bindings["LoadProfile"], source.BackendBindingBound, source.BackendSignatureLoad, "", false)
	if got := bindings["LoadBroken"]; got.Status != source.BackendBindingUnsupportedSignature {
		t.Fatalf("expected LoadBroken unsupported signature, got %#v", got)
	}
	if got := bindings["LoadMissing"]; got.Status != source.BackendBindingMissing {
		t.Fatalf("expected LoadMissing missing binding, got %#v", got)
	}
	if app.Pages[0].LoadBinding.FunctionName != "LoadDashboard" || app.Pages[0].LoadBinding.Status != source.BackendBindingBound {
		t.Fatalf("expected page load binding to be attached, got %#v", app.Pages[0].LoadBinding)
	}
}

func TestBindBackendHandlersBindsInlineSSRScriptLoad(t *testing.T) {
	root := t.TempDir()
	page := gwdkir.Page{
		ID:      "dashboard",
		Package: "pages",
		Source:  filepath.Join(root, "dashboard.page.gwdk"),
		Route:   "/dashboard",
		Render:  gowdk.SSR,
		Imports: []gwdkir.Import{{
			Alias: "ssr",
			Path:  ssrImportPath,
		}},
		Blocks: gwdkir.Blocks{
			Load: true,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "ssr",
				Body: `func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return map[string]any{"user": "Ada"}, nil
}`,
			}},
		},
	}

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{page}})
	bindings := compilerBindingsByBlock(app.BackendBindings)
	binding := bindings["LoadDashboard"]
	if binding.Status != source.BackendBindingBound || binding.Signature != source.BackendSignatureLoadError {
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
	page := gwdkir.Page{
		ID:      "home",
		Package: "pages",
		Source:  filepath.Join(root, "home.page.gwdk"),
		Route:   "/",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{{
				Name:   "Subscribe",
				Method: "POST",
				Route:  "/newsletter",
			}},
			APIs: []gwdkir.API{{
				Name:   "Session",
				Method: "GET",
				Route:  "/api/session",
			}},
			Fragments: []gwdkir.FragmentEndpoint{{
				Name:   "List",
				Method: "GET",
				Route:  "/items",
				Target: "#items",
			}},
			GoBlocks: []gwdkir.GoBlock{{
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

	app := bindBackendHandlers(appFixture{Pages: []gwdkir.Page{page}})
	bindings := compilerBindingsByBlock(app.BackendBindings)
	for _, name := range []string{"Subscribe", "Session", "List"} {
		if bindings[name].ImportPath != goblockgen.GeneratedImportPath("pages") || bindings[name].PackageName != "pages" {
			t.Fatalf("expected %s to bind generated inline go package, got %#v", name, bindings[name])
		}
	}
	assertBinding(t, bindings["Subscribe"], source.BackendBindingBound, source.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Session"], source.BackendBindingBound, source.BackendSignatureAPI, "", false)
	assertBinding(t, bindings["List"], source.BackendBindingBound, source.BackendSignatureFragment, "", false)
}

func TestDiscoverGoEndpointCommentsBindsStandaloneEndpoints(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
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
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "home",
		Source: filepath.Join(root, "home.page.gwdk"),
		Route:  "/",
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{View: true, ViewBody: "<main>Home</main>"},
	}}}

	ir := app.program(gowdk.Config{})
	if err := DiscoverGoEndpoints(&ir); err != nil {
		t.Fatal(err)
	}
	if len(ir.GoEndpoints) != 2 {
		t.Fatalf("expected two Go comment endpoints, got %#v", ir.GoEndpoints)
	}
	if err := ValidateProgram(gowdk.Config{}, ir); err != nil {
		t.Fatal(err)
	}
	bindings := compilerBindingsByBlock(BindBackendHandlers(&ir))
	assertBinding(t, bindings["Login"], source.BackendBindingBound, source.BackendSignatureAction0, "", false)
	assertBinding(t, bindings["Session"], source.BackendBindingBound, source.BackendSignatureAPI, "", false)
}

func TestDiscoverGoEndpointCommentsRejectsMalformedComments(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "missing method",
			comment: "//gowdk:api /api/health",
			want:    "expected //gowdk:act METHOD /path or //gowdk:api METHOD /path",
		},
		{
			name:    "missing path",
			comment: "//gowdk:api GET",
			want:    "expected //gowdk:act METHOD /path or //gowdk:api METHOD /path",
		},
		{
			name:    "unknown kind",
			comment: "//gowdk:route GET /api/health",
			want:    "supported endpoint kinds are act and api",
		},
		{
			name:    "invalid method",
			comment: "//gowdk:api G3T /api/health",
			want:    "method must contain only ASCII letters",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeCompilerTestModule(t, root)
			writeCompilerTestFile(t, filepath.Join(root, "handlers.go"), `package api

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

`+test.comment+`
func Session(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}
`)
			app := appFixture{Pages: []gwdkir.Page{{
				ID:     "home",
				Source: filepath.Join(root, "home.page.gwdk"),
				Route:  "/",
				Guards: []string{"public"},
				Blocks: gwdkir.Blocks{View: true, ViewBody: "<main>Home</main>"},
			}}}

			ir := app.program(gowdk.Config{})
			err := DiscoverGoEndpoints(&ir)
			if err == nil {
				t.Fatal("expected malformed endpoint comment diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if len(diagnostics) != 1 || diagnostics[0].Code != "malformed_go_endpoint_comment" {
				t.Fatalf("unexpected diagnostics: %#v", diagnostics)
			}
			if !strings.Contains(diagnostics[0].Message, test.want) {
				t.Fatalf("expected diagnostic to contain %q, got %q", test.want, diagnostics[0].Message)
			}
			if diagnostics[0].Source != filepath.Join(root, "handlers.go") || diagnostics[0].Span.Start.Line == 0 {
				t.Fatalf("expected source span on malformed endpoint comment, got %#v", diagnostics[0])
			}
		})
	}
}

func TestDiscoverGoEndpointCommentsIgnoresUnrelatedComments(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	writeCompilerTestFile(t, filepath.Join(root, "handlers.go"), `package api

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

//gowdkx:api GET /api/ignored
//gowdk:api GET /api/session
func Session(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}
`)
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "home",
		Source: filepath.Join(root, "home.page.gwdk"),
		Route:  "/",
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{View: true, ViewBody: "<main>Home</main>"},
	}}}

	ir := app.program(gowdk.Config{})
	if err := DiscoverGoEndpoints(&ir); err != nil {
		t.Fatal(err)
	}
	if len(ir.GoEndpoints) != 1 || ir.GoEndpoints[0].Route != "/api/session" {
		t.Fatalf("expected only the valid GOWDK endpoint, got %#v", ir.GoEndpoints)
	}
}

func TestValidateManifestRejectsGoEndpointConflictWithGOWDKEndpoint(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:     "home",
			Source: sourcePath,
			Route:  "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: "<main>Home</main>",
				APIs:     []gwdkir.API{{Name: "Session", Method: "GET", Route: "/api/session"}},
			},
		}},
		Endpoints: []gwdkir.GoEndpoint{{
			Kind:       "api",
			SourceKind: gwdkir.EndpointSourceGo,
			Package:    "api",
			Source:     filepath.Join(root, "handlers.go"),
			Name:       "Session",
			Method:     "GET",
			Route:      "/api/session",
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected route conflict diagnostic")
	}
	if !hasDiagnosticCode(err.(ValidationErrors), "route_method_conflict") {
		t.Fatalf("missing route_method_conflict diagnostic: %#v", err)
	}
}

func TestValidateBackendBindingPolicyFailsProductionMissingHandler(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "login",
		Source: filepath.Join(t.TempDir(), "login.page.gwdk"),
		Route:  "/login",
		Blocks: gwdkir.Blocks{
			Actions: []gwdkir.Action{{Name: "Login", Method: "POST"}},
		},
	}}}

	err := validateBackendBindingPolicy(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, app)
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
	app := appFixture{BackendBindings: []source.BackendBinding{{
		Kind:         actionHandlerKind,
		PageID:       "login",
		BlockName:    "Login",
		Method:       "POST",
		Route:        "/login",
		FunctionName: "Login",
		Status:       source.BackendBindingMissing,
	}}}

	if err := validateBackendBindingPolicy(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected development missing handler to remain non-fatal, got %v", err)
	}
}

func TestValidateBackendBindingPolicyAllowsExplicitProductionStubMode(t *testing.T) {
	app := appFixture{BackendBindings: []source.BackendBinding{{
		Kind:         apiHandlerKind,
		PageID:       "session",
		BlockName:    "Session",
		Method:       "GET",
		Route:        "/api/session",
		FunctionName: "Session",
		Status:       source.BackendBindingUnsupportedSignature,
	}}}

	config := gowdk.Config{Build: gowdk.BuildConfig{
		Mode:                gowdk.Production,
		AllowMissingBackend: true,
	}}
	if err := validateBackendBindingPolicy(config, app); err != nil {
		t.Fatalf("expected explicit production stub mode to allow missing backend, got %v", err)
	}
}

func assertBinding(t *testing.T, binding source.BackendBinding, status source.BackendBindingStatus, signature source.BackendSignatureKind, inputType string, inputPointer bool) {
	t.Helper()
	if binding.Status != status || binding.Signature != signature || binding.InputType != inputType || binding.InputPointer != inputPointer {
		t.Fatalf("unexpected binding: %#v", binding)
	}
}

func assertInputFields(t *testing.T, fields []source.BackendInputField, expected string) {
	t.Helper()
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.FieldName+":"+field.FormName+":"+field.Type)
	}
	if strings.Join(parts, ",") != expected {
		t.Fatalf("unexpected input fields: %s", strings.Join(parts, ","))
	}
}

func compilerBindingsByBlock(bindings []source.BackendBinding) map[string]source.BackendBinding {
	out := map[string]source.BackendBinding{}
	for _, binding := range bindings {
		out[binding.BlockName] = binding
	}
	return out
}

func writeCompilerTestModule(t *testing.T, root string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	writeCompilerTestFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/app

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, filepath.ToSlash(repoRoot)))
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

// bindBackendHandlers routes a fixture through the production IR binding path
// and mirrors the records back onto the fixture shape these tests assert
// against.
func bindBackendHandlers(app appFixture) appFixture {
	ir := app.program(gowdk.Config{})
	bindings := BindBackendHandlers(&ir)
	app.BackendBindings = bindings
	loadBindings := map[string]source.BackendBinding{}
	for _, binding := range bindings {
		if binding.Kind == loadHandlerKind {
			loadBindings[binding.PageID] = binding
		}
	}
	for index := range app.Pages {
		if binding, ok := loadBindings[app.Pages[index].ID]; ok {
			app.Pages[index].LoadBinding = gwdkir.Binding{
				Status:       binding.Status,
				Message:      binding.Message,
				ImportPath:   binding.ImportPath,
				PackageName:  binding.PackageName,
				FunctionName: binding.FunctionName,
				Signature:    binding.Signature,
			}
		}
	}
	return app
}

func validateBackendBindingPolicy(config gowdk.Config, app appFixture) error {
	return ValidateBackendBindingPolicyIR(config, app.program(config))
}

func TestValidateBackendBindingPolicyIRSeesMissingLoadBinding(t *testing.T) {
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:          "dashboard",
			Source:      "dashboard.page.gwdk",
			Route:       "/dashboard",
			Blocks:      gwdkir.Blocks{Load: true},
			LoadBinding: gwdkir.Binding{Status: source.BackendBindingMissing},
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:    gwdkir.EndpointAction,
			PageID:  "dashboard",
			Symbol:  "Save",
			Method:  "POST",
			Path:    "/dashboard",
			Binding: gwdkir.Binding{Status: source.BackendBindingBound, FunctionName: "HandleSave"},
		}},
	}

	err := ValidateBackendBindingPolicyIR(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, ir)
	if err == nil {
		t.Fatal("expected missing load binding to fail the production policy even when endpoint bindings exist")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "backend_binding_required") {
		t.Fatalf("missing backend_binding_required diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(err.Error(), "load") {
		t.Fatalf("expected diagnostic to identify the load binding, got %v", err)
	}
}

func TestValidateBackendBindingPolicyIRFailsProductionUnsupportedFragmentBinding(t *testing.T) {
	ir := gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{{
			Kind:       gwdkir.EndpointFragment,
			PageID:     "dashboard",
			SourceFile: "dashboard.page.gwdk",
			Symbol:     "Stats",
			Method:     "GET",
			Path:       "/fragments/stats",
			Binding: gwdkir.Binding{
				Status:       source.BackendBindingUnsupportedSignature,
				FunctionName: "Stats",
				Message:      "GOWDK fragment handler app.Stats must have signature func(context.Context) (response.Response, error)",
			},
		}},
	}

	err := ValidateBackendBindingPolicyIR(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, ir)
	if err == nil {
		t.Fatal("expected unsupported fragment binding to fail the production policy")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "backend_binding_required") {
		t.Fatalf("missing backend_binding_required diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(err.Error(), "fragment handler Stats") {
		t.Fatalf("expected diagnostic to identify the fragment binding, got %v", err)
	}
}

func TestBackendBindingFromIRKeepsFragmentKind(t *testing.T) {
	binding := backendBindingFromIR(gwdkir.Endpoint{
		Kind:    gwdkir.EndpointFragment,
		PageID:  "dashboard",
		Symbol:  "Stats",
		Method:  "GET",
		Path:    "/fragments/stats",
		Binding: gwdkir.Binding{Status: source.BackendBindingBound, FunctionName: "RenderStats"},
	})
	if binding.Kind != "fragment" {
		t.Fatalf("expected fragment binding kind to survive IR conversion, got %q", binding.Kind)
	}
}
