package appgen

import (
	"bufio"
	"context"
	"flag"
	"go/ast"
	"go/format"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	authaddon "github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
	"github.com/cssbruno/gowdk/internal/source"
)

var updateGolden = flag.Bool("update", false, "update appgen golden files")

func csrfDisabledConfig() gowdk.Config {
	return gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{Disabled: true}}}
}

func withCSRFDisabled(config gowdk.Config) gowdk.Config {
	config.Build.CSRF.Disabled = true
	return config
}

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
		"application, err := gowdkapp.App()",
		"gowdkruntime.Run(context.Background(), server, application",
		"ReadHeaderTimeout: 5 * time.Second",
		"ReadTimeout: 10 * time.Second",
		"WriteTimeout: 30 * time.Second",
		"IdleTimeout: 60 * time.Second",
		"MaxHeaderBytes: 1 << 20",
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
		"func App() (*gowdkruntime.Application, error)",
		"func Handler() (http.Handler, error)",
		"func newServeMux(identity gowdkruntime.Identity) (*http.ServeMux, error)",
		"func ServeMux() (*http.ServeMux, error)",
		"func configuredServices() ([]gowdkruntime.Service, error)",
		"func RegisterMiddleware(middleware gowdkruntime.Middleware)",
		`gowdkruntime "github.com/cssbruno/gowdk/runtime/app"`,
		`mux.Handle("/", gowdkruntime.ApplyMiddlewares(&gowdkruntime.Handler{`,
		`Identity: identity,`,
		`Assets: gowdkruntime.LoadAssetManifest(root),`,
		`ErrorPages: gowdkruntime.LoadErrorPages(root),`,
		`Backend: backend,`,
		`SSRExact: ssrExact,`,
		`SSRDynamic: ssrDynamic,`,
		`RequestTimeout: gowdkruntime.DefaultRequestTimeout}`,
	} {
		if !strings.Contains(string(packagePayload), expected) {
			t.Fatalf("expected generated gowdkapp/app.go to contain %q:\n%s", expected, packagePayload)
		}
	}
	if strings.Contains(string(packagePayload), `github.com/cssbruno/gowdk/addons/ssr`) {
		t.Fatalf("static-only generated app should not import SSR helpers:\n%s", packagePayload)
	}
	assertNoOptionalGeneratedAppDependencies(t, modulePayload, mainPayload, packagePayload)
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

func TestGenerateWiresConfiguredLifecycleServices(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: gowdk.Config{
		Lifecycle: gowdk.LifecycleConfig{Services: []gowdk.ServiceRef{
			{ImportPath: "example.com/site/services", Function: "Services"},
			{ImportPath: "example.com/site/services", Function: "Workers"},
			{ImportPath: "example.com/site/metrics", Function: "Metrics"},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	appSource := string(payload)
	if strings.Contains(appSource, "example.com/site/services") || strings.Contains(appSource, "example.com/site/metrics") {
		t.Fatalf("lifecycle providers must stay out of app.go:\n%s", appSource)
	}
	lifecyclePayload, err := os.ReadFile(filepath.Join(result.AppDir, lifecycleFileName))
	if err != nil {
		t.Fatal(err)
	}
	source := string(lifecyclePayload)
	for _, expected := range []string{
		`//go:build !js`,
		`gowdkservice0 "example.com/site/services"`,
		`gowdkservice1 "example.com/site/metrics"`,
		`func configuredServices() ([]gowdkruntime.Service, error)`,
		`provided0, err := gowdkservice0.Services()`,
		`services = append(services, provided0...)`,
		`provided1, err := gowdkservice0.Workers()`,
		`services = append(services, provided1...)`,
		`provided2, err := gowdkservice1.Metrics()`,
		`services = append(services, provided2...)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated lifecycle source to contain %q:\n%s", expected, source)
		}
	}
	jsPayload, err := os.ReadFile(filepath.Join(result.AppDir, lifecycleJSName))
	if err != nil {
		t.Fatal(err)
	}
	jsSource := string(jsPayload)
	for _, expected := range []string{
		`//go:build js`,
		`func configuredServices() ([]gowdkruntime.Service, error)`,
		`return nil, nil`,
	} {
		if !strings.Contains(jsSource, expected) {
			t.Fatalf("expected generated lifecycle js source to contain %q:\n%s", expected, jsSource)
		}
	}
	if strings.Contains(jsSource, "example.com/site/services") || strings.Contains(jsSource, "example.com/site/metrics") {
		t.Fatalf("js lifecycle stub must not import providers:\n%s", jsSource)
	}
}

func TestGenerateLifecycleServiceAliasesReserveBackendImports(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{
			Lifecycle: gowdk.LifecycleConfig{Services: []gowdk.ServiceRef{{
				ImportPath: "example.com/site/services",
				Function:   "Services",
			}}},
		},
		APIs: []APIEndpoint{{
			PageID:  "status",
			APIName: "Status",
			Method:  http.MethodGet,
			Route:   "/api/status",
			Guards:  []string{"public"},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/site/gowdkservice0",
				PackageName:  "gowdkservice0",
				FunctionName: "Status",
				Signature:    source.BackendSignatureAPI,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	appPayload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appPayload), `gowdkservice0 "example.com/site/gowdkservice0"`) {
		t.Fatalf("expected backend import to keep gowdkservice0 alias:\n%s", appPayload)
	}
	lifecyclePayload, err := os.ReadFile(filepath.Join(result.AppDir, lifecycleFileName))
	if err != nil {
		t.Fatal(err)
	}
	source := string(lifecyclePayload)
	for _, expected := range []string{
		`gowdkservice1 "example.com/site/services"`,
		`provided0, err := gowdkservice1.Services()`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected lifecycle provider alias reservation %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `gowdkservice0 "example.com/site/services"`) {
		t.Fatalf("lifecycle provider reused backend alias:\n%s", source)
	}
}

func TestGenerateLifecycleServiceAppCompiles(t *testing.T) {
	root := t.TempDir()
	repoRoot, ok := gowdkRuntimeModuleRoot()
	if !ok {
		t.Fatal("could not locate GOWDK module root")
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+filepath.ToSlash(repoRoot)+`
`)
	writeTestFile(t, filepath.Join(root, "dist", "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(root, "services", "services.go"), `package services

import gowdkapp "github.com/cssbruno/gowdk/runtime/app"

func Services() ([]gowdkapp.Service, error) {
	return []gowdkapp.Service{gowdkapp.ServiceHooks{ServiceName: "noop"}}, nil
}
`)
	t.Chdir(root)

	result, err := GenerateWithOptions(filepath.Join(root, "dist"), filepath.Join(root, "generated-app"), Options{Config: gowdk.Config{
		Lifecycle: gowdk.LifecycleConfig{Services: []gowdk.ServiceRef{{
			ImportPath: "example.com/site/services",
			Function:   "Services",
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = result.AppDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("expected lifecycle generated app to compile: %v\n%s", err, output)
	}
}

func TestGenerateDeniedContractAndFragmentEndpointsCompileWithoutStaleImports(t *testing.T) {
	root := t.TempDir()
	repoRoot, ok := gowdkRuntimeModuleRoot()
	if !ok {
		t.Fatal("could not locate GOWDK module root")
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+filepath.ToSlash(repoRoot)+`
`)
	writeTestFile(t, filepath.Join(root, "dist", "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(root, "contracts", "patients", "patients.go"), `package patients

import "github.com/cssbruno/gowdk/runtime/contracts"

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct{}

func Register(registry *contracts.Registry) {}
`)
	writeTestFile(t, filepath.Join(root, "fragments", "fragments.go"), `package fragments

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/response"
)

func List(ctx context.Context) (response.Response, error) {
	return response.FragmentFor("#patients", "<section>Patients</section>"), nil
}
`)
	t.Chdir(root)

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "example.com/site/contracts/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
		Method:      http.MethodPost,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
	}}}
	result, err := GenerateWithOptions(filepath.Join(root, "dist"), filepath.Join(root, "generated-app"), Options{
		Config: csrfDisabledConfig(),
		IR:     program,
		Fragments: []FragmentEndpoint{
			{
				PageID:       "patients",
				FragmentName: "StaticList",
				Method:       http.MethodGet,
				Route:        "/patients/list",
				Target:       "#patients",
				HTML:         "<section>Patients</section>",
			},
			{
				PageID:       "patients",
				FragmentName: "BoundList",
				Method:       http.MethodGet,
				Route:        "/patients/bound-list",
				Target:       "#patients",
				HTML:         "<section>Fallback</section>",
				Binding: source.BackendBinding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/site/fragments",
					PackageName:  "fragments",
					FunctionName: "List",
					Signature:    source.BackendSignatureFragment,
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
	for _, unexpected := range []string{
		`gowdkform "github.com/cssbruno/gowdk/runtime/form"`,
		`gowdkpartial "github.com/cssbruno/gowdk/runtime/partial"`,
		`patients "example.com/site/contracts/patients"`,
		`fragments "example.com/site/fragments"`,
		`func decodeContract`,
		`gowdkform.DecodeExpected`,
		`fragments.List(ctx)`,
		`gowdkpartial.Fragment`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("guardless denied endpoints must not leave stale generated dependency %q:\n%s", unexpected, source)
		}
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = result.AppDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("expected generated app with denied endpoints to compile: %v\n%s", err, output)
	}
}

func assertNoOptionalGeneratedAppDependencies(t *testing.T, payloads ...[]byte) {
	t.Helper()

	forbidden := []string{
		"github.com/cssbruno/gowdk/addons/actions",
		"github.com/cssbruno/gowdk/addons/api",
		"github.com/cssbruno/gowdk/addons/partial",
		"github.com/cssbruno/gowdk/addons/ratelimit",
		"github.com/cssbruno/gowdk/addons/realtime",
		"github.com/cssbruno/gowdk/addons/ssr",
		"github.com/cssbruno/gowdk/addons/tailwind",
		"github.com/cssbruno/gowdk/runtime/adapters/chi",
		"github.com/cssbruno/gowdk/runtime/adapters/echo",
		"github.com/cssbruno/gowdk/runtime/adapters/fiber",
		"github.com/cssbruno/gowdk/runtime/adapters/gin",
		"github.com/cssbruno/gowdk/runtime/contracts/natsbroker",
		"github.com/cssbruno/gowdk/runtime/contracts/redisstream",
		"github.com/cssbruno/gowdk/runtime/contracts/websocketfanout",
		"github.com/coder/websocket",
		"github.com/gin-gonic/gin",
		"github.com/go-chi/chi/v5",
		"github.com/gofiber/fiber/v2",
		"github.com/labstack/echo/v5",
		"github.com/nats-io/nats.go",
		"github.com/redis/go-redis/v9",
	}

	for _, payload := range payloads {
		source := string(payload)
		for _, dependency := range forbidden {
			if strings.Contains(source, dependency) {
				t.Fatalf("simple generated app should not import optional dependency %q:\n%s", dependency, source)
			}
		}
	}
}

func TestGenerateWiresSecurityHeadersWhenConfigured(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{
			Build: gowdk.BuildConfig{
				SecurityHeaders: gowdk.SecurityHeadersConfig{
					Enabled: true,
					Headers: map[string]string{
						"Content-Security-Policy": "default-src 'self'",
						"X-Frame-Options":         "DENY",
						"x-frame-options":         "DENY",
					},
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
	for _, expected := range []string{
		`SecurityHeaders: map[string]string{`,
		`"Content-Security-Policy": "default-src 'self'",`,
		`"X-Frame-Options": "DENY"`,
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected generated app to contain %q:\n%s", expected, payload)
		}
	}
	if count := strings.Count(string(payload), `"X-Frame-Options": "DENY"`); count != 1 {
		t.Fatalf("expected generated app to canonicalize duplicate frame headers once, got %d:\n%s", count, payload)
	}
	if strings.Contains(string(payload), `"x-frame-options"`) {
		t.Fatalf("expected generated app to canonicalize lowercase frame header:\n%s", payload)
	}
}

func TestGenerateWritesAuditIntegrationTest(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{
			Env: gowdk.EnvConfig{
				Vars: []gowdk.EnvVar{
					{Name: "GOWDK_TEST_REGION", Required: true},
				},
				Secrets: []gowdk.SecretEnv{
					{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
				},
			},
			Build: gowdk.BuildConfig{
				SecurityHeaders: gowdk.SecurityHeadersConfig{
					Enabled: true,
					Headers: map[string]string{"X-Frame-Options": "DENY"},
				},
				CSRF: gowdk.CSRFConfig{SecretEnv: "GOWDK_TEST_CSRF_SECRET"},
			},
		},
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "home",
			ActionName: "Submit",
			Route:      "/submit",
		}},
		IR: &gwdkir.Program{
			Routes: []gwdkir.Route{{
				Kind:   gwdkir.RouteSPA,
				Method: "GET",
				Path:   "/",
				PageID: "home",
				Render: gowdk.SPA,
				Guards: []string{"public"},
			}},
			Endpoints: []gwdkir.Endpoint{{
				Kind:   gwdkir.EndpointAction,
				Method: http.MethodPost,
				Path:   "/submit",
				PageID: "home",
				Symbol: "Submit",
				Guards: []string{"public"},
				CSRF:   true,
			}},
			AuditSpecs: []gwdkir.AuditSpec{{
				Source: "security.audit.gwdk",
				Tests: []gwdkir.AuditTest{{
					Name: "home",
					Body: `expect GET "/" status 200`,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(result.AppDir, auditTestFileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"package gowdkapp",
		"func TestGOWDKAuditGeneratedSecurityPosture(t *testing.T)",
		`t.Setenv("GOWDK_TEST_CSRF_SECRET", "gowdk-audit-test-csrf-secret-32-bytes")`,
		`t.Setenv("GOWDK_TEST_DATABASE_URL", "gowdk-audit-test")`,
		`t.Setenv("GOWDK_TEST_REGION", "gowdk-audit-test")`,
		"handler, err := Handler()",
		`Name:       "route serves /"`,
		`WantStatus: http.StatusOK`,
		`Name:       "security header X-Frame-Options"`,
		`WantHeader: map[string]string{`,
		`"X-Frame-Options": "DENY"`,
		`Name:       "home GET /"`,
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected generated audit test to contain %q:\n%s", expected, payload)
		}
	}
}

func TestStandaloneAuditTestRejectsActorScenarios(t *testing.T) {
	specs := []gwdkir.AuditSpec{{
		Source: "security.audit.gwdk",
		Tests: []gwdkir.AuditTest{{
			Name: "admin",
			Body: `expect GET "/admin" as "role:admin" status 200`,
		}},
	}}
	// The standalone harness cannot enforce role/permission guards, so emitting
	// an actor scenario as a standalone test must fail loudly rather than
	// produce a test that passes or fails for the wrong reason.
	if _, err := StandaloneAuditTestSource(gowdk.Config{}, securitymanifest.SecurityManifest{}, specs); err == nil {
		t.Fatal("expected standalone audit emit to reject actor scenarios")
	}
	// The generated-app path runs against the real guard pipeline, so the same
	// actor scenario is supported there.
	source, err := GeneratedAuditTestSource(Options{
		SSR: []SSRRoute{{Route: "/admin", Guards: []string{"role:admin"}}},
		IR: &gwdkir.Program{
			Routes: []gwdkir.Route{{
				Kind:   gwdkir.RouteSSR,
				Method: "GET",
				Path:   "/admin",
				PageID: "admin",
				Render: gowdk.SSR,
				Guards: []string{"role:admin"},
			}},
			AuditSpecs: specs,
		},
	})
	if err != nil {
		t.Fatalf("expected generated-app actor test to succeed: %v", err)
	}
	if !strings.Contains(string(source), `"X-GOWDK-Audit-Actor": "role:admin"`) {
		t.Fatalf("expected generated-app actor scenario, got:\n%s", source)
	}
}

func TestStandaloneAuditTestRejectsEndpointExpectations(t *testing.T) {
	specs := []gwdkir.AuditSpec{{
		Source: "security.audit.gwdk",
		Tests: []gwdkir.AuditTest{{
			Name: "submit",
			Body: `expect POST "/submit" status 303`,
		}},
	}}
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{{
			ID:     "Submit",
			Kind:   "action",
			Method: http.MethodPost,
			Path:   "/submit",
		}},
	}
	if _, err := StandaloneAuditTestSource(gowdk.Config{}, manifest, specs); err == nil || !strings.Contains(err.Error(), `targets action endpoint Submit`) {
		t.Fatalf("expected standalone audit emit to reject action expectation, got %v", err)
	}
	if _, err := auditTestSource("gowdkapp", auditTestGeneratedApp, gowdk.Config{}, manifest, specs); err != nil {
		t.Fatalf("expected generated-app audit test to accept action expectation: %v", err)
	}
}

func TestStandaloneAuditTestRejectsDynamicEndpointExpectations(t *testing.T) {
	specs := []gwdkir.AuditSpec{{
		Source: "security.audit.gwdk",
		Tests: []gwdkir.AuditTest{{
			Name: "post",
			Body: `expect GET "/posts/42" status 200`,
		}},
	}}
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{{
			ID:     "Show",
			Kind:   "api",
			Method: http.MethodGet,
			Path:   "/posts/{id:int}",
		}},
	}
	if _, err := StandaloneAuditTestSource(gowdk.Config{}, manifest, specs); err == nil || !strings.Contains(err.Error(), `targets api endpoint Show`) {
		t.Fatalf("expected standalone audit emit to reject dynamic API expectation, got %v", err)
	}
}

func TestAuditSecurityHeadersCanonicalizesCaseVariants(t *testing.T) {
	headers := auditSecurityHeaders(gowdk.Config{Build: gowdk.BuildConfig{
		SecurityHeaders: gowdk.SecurityHeadersConfig{
			Enabled: true,
			Headers: map[string]string{
				" content-security-policy ": "default-src 'self'",
				"X-Frame-Options":           "DENY",
				"x-frame-options":           "DENY",
			},
		},
	}})
	if len(headers) != 2 {
		t.Fatalf("expected two canonical audit header expectations, got %#v", headers)
	}
	if headers[0].Name != "Content-Security-Policy" || headers[1].Name != "X-Frame-Options" {
		t.Fatalf("expected canonical sorted audit header names, got %#v", headers)
	}
}

func TestGeneratedAuditTestInstallsNativeRBACActorProvider(t *testing.T) {
	source, err := GeneratedAuditTestSource(Options{
		SSR: []SSRRoute{{
			Route:  "/admin",
			Guards: []string{"role:admin"},
		}},
		IR: &gwdkir.Program{
			Routes: []gwdkir.Route{{
				Kind:   gwdkir.RouteSSR,
				Method: "GET",
				Path:   "/admin",
				PageID: "admin",
				Render: gowdk.SSR,
				Guards: []string{"role:admin"},
			}},
			AuditSpecs: []gwdkir.AuditSpec{{
				Source: "security.audit.gwdk",
				Tests: []gwdkir.AuditTest{{
					Name: "admin",
					Body: `expect GET "/admin" as "role:admin" status 200`,
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := string(source)
	for _, expected := range []string{
		`gowdkauth "github.com/cssbruno/gowdk/runtime/auth"`,
		`"strings"`,
		`RegisterAuthProvider(gowdkauth.ProviderFunc`,
		`strings.TrimPrefix(actor, "role:")`,
		`"X-GOWDK-Audit-Actor": "role:admin"`,
	} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("expected generated audit test to contain %q:\n%s", expected, payload)
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
	writeTestFile(t, filepath.Join(outputDir, "source", "home.page.gwdk"), "page home")
	writeTestFile(t, filepath.Join(outputDir, "source", "main.go"), "package main")
	writeTestFile(t, filepath.Join(outputDir, "tmp", "asset.css"), "body{}")
	writeTestFile(t, filepath.Join(outputDir, "private", "notes.txt"), "private")
	writeTestFile(t, filepath.Join(outputDir, "secrets", "config.json"), `{"token":"secret"}`)
	writeTestFile(t, filepath.Join(outputDir, "keys", "server.key"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "server.pem"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "server-upper.PEM"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "bundle.p12"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "bundle-upper.P12"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "client.PFX"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "id_ed25519"), "private key")
	writeTestFile(t, filepath.Join(outputDir, "keys", "ID_RSA"), "private key")
	writeTestFile(t, filepath.Join(outputDir, ".npmrc"), "//registry.example/:_authToken=secret")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-security.json"), `{"endpoints":[{"path":"/admin"}]}`)
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
		filepath.Join(result.OutputDir, "private", "notes.txt"),
		filepath.Join(result.OutputDir, "secrets", "config.json"),
		filepath.Join(result.OutputDir, "gowdk-security.json"),
		filepath.Join(result.OutputDir, "keys", "server.key"),
		filepath.Join(result.OutputDir, "keys", "server.pem"),
		filepath.Join(result.OutputDir, "keys", "server-upper.PEM"),
		filepath.Join(result.OutputDir, "keys", "bundle.p12"),
		filepath.Join(result.OutputDir, "keys", "bundle-upper.P12"),
		filepath.Join(result.OutputDir, "keys", "client.PFX"),
		filepath.Join(result.OutputDir, "keys", "id_ed25519"),
		filepath.Join(result.OutputDir, "keys", "ID_RSA"),
		filepath.Join(result.OutputDir, ".npmrc"),
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
		Guards:           []string{"public"},
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
		`gowdkvalidation.MatchPattern("[a-z]+@[a-z]+[.][a-z]{2,4}", value)`,
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
			InputFields: []source.BackendInputField{
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
			Guards:    []string{"public"},
		},
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			InputFields: []source.BackendInputField{
				{FieldName: "Filter", FormName: "filter", Type: "string"},
			},
			Method:    "GET",
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "LoadPatientPage",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Guards:    []string{"public"},
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
		`context`,
		`gowdkcontracts "github.com/cssbruno/gowdk/runtime/contracts"`,
		`patients "example.com/app/contracts/patients"`,
		`sync`,
		`contractRegistry := gowdkcontracts.NewRegistry()`,
		`patients.Register(contractRegistry)`,
		`values[gowdkruntime.ServiceValueContractRegistry] = ContractRegistry()`,
		`contractRegistryOnce sync.Once`,
		`contractRegistry := ContractRegistry()`,
		`Kind: "command", Handler: commandPatientsCreatePatientPOSTPatients(contractRegistry)`,
		`Kind: "query", Handler: queryPatientsGetPatientPageGETPatients(contractRegistry)`,
		`var contractEventSink gowdkcontracts.CommandEventSink`,
		`func RegisterContractEventSink(sink gowdkcontracts.CommandEventSink)`,
		`contractEventSinkMu.Lock()`,
		`contractEventSinkMu.RLock()`,
		`func NewContractRegistry() *gowdkcontracts.Registry`,
		`func ContractRegistry() *gowdkcontracts.Registry`,
		`func RunContractEventWorker(ctx context.Context, source gowdkcontracts.EventSource) error`,
		`func RunContractEventWorkerWithOptions(ctx context.Context, source gowdkcontracts.EventSource, options ...gowdkcontracts.EventWorkerOption) error`,
		`func RunContractEventWorkerWithSeenStore(ctx context.Context, source gowdkcontracts.EventSource, seen gowdkcontracts.SeenStore) error`,
		`func RunContractEventWorkerWithSeenStoreAndOptions(ctx context.Context, source gowdkcontracts.EventSource, seen gowdkcontracts.SeenStore, options ...gowdkcontracts.EventWorkerOption) error`,
		`func commandPatientsCreatePatientPOSTPatients(contractRegistry *gowdkcontracts.Registry) gowdkruntime.BackendHandler`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`values := gowdkform.FromURLValues(request.PostForm)`,
		`input, err := decodeContractPatientsCreatePatientInput(values)`,
		`gowdkcontracts.CaptureCommandEventsForRole[patients.CreatePatient, patients.CreatePatientResult]`,
		`gowdkcontracts.DispatchCommandEvents(ctx, currentContractEventSink(), contractRegistry, gowdkcontracts.RoleWeb, events)`,
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
		`gowdkresponse.WriteNoStoreHandlerJSONError(response, err, http.StatusInternalServerError)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated contract app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `gowdkcontracts.ExecuteCommandForRole[patients.CreatePatient, patients.CreatePatientResult]`) {
		t.Fatalf("generated command contract must capture events instead of direct command execution:\n%s", source)
	}
	// Without query invalidations there is no realtimeQueryInvalidations symbol,
	// so the single-flight refresh header must not be emitted.
	if strings.Contains(source, `X-GOWDK-Queries`) {
		t.Fatalf("command without query invalidations must not set the X-GOWDK-Queries header:\n%s", source)
	}
}

func TestGenerateWritesRealtimeFanoutForSubscriptions(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		RealtimeSubscriptions: []gwdkir.RealtimeSubscription{{
			Query:           "patients.GetPatientPage",
			Event:           "patients.PatientNotice",
			EventImportPath: "example.com/app/contracts/patients",
			EventType:       "PatientNotice",
			Status:          gwdkir.ContractBindingBound,
			OwnerKind:       gwdkir.SourcePage,
			OwnerID:         "patients",
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`gowdkrealtime "github.com/cssbruno/gowdk/runtime/realtime"`,
		`const RealtimeEventsPath = "/_gowdk/realtime/events"`,
		`mux.Handle(RealtimeEventsPath, realtimeEventsHandler())`,
		`var realtimeFanout gowdkrealtime.PresentationFanout = gowdkrealtime.NewSSE()`,
		`func RegisterRealtimeFanout(fanout gowdkrealtime.PresentationFanout)`,
		`"example.com/app/contracts/patients.PatientNotice": true`,
		`event.Category == gowdkcontracts.PresentationEvent`,
		`gowdkcontracts.PresentationFanoutCommandEventSink(realtimeSubscriptionFanout{inner: fanout})`,
		`gowdkcontracts.CompositeCommandEventSink(gowdkcontracts.InProcessCommandEventSink(), fanoutSink)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated realtime app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesRealtimeQueryInvalidationFanout(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		QueryInvalidations: []gwdkir.QueryInvalidation{{
			Query:         "patients.GetPatientPage",
			QueryType:     "example.com/app/contracts/patients.GetPatientPage",
			Event:         "example.com/app/contracts/patients.PatientCreated",
			EventType:     "example.com/app/contracts/patients.PatientCreated",
			EventCategory: "domain",
			Status:        gwdkir.ContractBindingBound,
			OwnerKind:     gwdkir.SourcePage,
			OwnerID:       "patients",
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`gowdkcontracts.QueryInvalidationPresentationEventType: true`,
		`var realtimeQueryInvalidations []gowdkcontracts.QueryInvalidation = []gowdkcontracts.QueryInvalidation{gowdkcontracts.QueryInvalidation{EventCategory: gowdkcontracts.DomainEvent, EventType: "example.com/app/contracts/patients.PatientCreated", QueryType: "example.com/app/contracts/patients.GetPatientPage"}}`,
		`gowdkcontracts.QueryInvalidationCommandEventSink(fanout, realtimeQueryInvalidations)`,
		`gowdkcontracts.CompositeCommandEventSink(gowdkcontracts.InProcessCommandEventSink(), gowdkcontracts.QueryInvalidationCommandEventSink(fanout, realtimeQueryInvalidations), fanoutSink)`,
		// Single-flight write path: the command adapter tells the submitting
		// client which g:query regions to refresh via the X-GOWDK-Queries header.
		`invalidatedQueries := gowdkcontracts.InvalidatedQueryTypes(realtimeQueryInvalidations, events)`,
		`response.Header().Set("X-GOWDK-Queries", strings.Join(invalidatedQueries, ","))`,
		`invalidatedEventIDs := gowdkcontracts.InvalidatedEventIDs(realtimeQueryInvalidations, events)`,
		`response.Header().Set("X-GOWDK-Events", strings.Join(invalidatedEventIDs, ","))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated invalidation source to contain %q:\n%s", expected, source)
		}
	}
	for _, unexpected := range []string{
		`gowdkssr.RenderInvalidatedRegions`,
		`X-GOWDK-Patches`,
		`gowdkssr.CommandEnvelope`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("did not expect inline patch source without an eligible SSR region %q:\n%s", unexpected, source)
		}
	}
}

func TestGenerateRegistersSingleFlightRegionRenderers(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "board", "index.html"), "<main>Board</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		QueryInvalidations: []gwdkir.QueryInvalidation{{
			Query:         "patients.GetPatientPage",
			QueryType:     "example.com/app/board.GetPatientPage",
			Event:         "example.com/app/contracts/patients.PatientCreated",
			EventType:     "example.com/app/contracts/patients.PatientCreated",
			EventCategory: "domain",
			Status:        gwdkir.ContractBindingBound,
			OwnerKind:     gwdkir.SourcePage,
			OwnerID:       "patients",
		}},
	}
	ssrRoute := SSRRoute{
		PageID:  "board",
		Route:   "/board",
		Render:  gowdk.SSR,
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/board",
			PackageName:  "board",
			FunctionName: "LoadBoard",
			Signature:    source.BackendSignatureLoad,
		},
		HTML: `<main><section data-gowdk-query-type="example.com/app/board.GetPatientPage"><ul>__GOWDK_SSR_LIST_board__</ul></section></main>`,
		QueryRegions: []SSRQueryRegion{{
			QueryType: "example.com/app/board.GetPatientPage",
			Template:  `<section data-gowdk-query-type="example.com/app/board.GetPatientPage"><ul>__GOWDK_SSR_LIST_board__</ul></section>`,
			ListSpecs: []SSRListSpec{{Placeholder: "__GOWDK_SSR_LIST_board__", SourcePath: "patients"}},
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program, SSR: []SSRRoute{ssrRoute}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`func init() {`,
		`gowdkssr.RegisterRegion(gowdkssr.RegionRenderer{QueryType: "example.com/app/board.GetPatientPage"`,
		`Load: func(request *http.Request) (map[string]any, error)`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "board", Method: "GET", Path: "/board", Render: "ssr", Guards: []string{"public"}, HasLoad: true})`,
		`pageRequest.Method = http.MethodGet`,
		`loadContext := gowdkssr.NewLoadContext(request, nil)`,
		`if request.Header.Get("X-GOWDK-Command") == "1"`,
		`singleFlightPatches := gowdkssr.RenderInvalidatedRegions(request, invalidatedQueries)`,
		`response.Header().Set("X-GOWDK-Patches", "1")`,
		`gowdkssr.CommandEnvelope{Result: result, Patches: singleFlightPatches}`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated region registration to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateObservabilityTracesSSRRouteAndLoad(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "dashboard", "index.html"), "<main>Dashboard</main>")

	config := csrfDisabledConfig()
	config.Addons = []gowdk.Addon{gowdk.NewAddon("observability", gowdk.FeatureObservability)}
	ssrRoute := SSRRoute{
		PageID:     "dashboard",
		Route:      "/dashboard",
		Render:     gowdk.SSR,
		Guards:     []string{"public"},
		HasLoad:    true,
		Source:     "dashboard.page.gwdk",
		SourceSpan: source.SourceSpan{Start: source.SourcePosition{Line: 3, Column: 1}},
		LoadBinding: source.BackendBinding{
			Kind:         "load",
			PageID:       "dashboard",
			Source:       "dashboard.page.gwdk",
			Span:         source.SourceSpan{Start: source.SourcePosition{Line: 6, Column: 1}},
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadError,
		},
		HTML: "<main>Dashboard</main>",
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: config, SSR: []SSRRoute{ssrRoute}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`gowdktrace "github.com/cssbruno/gowdk/runtime/trace"`,
		`Tracer: traceTracer,`,
		`ctx, ssrSpan := gowdktrace.Start(request.Context(), "ssr /dashboard"`,
		`gowdktrace.WithLane(gowdktrace.LaneSSR)`,
		`gowdktrace.WithSource(gowdktrace.SourceRef{File: "dashboard.page.gwdk", Line: 3, Column: 1, OwnerKind: "page", OwnerID: "dashboard"})`,
		`gowdktrace.AttrHTTPRoute: "/dashboard"`,
		`defer gowdkruntime.FinishHTTPTrace(response, ssrSpan)`,
		`ctx, loadSpan := gowdktrace.Start(request.Context(), "load /dashboard"`,
		`loadRequest := request.WithContext(ctx)`,
		`loadContext := gowdkssr.NewLoadContext(loadRequest, nil)`,
		`loadData, err := dashboard.LoadDashboard(loadContext)`,
		`gowdkruntime.FinishTrace(loadSpan, err)`,
		`gowdkruntime.FinishTrace(loadSpan, nil)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated observability source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateSkipsSingleFlightRegionRenderersForGuardedRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "dashboard", "index.html"), "<main>Dashboard</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		QueryInvalidations: []gwdkir.QueryInvalidation{{
			Query:         "patients.GetPatientPage",
			QueryType:     "example.com/app/dashboard.GetPatientPage",
			Event:         "example.com/app/contracts/patients.PatientCreated",
			EventType:     "example.com/app/contracts/patients.PatientCreated",
			EventCategory: "domain",
			Status:        gwdkir.ContractBindingBound,
			OwnerKind:     gwdkir.SourcePage,
			OwnerID:       "patients",
		}},
	}
	ssrRoute := SSRRoute{
		PageID:  "dashboard",
		Route:   "/dashboard",
		Render:  gowdk.SSR,
		Guards:  []string{"auth.required"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoad,
		},
		HTML: `<main><section data-gowdk-query-type="example.com/app/dashboard.GetPatientPage"></section></main>`,
		QueryRegions: []SSRQueryRegion{{
			QueryType: "example.com/app/dashboard.GetPatientPage",
			Template:  `<section data-gowdk-query-type="example.com/app/dashboard.GetPatientPage"></section>`,
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program, SSR: []SSRRoute{ssrRoute}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, unexpected := range []string{
		`gowdkssr.RegisterRegion`,
		`gowdkssr.RenderInvalidatedRegions`,
		`X-GOWDK-Patches`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("guarded SSR route must not emit inline region patch code %q:\n%s", unexpected, source)
		}
	}
	if !strings.Contains(source, `response.Header().Set("X-GOWDK-Queries", strings.Join(invalidatedQueries, ","))`) {
		t.Fatalf("guarded route should keep invalidation header fallback:\n%s", source)
	}
}

func TestGenerateGuardsRealtimeStreamForSubscribedPages(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "dashboard", "index.html"), "<main>Dashboard</main>")

	program := &gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required"},
		}},
		RealtimeSubscriptions: []gwdkir.RealtimeSubscription{{
			Query:           "patients.GetPatientPage",
			Event:           "patients.PatientNotice",
			EventImportPath: "example.com/app/contracts/patients",
			EventType:       "PatientNotice",
			Guards:          []string{"auth.required"},
			Status:          gwdkir.ContractBindingBound,
			OwnerKind:       gwdkir.SourcePage,
			OwnerID:         "dashboard",
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`neturl "net/url"`,
		`gowdkroute "github.com/cssbruno/gowdk/runtime/route"`,
		`func realtimeStreamGuards(request *http.Request) []string`,
		`request.URL.Query().Get("path")`,
		`referer := request.Referer()`,
		`neturl.Parse(referer)`,
		`gowdkroute.Match("/dashboard", requestPath)`,
		`return []string{"auth.required"}`,
		`if !runGuards(response, request, realtimeStreamGuards(request))`,
		`RegisterGuards(GOWDKGuardRegistry())`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated guarded realtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestBoundActionFieldDecodePanicsOnUnsupportedFieldType(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected unsupported backend input field type panic")
		}
		message, ok := recovered.(string)
		if !ok || !strings.Contains(message, `unsupported backend input field type "float64"`) {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	_ = boundActionFieldDecodeStmts(0, source.BackendInputField{
		FieldName: "Amount",
		FormName:  "amount",
		Type:      "float64",
	})
}

func TestGenerateWritesDerivedCommandContractBackendRoute(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := gwdkanalysis.BuildProgram(gowdk.Config{}, gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Guards:  []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><form g:command="patients.CreatePatient"></form></main>`,
		},
	}}})

	result, err := GenerateWithOptions(outputDir, appDir, Options{IR: &program})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`Kind: "command", Handler: commandPatientsCreatePatientPOSTPatients`,
		`func commandPatientsCreatePatientPOSTPatients(response http.ResponseWriter, request *http.Request) bool`,
		`GOWDK command contract is not implemented`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated derived command route source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGeneratedGoMatchesGoldenFixture(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []source.BackendInputField{
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
			Guards:    []string{"public"},
		}, {
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			InputFields: []source.BackendInputField{
				{FieldName: "Filter", FormName: "filter", Type: "string"},
			},
			Method:    "GET",
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "LoadPatientPage",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Guards:    []string{"public"},
		},
		},
	}
	actions := []ActionEndpoint{{
		Guards:      []string{"public"},
		PageID:      "newsletter",
		ActionName:  "Subscribe",
		Method:      "POST",
		Route:       "/newsletter",
		InputFields: []string{"email", "tag", "age", "remember"},
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/newsletter",
			PackageName:  "newsletter",
			FunctionName: "Subscribe",
			Signature:    source.BackendSignatureActionForm,
			InputType:    "SubscribeInput",
			InputFields: []source.BackendInputField{
				{FieldName: "Email", FormName: "email", Type: "string"},
				{FieldName: "Tags", FormName: "tag", Type: "[]string"},
				{FieldName: "Age", FormName: "age", Type: "int"},
				{FieldName: "Remember", FormName: "remember", Type: "bool"},
			},
		},
	}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: actions, IR: program})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}

	goldenPath := filepath.FromSlash("testdata/generated_go_golden/app.go.golden")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Fatalf("generated Go golden mismatch (run go test ./internal/appgen -run TestGeneratedGoMatchesGoldenFixture -update if intentional)\nexpected:\n%s\nactual:\n%s", expected, actual)
	}
}

func TestGenerateBackendAppRegistersBackendRoutes(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "generated-backend")

	result, err := GenerateBackendWithOptions(appDir, Options{
		Config: gowdk.Config{Env: gowdk.EnvConfig{
			Secrets: []gowdk.SecretEnv{
				{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
			},
		}},
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Route:      "/newsletter",
			Redirect:   "/newsletter?ok=1",
		}},
		Fragments: []FragmentEndpoint{{
			Guards:       []string{"public"},
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
		`"errors"`,
		`gowdkruntime "github.com/cssbruno/gowdk/runtime/app"`,
		`"os"`,
		`strings`,
		`func RegisterMiddleware(middleware gowdkruntime.Middleware)`,
		`if err := validateEnvContract(); err != nil {`,
		`backendRouter, err := newBackendRouter()`,
		`mux.Handle("/", gowdkruntime.ApplyMiddlewares(backendRouter, registeredMiddlewares()...))`,
		`func validateEnvContract() error`,
		`value := os.Getenv("GOWDK_TEST_DATABASE_URL")`,
		`missing = append(missing, "GOWDK_TEST_DATABASE_URL is required but is not set")`,
		`func newBackendRouter() (*gowdkruntime.BackendRouter, error)`,
		`gowdkpartial "github.com/cssbruno/gowdk/runtime/partial"`,
		`gowdkruntime.BackendRoute{Method: http.MethodPost, Path: "/newsletter", Kind: "action", Handler: action}`,
		`gowdkruntime.BackendRoute{Method: http.MethodGet, Path: "/patients/list", Kind: "fragment", Handler: fragment}`,
		`func fragment(response http.ResponseWriter, request *http.Request) bool`,
		`gowdkpartial.Fragment("#patients", "<section>Patients</section>")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated backend app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `github.com/cssbruno/gowdk/addons/ssr`) {
		t.Fatalf("fragment/action backend output should not import SSR helpers:\n%s", source)
	}
	if strings.Contains(source, `func backend(response http.ResponseWriter, request *http.Request) bool`) {
		t.Fatalf("expected backend-only app to use BackendRouter instead of generated backend dispatcher:\n%s", source)
	}
}

func TestGenerateRenamesBackendAliasReservedByGeneratedRuntime(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		PageID:     "newsletter",
		ActionName: "Subscribe",
		Method:     "POST",
		Route:      "/newsletter",
		Guards:     []string{"public"},
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/sync",
			PackageName:  "sync",
			FunctionName: "Subscribe",
			Signature:    source.BackendSignatureAction0,
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	frontendSource := string(payload)
	for _, expected := range []string{
		`sync2 "example.com/app/sync"`,
		`"sync"`,
		`result, err := sync2.Subscribe(ctx)`,
		`middlewareMu sync.RWMutex`,
	} {
		if !strings.Contains(frontendSource, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, frontendSource)
		}
	}
	if strings.Contains(frontendSource, `sync "example.com/app/sync"`) {
		t.Fatalf("backend import must not overwrite generated sync import:\n%s", frontendSource)
	}

	backendResult, err := GenerateBackendWithOptions(filepath.Join(root, "generated-backend"), Options{APIs: []APIEndpoint{{
		PageID:  "status",
		APIName: "Health",
		Method:  "GET",
		Route:   "/api/health",
		Guards:  []string{"public"},
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/sync",
			PackageName:  "sync",
			FunctionName: "Health",
			Signature:    source.BackendSignatureAPI,
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	backendPayload, err := os.ReadFile(backendResult.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	backendSource := string(backendPayload)
	for _, expected := range []string{
		`sync2 "example.com/app/sync"`,
		`"sync"`,
		`result, err := sync2.Health(ctx, request)`,
		`middlewareMu sync.RWMutex`,
	} {
		if !strings.Contains(backendSource, expected) {
			t.Fatalf("expected generated backend app source to contain %q:\n%s", expected, backendSource)
		}
	}
	if strings.Contains(backendSource, `sync "example.com/app/sync"`) {
		t.Fatalf("backend-only import must not overwrite generated sync import:\n%s", backendSource)
	}
}

func TestGenerateBackendAppWiresSecurityHeaders(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "generated-backend")

	result, err := GenerateBackendWithOptions(appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{
			SecurityHeaders: gowdk.SecurityHeadersConfig{
				Enabled: true,
				Headers: map[string]string{"X-Frame-Options": "DENY"},
			},
		}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "status",
			APIName: "Health",
			Method:  "GET",
			Route:   "/api/health",
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
		`"strings"`,
		`mux.Handle("/", gowdkruntime.ApplyMiddlewares(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {`,
		`for name, value := range map[string]string{"X-Frame-Options": "DENY"} {`,
		`if strings.TrimSpace(name) == "" {`,
		`response.Header().Set(name, value)`,
		`backendRouter.ServeHTTP(response, request)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated backend app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `mux.Handle("/", backendRouter)`) {
		t.Fatalf("backend-only app with configured security headers should wrap the router:\n%s", source)
	}
}

func TestGenerateWiresCORSForAPIRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "status", "index.html"), "<main>Status</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CORS: gowdk.CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"https://app.example"},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost},
			AllowedHeaders:   []string{"Content-Type", "X-CSRF"},
			ExposedHeaders:   []string{"X-Total-Count"},
			AllowCredentials: true,
			MaxAgeSeconds:    600,
		}}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "status",
			APIName: "Health",
			Method:  http.MethodGet,
			Route:   "/api/health",
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
		`backendRouter, err := gowdkruntime.NewBackendRouter(gowdkruntime.BackendRoute{Method: "GET", Path: "/api/health", Kind: "api", Handler: api})`,
		`if err := backendRouter.SetCORSPolicy(gowdkruntime.CORSPolicy{AllowedOrigins: []string{"https://app.example"}, AllowedMethods: []string{"GET", "POST"}, AllowedHeaders: []string{"Content-Type", "X-CSRF"}, ExposedHeaders: []string{"X-Total-Count"}, AllowCredentials: true, MaxAgeSeconds: 600}); err != nil {`,
		`return backendRouter, nil`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateRejectsInvalidCORSConfig(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "status", "index.html"), "<main>Status</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CORS: gowdk.CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"*"},
			AllowCredentials: true,
		}}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "status",
			APIName: "Health",
			Method:  http.MethodGet,
			Route:   "/api/health",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "wildcard origin") {
		t.Fatalf("expected invalid CORS config error, got %v", err)
	}
}

func TestGenerateSplitFrontendProxyMatchesAdapterRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		ProxyBackend: true,
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
		}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "status",
			APIName: "Health",
			Method:  "GET",
			Route:   "/api/health",
		}},
		Fragments: []FragmentEndpoint{{
			Guards:       []string{"public"},
			PageID:       "patients",
			FragmentName: "List",
			Method:       "GET",
			Route:        "/patients/{id}/list",
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
		`Backend: backendProxy,`,
		`func isBackendRoute(method string, requestPath string) bool`,
		`method == http.MethodPost && requestPath == "/newsletter"`,
		`method == "GET" && requestPath == "/api/health"`,
		`rawRequestPath := requestPath`,
		`gowdkroute.Match("/patients/{id}/list", rawRequestPath)`,
		`"net/http/httputil"`,
		`neturl "net/url"`,
		`gowdkroute "github.com/cssbruno/gowdk/runtime/route"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated split frontend source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `func newBackendRouter()`) {
		t.Fatalf("split frontend proxy should not build a local backend router:\n%s", source)
	}
}

func TestGenerateSplitFrontendProxyKeepsBackendAdaptersRemote(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), `<main><form method="post" action="/patients"></form></main>`)

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "gowdk-generated-app/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		Method:      http.MethodPost,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
	}}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		ProxyBackend: true,
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     http.MethodPost,
			Route:      "/newsletter",
		}},
		IR: program,
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
		`Backend: backendProxy,`,
		`CSRF: csrfTokenSource,`,
		`func newCSRF() (*gowdkactions.CSRF, error)`,
		`func isBackendRoute(method string, requestPath string) bool`,
		`method == http.MethodPost && requestPath == "/newsletter"`,
		`method == http.MethodPost && requestPath == "/patients"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated split frontend source to contain %q:\n%s", expected, source)
		}
	}
	for _, unexpected := range []string{
		`func newBackendRouter()`,
		`func commandPatientsCreatePatientPOSTPatients`,
		`NewContractRegistry`,
		`patients.CreatePatient`,
		`gowdkcontracts`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("split frontend proxy should not emit local backend contract code %q:\n%s", unexpected, source)
		}
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateWiresCSRFByDefault(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), `<main><form method="post" action="/newsletter"><input name="email"></form></main>`)

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv:  "GOWDK_TEST_CSRF_SECRET",
			CookieName: "csrf",
			FieldName:  "_csrf",
			HeaderName: "X-CSRF",
			Insecure:   true,
		}}},
		Actions: []ActionEndpoint{{
			Guards:      []string{"public"},
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
		`gowdkactions "github.com/cssbruno/gowdk/runtime/actions"`,
		`CSRF: csrfTokenSource,`,
		`var csrfTokenSource *gowdkactions.CSRF`,
		`var csrfErr error`,
		`csrfTokenSource, csrfErr = newCSRF()`,
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

func TestGenerateWiresCSRFForStateChangingAPIs(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "status", "index.html"), "<main>Status</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv:  "GOWDK_TEST_CSRF_SECRET",
			CookieName: "csrf",
			FieldName:  "_csrf",
			HeaderName: "X-CSRF",
			Insecure:   true,
		}}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "status",
			APIName: "Update",
			Method:  http.MethodPost,
			Route:   "/api/status",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/status",
				PackageName:  "status",
				FunctionName: "Update",
				Signature:    source.BackendSignatureAPI,
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
		`gowdkactions "github.com/cssbruno/gowdk/runtime/actions"`,
		`CSRF: csrfTokenSource,`,
		`var csrfTokenSource *gowdkactions.CSRF`,
		`var csrfValidator gowdkactions.CSRFValidator`,
		`case request.Method == "POST" && requestPath == "/api/status":`,
		`if csrfValidator != nil {`,
		`err := csrfValidator.Validate(request)`,
		`gowdkresponse.WriteNoStoreJSONError(response, http.StatusForbidden, "invalid csrf token")`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxAPIBodyBytes)`,
		`result, err := status.Update(ctx, request)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated state-changing API CSRF source to contain %q:\n%s", expected, source)
		}
	}
	apiIndex := strings.Index(source, `case request.Method == "POST" && requestPath == "/api/status":`)
	if apiIndex < 0 {
		t.Fatalf("expected generated source to contain API case:\n%s", source)
	}
	assertSourceOrder(t, source[apiIndex:],
		`request.Body = http.MaxBytesReader(response, request.Body, maxAPIBodyBytes)`,
		`err := csrfValidator.Validate(request)`,
		`result, err := status.Update(ctx, request)`,
	)
}

func TestGenerateSkipsCSRFWhenDisabled(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), `<main><form method="post" action="/newsletter"><input name="email"></form></main>`)

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: csrfDisabledConfig(),
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
	for _, unexpected := range []string{
		`CSRF: csrfTokenSource,`,
		`csrfTokenSource, csrfErr = newCSRF()`,
		`func newCSRF() (*gowdkactions.CSRF, error)`,
		`err := csrfValidator.Validate(request)`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("disabled CSRF should not emit %q:\n%s", unexpected, source)
		}
	}
}

func TestGenerateWiresCSRFForCommandContracts(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), `<main><form method="post" action="/patients" g:command="patients.CreatePatient"></form></main>`)

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "example.com/app/contracts/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		Method:      "POST",
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
		Guards:      []string{"public"},
	}}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv:  "GOWDK_TEST_CSRF_SECRET",
			CookieName: "csrf",
			FieldName:  "_csrf",
			HeaderName: "X-CSRF",
			Insecure:   true,
		}}},
		IR: program,
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
		`gowdkactions "github.com/cssbruno/gowdk/runtime/actions"`,
		`CSRF: csrfTokenSource,`,
		`var csrfTokenSource *gowdkactions.CSRF`,
		`var csrfErr error`,
		`csrfTokenSource, csrfErr = newCSRF()`,
		`csrfValidator = csrfTokenSource`,
		`var csrfValidator gowdkactions.CSRFValidator`,
		`func commandPatientsCreatePatientPOSTPatients(contractRegistry *gowdkcontracts.Registry) gowdkruntime.BackendHandler`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`if err := request.ParseForm(); err != nil`,
		`gowdkresponse.WriteNoStoreJSONError(response, http.StatusRequestEntityTooLarge, "request body too large")`,
		`gowdkresponse.WriteNoStoreJSONError(response, http.StatusBadRequest, "invalid form")`,
		`if csrfValidator != nil {`,
		`err := csrfValidator.Validate(request)`,
		`gowdkresponse.WriteNoStoreJSONError(response, http.StatusForbidden, "invalid csrf token")`,
		`input := patients.CreatePatient{}`,
		`gowdkcontracts.CaptureCommandEventsForRole[patients.CreatePatient, patients.CreatePatientResult]`,
		`gowdkcontracts.DispatchCommandEvents(ctx, currentContractEventSink(), contractRegistry, gowdkcontracts.RoleWeb, events)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated command contract CSRF source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `Kind: "action", Handler: action`) {
		t.Fatalf("did not expect a classic action route for contract-only CSRF app:\n%s", source)
	}
	commandIndex := strings.Index(source, `ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "command"`)
	if commandIndex < 0 {
		t.Fatalf("expected generated source to contain command contract endpoint context:\n%s", source)
	}
	assertSourceOrder(t, source[commandIndex:],
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`if err := request.ParseForm(); err != nil`,
		`err := csrfValidator.Validate(request)`,
		`input := patients.CreatePatient{}`,
		`gowdkcontracts.CaptureCommandEventsForRole[patients.CreatePatient, patients.CreatePatientResult]`,
		`gowdkcontracts.DispatchCommandEvents(ctx, currentContractEventSink(), contractRegistry, gowdkcontracts.RoleWeb, events)`,
	)
}

func TestGenerateWiresEnvContractValidation(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Env: gowdk.EnvConfig{
			Vars: []gowdk.EnvVar{
				{Name: "GOWDK_TEST_BACKEND_ORIGIN", Required: true},
				{Name: "GOWDK_TEST_ADDR", Required: true, Default: "127.0.0.1:8080"},
			},
			Secrets: []gowdk.SecretEnv{
				{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
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
		`errors`,
		`gowdkenvfile "github.com/cssbruno/gowdk/runtime/envfile"`,
		`os`,
		`strings`,
		`if err := loadEnvFile(); err != nil {`,
		`func loadEnvFile() error`,
		`explicit := strings.TrimSpace(os.Getenv("GOWDK_ENV_FILE"))`,
		`path, _, err := gowdkenvfile.LookupPath("", explicit)`,
		`_, err = gowdkenvfile.LoadIntoEnv(path, explicit != "")`,
		`applyEnvDefaults()`,
		`func applyEnvDefaults()`,
		`os.Setenv("GOWDK_TEST_ADDR", "127.0.0.1:8080")`,
		`if err := validateEnvContract(); err != nil {`,
		`func validateEnvContract() error`,
		`value := os.Getenv("GOWDK_TEST_BACKEND_ORIGIN")`,
		`missing = append(missing, "GOWDK_TEST_BACKEND_ORIGIN is required but is not set")`,
		`value := os.Getenv("GOWDK_TEST_DATABASE_URL")`,
		`missing = append(missing, "GOWDK_TEST_DATABASE_URL is required but is not set")`,
		`return errors.New(strings.Join(missing, "\n"))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `GOWDK_TEST_ADDR is required but is not set`) {
		t.Fatalf("required env var with a default should not need runtime validation:\n%s", source)
	}
	muxIndex := strings.Index(source, `func newServeMux(identity gowdkruntime.Identity) (*http.ServeMux, error)`)
	if muxIndex < 0 {
		t.Fatalf("expected generated source to contain newServeMux:\n%s", source)
	}
	assertSourceOrder(t, source[muxIndex:],
		`if err := loadEnvFile(); err != nil {`,
		`applyEnvDefaults()`,
		`if err := validateEnvContract(); err != nil {`,
	)
}

func TestGenerateLoadsEnvFileForOptionalEnvConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		vars          []gowdk.EnvVar
		expectDefault bool
	}{
		{
			name: "optional only",
			vars: []gowdk.EnvVar{{Name: "GOWDK_TEST_OPTIONAL_ORIGIN"}},
		},
		{
			name:          "default only",
			vars:          []gowdk.EnvVar{{Name: "GOWDK_TEST_ADDR", Default: "127.0.0.1:8080"}},
			expectDefault: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			outputDir := filepath.Join(root, "dist")
			appDir := filepath.Join(root, "generated-app")
			writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

			result, err := GenerateWithOptions(outputDir, appDir, Options{
				Config: gowdk.Config{Env: gowdk.EnvConfig{Vars: tc.vars}},
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
				`gowdkenvfile "github.com/cssbruno/gowdk/runtime/envfile"`,
				`if err := loadEnvFile(); err != nil {`,
				`func loadEnvFile() error`,
				`_, err = gowdkenvfile.LoadIntoEnv(path, explicit != "")`,
			} {
				if !strings.Contains(source, expected) {
					t.Fatalf("expected optional env config to load .env via %q:\n%s", expected, source)
				}
			}
			if strings.Contains(source, `func validateEnvContract() error`) {
				t.Fatalf("optional-only env config should not emit required validation:\n%s", source)
			}
			if tc.expectDefault {
				for _, expected := range []string{
					`applyEnvDefaults()`,
					`func applyEnvDefaults()`,
					`os.Setenv("GOWDK_TEST_ADDR", "127.0.0.1:8080")`,
				} {
					if !strings.Contains(source, expected) {
						t.Fatalf("expected default-only env config to emit %q:\n%s", expected, source)
					}
				}
			} else if strings.Contains(source, `func applyEnvDefaults()`) {
				t.Fatalf("optional env config without defaults should not emit defaults helper:\n%s", source)
			}
		})
	}
}

func TestGenerateEnvContractEnforcesSecretMinBytes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Env: gowdk.EnvConfig{
			Secrets: []gowdk.SecretEnv{
				{Name: "GOWDK_TEST_SESSION_SECRET", Required: true, MinBytes: 32},
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
		`value := strings.TrimSpace(os.Getenv("GOWDK_TEST_SESSION_SECRET"))`,
		`missing = append(missing, "GOWDK_TEST_SESSION_SECRET is required but is not set")`,
		`} else if len(value) < 32 {`,
		`missing = append(missing, "GOWDK_TEST_SESSION_SECRET must be at least 32 bytes")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated env contract to enforce the secret minimum %q:\n%s", expected, source)
		}
	}
}

func TestGenerateRunsRateLimitAndGuardsBeforeContractExecution(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), `<main>Patients</main>`)

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{
		{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Guards:      []string{"auth.required"},
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
			Method:      "POST",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
		},
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			Guards:      []string{"auth.required"},
			InputFields: []source.BackendInputField{{FieldName: "Filter", FormName: "filter", Type: "string"}},
			Method:      "GET",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "LoadPatientPage",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
		},
	}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ratelimit", gowdk.FeatureRateLimit)}},
		IR:     program,
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
		`gowdkratelimit "github.com/cssbruno/gowdk/runtime/ratelimit"`,
		`gowdkauth "github.com/cssbruno/gowdk/runtime/auth"`,
		`gowdkguard "github.com/cssbruno/gowdk/runtime/guard"`,
		`func RegisterRateLimiter(limiter *gowdkratelimit.Limiter)`,
		`func RegisterGuards(registry gowdkguard.Registry)`,
		`func RegisterAuthProvider(provider gowdkauth.Provider)`,
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected guarded/rate-limited contract source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `github.com/cssbruno/gowdk/addons/ssr`) {
		t.Fatalf("guarded contract endpoints should not import SSR helpers:\n%s", source)
	}
	commandIndex := strings.Index(source, `ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "command"`)
	if commandIndex < 0 {
		t.Fatalf("expected generated source to contain command contract endpoint context:\n%s", source)
	}
	assertSourceOrder(t, source[commandIndex:],
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`values := gowdkform.FromURLValues(request.PostForm)`,
		`input, err := decodeContractPatientsCreatePatientInput(values)`,
		`gowdkcontracts.CaptureCommandEventsForRole[patients.CreatePatient, patients.CreatePatientResult]`,
		`gowdkcontracts.DispatchCommandEvents(ctx, currentContractEventSink(), contractRegistry, gowdkcontracts.RoleWeb, events)`,
	)
	queryIndex := strings.Index(source, `ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "query"`)
	if queryIndex < 0 {
		t.Fatalf("expected generated source to contain query contract endpoint context:\n%s", source)
	}
	assertSourceOrder(t, source[queryIndex:],
		`if runRateLimit(response, request)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
		`values := gowdkform.FromURLValues(request.URL.Query())`,
		`input, err := decodeContractPatientsGetPatientPageInput(values)`,
		`gowdkcontracts.ExecuteQueryForRole[patients.GetPatientPage, patients.PatientPageData]`,
	)
}

func TestGenerateWritesTypedBoundActionHandlers(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "Login", "index.html"), "<main>Login</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{
		{
			Guards:      []string{"public"},
			PageID:      "Login",
			ActionName:  "Login",
			Route:       "/Login",
			Redirect:    "/dashboard",
			Fragments:   []ActionFragment{{Target: "#login", HTML: "<p>ignored</p>"}},
			InputFields: []string{"email"},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Login",
				Signature:    source.BackendSignatureActionForm,
				InputType:    "LoginInput",
				InputFields: []source.BackendInputField{
					{FieldName: "Email", FormName: "email", Type: "string"},
					{FieldName: "Tags", FormName: "tag", Type: "[]string"},
					{FieldName: "Age", FormName: "age", Type: "int"},
					{FieldName: "Remember", FormName: "remember", Type: "bool"},
					{FieldName: "Code", FormName: "code", Type: "byte"},
					{FieldName: "Letter", FormName: "letter", Type: "rune"},
				},
			},
		},
		{
			Guards:      []string{"public"},
			PageID:      "Login",
			ActionName:  "save",
			Route:       "/Login/save",
			InputFields: []string{"email"},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Save",
				Signature:    source.BackendSignatureActionFormPtr,
				InputType:    "LoginInput",
				InputPointer: true,
				InputFields: []source.BackendInputField{
					{FieldName: "Email", FormName: "email", Type: "string"},
				},
			},
		},
		{
			Guards:     []string{"public"},
			PageID:     "Login",
			ActionName: "Ping",
			Route:      "/Login/Ping",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Ping",
				Signature:    source.BackendSignatureAction0,
			},
		},
		{
			Guards:      []string{"public"},
			PageID:      "Upload",
			ActionName:  "Upload",
			Route:       "/upload",
			InputFields: []string{"avatar", "photos", "caption"},
			UploadFields: []ActionUploadField{
				{Field: "avatar", MaxFiles: 1, MaxBytes: 2048, AllowedContentTypes: []string{"image/png"}},
				{Field: "photos", MaxFiles: 3, MaxBytes: 4096, AllowedContentTypes: []string{"image/*"}},
			},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/auth",
				PackageName:  "auth",
				FunctionName: "Upload",
				Signature:    source.BackendSignatureActionForm,
				InputType:    "UploadInput",
				InputFields: []source.BackendInputField{
					{FieldName: "Avatar", FormName: "avatar", Type: "form.File"},
					{FieldName: "Photos", FormName: "photos", Type: "[]form.File"},
					{FieldName: "Caption", FormName: "caption", Type: "string"},
				},
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
		`field4, ok, err := gowdkform.Uint(values, "code", 8)`,
		`input.Code = byte(field4)`,
		`field5, ok, err := gowdkform.Int(values, "letter", 32)`,
		`input.Letter = rune(field5)`,
		`input, err := decodeLoginLoginBoundInput(values)`,
		`ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "action", PageID: "Login", Name: "Login", Method: "POST", Path: "/Login"})`,
		`result, err := auth.Login(ctx, input)`,
		`result, err := auth.Save(ctx, &input)`,
		`result, err := auth.Ping(ctx)`,
		`request.ParseMultipartForm(gowdkform.DefaultMultipartMemoryBytes)`,
		`gowdkform.DecodeExpectedData(data, gowdkform.Schema{Fields: []gowdkform.Field{{Name: "avatar", File: &gowdkform.FilePolicy{MaxFiles: 1, MaxBytes: 2048, AllowedContentTypes: []string{"image/png"}}}, {Name: "photos", File: &gowdkform.FilePolicy{MaxFiles: 3, MaxBytes: 4096, AllowedContentTypes: []string{"image/*"}}}, {Name: "caption"}}})`,
		`func decodeUploadUploadBoundInput(data gowdkform.Data) (auth.UploadInput, error)`,
		`field0, ok, err := data.File("avatar")`,
		`input.Avatar = field0`,
		`field1 := data.FileList("photos")`,
		`input.Photos = field1`,
		`field2, ok, err := gowdkform.String(values, "caption")`,
		`input.Caption = field2`,
		`result, err := auth.Upload(ctx, input)`,
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
	if strings.Contains(source, `gowdkpartial "github.com/cssbruno/gowdk/runtime/partial"`) {
		t.Fatalf("bound action fragments must not import partial helpers when generated partial branches are not emitted:\n%s", source)
	}
}

func TestBoundActionDecoderRejectsUnsupportedInputFieldType(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected unsupported backend input field type panic")
		}
		message, ok := recovered.(string)
		if !ok || !strings.Contains(message, `unsupported backend input field type "float64"`) {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	boundActionFieldDecodeStmts(0, source.BackendInputField{
		FieldName: "Score",
		FormName:  "score",
		Type:      "float64",
	})
}

func TestGenerateDoesNotImportMissingOrUnsupportedBackendPackages(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "home",
			ActionName: "Submit",
			Route:      "/",
			Fragments:  []ActionFragment{{Target: "#home", HTML: "<p>ignored</p>"}},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingMissing,
				ImportPath:   "example.com/app/missing",
				PackageName:  "missing",
				FunctionName: "Submit",
				Message:      "GOWDK action handler missing.Submit is not implemented",
			},
		}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "home",
			APIName: "Status",
			Method:  "GET",
			Route:   "/api/status",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingUnsupportedSignature,
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
		`gowdkpartial "github.com/cssbruno/gowdk/runtime/partial"`,
		`missing.Submit(`,
		`status.Status(`,
		`<p>ignored</p>`,
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
				Guards:     []string{"public"},
				PageID:     "z",
				ActionName: "Zed",
				Route:      "/z",
				Binding: source.BackendBinding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/beta",
					PackageName:  "beta",
					FunctionName: "Zed",
					Signature:    source.BackendSignatureAction0,
				},
			},
			{
				Guards:     []string{"public"},
				PageID:     "a",
				ActionName: "Alpha",
				Route:      "/a",
				Binding: source.BackendBinding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/alpha",
					PackageName:  "alpha",
					FunctionName: "Alpha",
					Signature:    source.BackendSignatureAction0,
				},
			},
		},
		APIs: []APIEndpoint{
			{
				Guards:  []string{"public"},
				PageID:  "z",
				APIName: "ZedAPI",
				Method:  http.MethodGet,
				Route:   "/api/z",
				Binding: source.BackendBinding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/beta",
					PackageName:  "beta",
					FunctionName: "ZedAPI",
					Signature:    source.BackendSignatureAPI,
				},
			},
			{
				Guards:  []string{"public"},
				PageID:  "a",
				APIName: "AlphaAPI",
				Method:  http.MethodGet,
				Route:   "/api/a",
				Binding: source.BackendBinding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/alpha",
					PackageName:  "alpha",
					FunctionName: "AlphaAPI",
					Signature:    source.BackendSignatureAPI,
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

func TestActionHandlerSourceReturnsInvalidGeneratedIdentifierError(t *testing.T) {
	_, err := actionHandlerSource([]ActionEndpoint{invalidGeneratedIdentifierActionEndpoint()}, false)
	assertInvalidGeneratedIdentifierError(t, err)
}

func TestGenerateWithOptionsReturnsInvalidGeneratedIdentifierError(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "Login", "index.html"), "<main>Login</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{
		Actions: []ActionEndpoint{invalidGeneratedIdentifierActionEndpoint()},
	})
	assertInvalidGeneratedIdentifierError(t, err)
}

func invalidGeneratedIdentifierActionEndpoint() ActionEndpoint {
	return ActionEndpoint{
		PageID:       "Login",
		ActionName:   "Login",
		Method:       http.MethodPost,
		Route:        "/Login",
		BackendAlias: "auth",
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/auth",
			PackageName:  "auth",
			FunctionName: "Login",
			Signature:    source.BackendSignatureActionForm,
			InputType:    "LoginInput",
			InputFields: []source.BackendInputField{{
				FieldName: "Email-Address",
				FormName:  "email",
				Type:      "string",
			}},
		},
	}
}

func assertInvalidGeneratedIdentifierError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected invalid generated identifier error")
	}
	if !strings.Contains(err.Error(), `invalid generated Go identifier "Email-Address"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeneratedPackageSourceIsGoFormatted(t *testing.T) {
	source, err := appPackageSource(Options{IR: &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "example.com/app/contracts/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		Method:      http.MethodPost,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
	}}}})
	if err != nil {
		t.Fatalf("appPackageSource: %v", err)
	}
	formatted, err := format.Source([]byte(source))
	if err != nil {
		t.Fatalf("generated package source is not valid Go: %v\n%s", err, source)
	}
	if source != string(formatted) {
		t.Fatalf("generated package source must be gofmt-formatted")
	}
}

func TestGeneratedDeclarationSnippetIsGoFormatted(t *testing.T) {
	source, err := actionHandlerSource([]ActionEndpoint{{
		Guards:      []string{"public"},
		PageID:      "newsletter",
		ActionName:  "Subscribe",
		Method:      http.MethodPost,
		Route:       "/newsletter",
		InputFields: []string{"email"},
		Redirect:    "/newsletter?ok=1",
	}}, false)
	if err != nil {
		t.Fatalf("actionHandlerSource: %v", err)
	}
	wrapped := []byte("package gowdkapp\n\n" + source)
	formatted, err := format.Source(wrapped)
	if err != nil {
		t.Fatalf("generated declaration snippet is not valid Go: %v\n%s", err, source)
	}
	formattedSnippet := strings.TrimSuffix(strings.TrimPrefix(string(formatted), "package gowdkapp\n\n"), "\n")
	if source != formattedSnippet {
		t.Fatalf("generated declaration snippet must be gofmt-formatted")
	}
}

func TestGenerateWritesBoundAPIHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "status", "index.html"), "<main>Status</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{APIs: []APIEndpoint{{
		Guards:  []string{"public"},
		PageID:  "status",
		APIName: "Health",
		Method:  http.MethodGet,
		Route:   "/api/health",
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/status",
			PackageName:  "status",
			FunctionName: "Health",
			Signature:    source.BackendSignatureAPI,
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
		`const maxAPIBodyBytes int64 = 1 << 20`,
		`func api(response http.ResponseWriter, request *http.Request) bool`,
		`requestPath := path.Clean("/" + request.URL.Path)`,
		`case request.Method == "GET" && requestPath == "/api/health":`,
		`ctx := gowdkruntime.WithEndpoint(gowdkruntime.WithRequest(request.Context(), request), gowdkruntime.EndpointMetadata{Kind: "api", PageID: "status", Name: "Health", Method: "GET", Path: "/api/health"})`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxAPIBodyBytes)`,
		`result, err := status.Health(ctx, request)`,
		`gowdkresponse.WriteNoStoreHandlerError(response, err, http.StatusInternalServerError)`,
		`gowdkresponse.WriteNoStoreHTTP(response, result)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `github.com/cssbruno/gowdk/addons/ssr`) || strings.Contains(source, `github.com/cssbruno/gowdk/runtime/render`) {
		t.Fatalf("API output should not import SSR or render helpers:\n%s", source)
	}
	for _, unexpected := range []string{
		`func newCSRF() (*gowdkactions.CSRF, error)`,
		`err := csrfValidator.Validate(request)`,
		`gowdkresponse.WriteNoStoreJSONError(response, http.StatusForbidden, "invalid csrf token")`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("safe API output should not emit CSRF validation %q:\n%s", unexpected, source)
		}
	}
}

func TestGenerateUsesConfiguredBodyLimits(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{BodyLimits: gowdk.BodyLimitsConfig{
			ActionBytes: 2048,
			APIBytes:    4096,
		}}},
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Route:      "/newsletter",
		}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "newsletter",
			APIName: "Health",
			Method:  http.MethodPost,
			Route:   "/api/health",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/status",
				PackageName:  "status",
				FunctionName: "Health",
				Signature:    source.BackendSignatureAPI,
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
		`const maxActionBodyBytes int64 = 2048`,
		`const maxAPIBodyBytes int64 = 4096`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)`,
		`request.Body = http.MaxBytesReader(response, request.Body, maxAPIBodyBytes)`,
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
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Route:      "/newsletter",
			ErrorPage:  "/errors/subscribe.html",
		}},
		APIs: []APIEndpoint{{
			Guards:    []string{"public"},
			PageID:    "status",
			APIName:   "Health",
			Method:    http.MethodGet,
			Route:     "/api/health",
			ErrorPage: "/errors/health.html",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/status",
				PackageName:  "status",
				FunctionName: "Health",
				Signature:    source.BackendSignatureAPI,
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
		Guards:      []string{"public"},
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
		`gowdkpartial "github.com/cssbruno/gowdk/runtime/partial"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`partial := strings.TrimSpace(request.Header.Get("X-GOWDK-Partial"))`,
		`fragment := gowdkpartial.Fragment("#patients", "<section><p>Updated patients</p></section>")`,
		`gowdkpartial.Swap(fragment.Target, gowdkpartial.SwapMode(swap), fragment.Body)`,
		`gowdkresponse.WriteNoStoreHTTP(response, fragment)`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusNotFound, "partial fragment not found")`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.RedirectTo("/patients"))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `github.com/cssbruno/gowdk/addons/ssr`) {
		t.Fatalf("action fragment output should not import SSR helpers:\n%s", source)
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
		Guards: []string{"public"},
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
		`SSRDynamic: ssrDynamic,`,
		`RequestTimeout: gowdkruntime.DefaultRequestTimeout}`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`func ssrExact(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`func ssrDynamic(response http.ResponseWriter, request *http.Request) (handled bool)`,
		`case "/dashboard":`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr", Guards: []string{"public"}})`,
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

func TestGenerateInjectsCSRFIntoSSRForms(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Route:       "/newsletter",
			Guards:      []string{"public"},
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
		}},
		SSR: []SSRRoute{{
			PageID: "newsletter",
			Route:  "/newsletter",
			Guards: []string{"public"},
			HTML:   `<main><form method="post" action="/newsletter"><input name="email"></form></main>`,
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
		`var csrfTokenSource *gowdkactions.CSRF`,
		`htmlBytes, csrfOK := gowdkruntime.CSRFInjectHTML(response, request, []byte(html), csrfTokenSource)`,
		`if !csrfOK {`,
		`html = string(htmlBytes)`,
		`gowdkresponse.WriteNoStoreHTML(response, request, html)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected SSR CSRF source to contain %q:\n%s", expected, source)
		}
	}
	assertSourceOrder(t, source,
		`html := "<main><form method=\"post\" action=\"/newsletter\"><input name=\"email\"></form></main>"`,
		`gowdkruntime.CSRFInjectHTML(response, request, []byte(html), csrfTokenSource)`,
		`gowdkresponse.WriteNoStoreHTML(response, request, html)`,
	)
}

func TestGenerateWritesSSRCachePolicy(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "docs",
		Route:  "/docs",
		Guards: []string{"public"},
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
		Guards:    []string{"public"},
		ErrorPage: "/errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadError,
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
		`gowdkssr "github.com/cssbruno/gowdk/runtime/ssr"`,
		`"fmt"`,
		`ErrorPages: gowdkruntime.LoadErrorPagesWith(root, gowdkruntime.ErrorPage{Path: "errors/dashboard.html"})`,
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr", ErrorPage: "errors/dashboard.html", Guards: []string{"public"}, HasLoad: true})`,
		`gowdkruntime.RecoverSSRRoutePanic(response, request, recovered)`,
		`loadContext := gowdkssr.NewLoadContext(request, nil)`,
		`loadData, err := dashboard.LoadDashboard(loadContext)`,
		`redirectURL, redirectStatus, ok := gowdkssr.RedirectTarget(err)`,
		`gowdkresponse.WriteNoStoreHTTP(response, gowdkresponse.Response{Kind: gowdkresponse.Redirect, Status: redirectStatus, URL: redirectURL})`,
		`gowdkruntime.WriteErrorPage(response, request, http.StatusInternalServerError, gowdkresponse.HandlerErrorMessage(err, http.StatusInternalServerError))`,
		`loadValue0, loadOK0 := gowdkssr.LoadPath(loadData, "user.name")`,
		`gowdkruntime.WriteErrorPage(response, request, http.StatusInternalServerError, "missing load field user.name")`,
		`strings.ReplaceAll(html, "__USER__", gowdkhtml.Escape(fmt.Sprint(loadValue0)))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesTypedSSRLoadResultAdapter(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "dashboard",
		Route:   "/dashboard",
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadStructError,
			ResultType:   "DashboardData",
			ResultFields: []source.BackendResultField{
				{Path: "user", Selector: "User"},
				{Path: "User", Selector: "User"},
				{Path: "user.name", Selector: "User.Name"},
				{Path: "count", Selector: "Count"},
				{Path: "Count", Selector: "Count"},
			},
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
		`typedLoadData, err := dashboard.LoadDashboard(loadContext)`,
		`loadData := map[string]any{"Count": typedLoadData.Count, "User": typedLoadData.User, "count": typedLoadData.Count, "user": typedLoadData.User}`,
		`loadValue0, loadOK0 := gowdkssr.LoadPath(loadData, "user.name")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateWritesURLAwareSSRLoadReplacements(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "profile",
		Route:   "/profile",
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/profile",
			PackageName:  "profile",
			FunctionName: "LoadProfile",
			Signature:    source.BackendSignatureLoadError,
		},
		HTML: `<main><a href="/user/__SLUG_URL__">__SLUG__</a></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.slug", Placeholder: "__SLUG__"},
			{Path: "user.slug", Placeholder: "__SLUG_URL__", URL: true},
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
		`strings.ReplaceAll(html, "__SLUG__", gowdkhtml.Escape(fmt.Sprint(loadValue0)))`,
		`strings.ReplaceAll(html, "__SLUG_URL__", gowdkhtml.EscapeURL(fmt.Sprint(loadValue1)))`,
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
			Guards:  []string{"public"},
			HasLoad: true,
			LoadBinding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "LoadDashboard",
				Signature:    source.BackendSignatureLoadError,
			},
			HTML: `<main><h1>__USER__</h1></main>`,
			LoadReplacements: []SSRLoadReplacement{{
				Path:        "user.name",
				Placeholder: "__USER__",
			}},
		}},
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "dashboard",
			ActionName: "Save",
			Method:     http.MethodPost,
			Route:      "/dashboard",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "Save",
				Signature:    source.BackendSignatureAction0,
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
		Guards: []string{"public"},
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
		`ctx := gowdkruntime.WithRoute(request.Context(), gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "blog.post", Method: "GET", Path: "/blog/{slug}", Render: "ssr", DynamicParams: []string{"slug"}, Guards: []string{"public"}})`,
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

func TestGenerateWritesURLAwareSSRReplacements(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "blog.post",
		Route:  "/blog/{slug}",
		Guards: []string{"public"},
		HTML:   `<main><a href="/blog/__SLUG_URL__">__SLUG__</a></main>`,
		Replacements: []SSRReplacement{
			{Param: "slug", Placeholder: "__SLUG__"},
			{Param: "slug", Placeholder: "__SLUG_URL__", URL: true},
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
		`strings.ReplaceAll(html, "__SLUG__", gowdkhtml.Escape(params["slug"]))`,
		`strings.ReplaceAll(html, "__SLUG_URL__", gowdkhtml.EscapeURL(params["slug"]))`,
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
		Guards: []string{"public"},
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

func TestGenerateWritesRestParamSSRHandler(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:        "docs.page",
		Route:         "/docs/{path...}",
		Guards:        []string{"public"},
		DynamicParams: []string{"path"},
		RouteParams:   []source.RouteParam{{Name: "path", Type: "string"}},
		HTML:          `<main>__PATH__</main>`,
		Replacements: []SSRReplacement{{
			Param:       "path",
			Placeholder: "__PATH__",
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
		`gowdkroute.Match("/docs/{path...}", request.URL.Path)`,
		`paramValue0, paramOK0, paramErr0 := gowdkroute.String(params, "path")`,
		`strings.ReplaceAll(html, "__PATH__", gowdkhtml.Escape(params["path"]))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `case "/docs/{path...}":`) {
		t.Fatalf("expected generated main.go not to use exact literal match for rest route:\n%s", source)
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
		Guards:        []string{"public"},
		DynamicParams: []string{"id"},
		RouteParams:   []source.RouteParam{{Name: "id", Type: "int"}},
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

func TestGenerateWritesDynamicFragmentRouteParamBindings(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{Fragments: []FragmentEndpoint{{
		Guards:       []string{"public"},
		PageID:       "patients",
		FragmentName: "Vitals",
		Method:       "GET",
		Route:        "/patients/{id:int}/vitals",
		RouteParams:  []source.RouteParam{{Name: "id", Type: "int"}},
		Target:       "#vitals",
		HTML:         "<section>Vitals</section>",
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
		`gowdkroute "github.com/cssbruno/gowdk/runtime/route"`,
		`if params, ok := gowdkroute.Match("/patients/{id:int}/vitals", request.URL.Path); request.Method == "GET" && ok {`,
		`ctx = gowdkruntime.WithParams(ctx, params)`,
		`paramValue0, paramOK0, paramErr0 := gowdkroute.Int(params, "id")`,
		`gowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, "invalid route parameter id")`,
		`typedParams["id"] = paramValue0`,
		`ctx = gowdkruntime.WithTypedParams(ctx, typedParams)`,
		`gowdkpartial.Fragment("#vitals", "<section>Vitals</section>")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `requestPath == "/patients/{id:int}/vitals"`) {
		t.Fatalf("expected generated main.go not to use exact literal match for dynamic fragment route:\n%s", source)
	}
}

func TestGenerateAutoDetectsActionAndSSRRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{
		{
			ID:     "newsletter",
			Route:  "/newsletter",
			Guards: []string{"public"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<form g:post={Subscribe}><input name="email" required /></form>`,
				Actions: []gwdkir.Action{{
					Name:           "Subscribe",
					InputName:      "input",
					InputType:      "SubscribeInput",
					ValidatesInput: true,
					Redirect:       "/newsletter?ok=1",
				}},
				Fragments: []gwdkir.FragmentEndpoint{{
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
			Guards: []string{"auth.required"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><h1>Dashboard</h1></main>`,
			},
		},
	}}

	config := gowdk.Config{
		Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
	}
	ir := gwdkanalysis.BuildProgram(config, app)
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
		`gowdkpartial.Fragment("#newsletter", "<section>Newsletter list</section>")`,
		`case "/dashboard":`,
		`gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr", Guards: []string{"auth.required"}}`,
		`<main><h1>Dashboard</h1></main>`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected auto-detected generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateAutoRoutesLocalizesSSRRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><h1>Dashboard</h1></main>`,
		},
	}}}

	config := gowdk.Config{
		Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
		I18N: gowdk.I18NConfig{
			Locales: []gowdk.LocaleConfig{{Code: "en"}, {Code: "pt"}},
		},
	}
	ir := gwdkanalysis.BuildProgram(config, app)
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
		`case "/en/dashboard":`,
		`case "/pt/dashboard":`,
		`gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/en/dashboard", Render: "ssr", Locale: "en", Guards: []string{"public"}}`,
		`gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/pt/dashboard", Render: "ssr", Locale: "pt", Guards: []string{"public"}}`,
		`ctx = gowdkruntime.WithLocale(ctx, "en")`,
		`ctx = gowdkruntime.WithLocale(ctx, "pt")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected localized SSR generated app source to contain %q:\n%s", expected, source)
		}
	}
}

func TestGenerateAutoRoutesUseDefaultHybridRenderMetadata(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><h1>Dashboard</h1></main>`,
		},
	}}}

	config := gowdk.Config{
		Render: gowdk.RenderConfig{Default: gowdk.Hybrid},
		Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
	}
	ir := gwdkanalysis.BuildProgram(config, app)
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
	expected := `gowdkruntime.RouteMetadata{Kind: "ssr", PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "hybrid", Guards: []string{"public"}}`
	if !strings.Contains(source, expected) {
		t.Fatalf("expected default-hybrid generated app source to contain %q:\n%s", expected, source)
	}
	if strings.Contains(source, `PageID: "dashboard", Method: "GET", Path: "/dashboard", Render: "ssr"`) {
		t.Fatalf("expected default-hybrid metadata not to report ssr:\n%s", source)
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
		`gowdkauth "github.com/cssbruno/gowdk/runtime/auth"`,
		`gowdkguard "github.com/cssbruno/gowdk/runtime/guard"`,
		`var guardRegistry gowdkguard.Registry`,
		`func RegisterGuards(registry gowdkguard.Registry)`,
		`var authProvider gowdkauth.Provider`,
		`func RegisterAuthProvider(provider gowdkauth.Provider)`,
		`func init()`,
		`RegisterGuards(GOWDKGuardRegistry())`,
		`gowdkguard.RunGuardsWithAuth(guardContext, guards, guardRegistry, authProvider)`,
		`gowdkguard.WriteNoStoreFailure(response, err)`,
		`if !runGuards(response, request, []string{"auth.required"})`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected guard generated source to contain %q:\n%s", expected, source)
		}
	}
	if strings.Contains(source, `github.com/cssbruno/gowdk/addons/ssr`) {
		t.Fatalf("guard-only request-time output should not import SSR helpers:\n%s", source)
	}
}

func TestGenerateWiresAuthAddonSessionProviderAndRequiredGuard(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{authaddon.Addon(authaddon.Options{
			SecretEnv:  "GOWDK_TEST_AUTH_SECRET",
			CookieName: "site_session",
			TTL:        2 * time.Hour,
			Insecure:   true,
		})}},
		SSR: []SSRRoute{{
			PageID: "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required", "role:user"},
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
		`gowdkauthaddon "github.com/cssbruno/gowdk/addons/auth"`,
		`func configureAuth() error`,
		`gowdkauthaddon.Configure(gowdkauthaddon.Options{SecretEnv: "GOWDK_TEST_AUTH_SECRET", CookieName: "site_session", TTL: 7200000000000, Insecure: true})`,
		`RegisterAuthProvider(sessions.Provider())`,
		`guardRegistry["auth.required"] = gowdkauthaddon.RequireAuthenticated(authProvider)`,
		`if err := configureAuth(); err != nil`,
		`gowdkguard.RunGuardsWithAuth(guardContext, guards, guardRegistry, authProvider)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected auth addon generated source to contain %q:\n%s", expected, source)
		}
	}
	for _, unexpected := range []string{
		`RegisterGuards(GOWDKGuardRegistry())`,
		`RegisterAuthProvider(GOWDKAuthProvider())`,
	} {
		if strings.Contains(source, unexpected) {
			t.Fatalf("auth addon should not require app hook %q:\n%s", unexpected, source)
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
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/session",
				PackageName:  "session",
				FunctionName: "Session",
				Signature:    source.BackendSignatureAPI,
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
			LoadBinding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/dashboard",
				PackageName:  "dashboard",
				FunctionName: "LoadDashboard",
				Signature:    source.BackendSignatureLoadError,
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
		`gowdkratelimit "github.com/cssbruno/gowdk/runtime/ratelimit"`,
		`gowdkguard "github.com/cssbruno/gowdk/runtime/guard"`,
		`var rateLimiter *gowdkratelimit.Limiter`,
		`func RegisterRateLimiter(limiter *gowdkratelimit.Limiter)`,
		`result, err := rateLimiter.AllowRequest(request)`,
		`gowdkratelimit.WriteHeaders(response, result)`,
		`rateLimiter.HandleError(response, request, err)`,
		`rateLimiter.HandleLimit(response, request, result)`,
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
		`fragment := gowdkpartial.Fragment("#patients", "<section>Patients</section>")`,
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

func TestGenerateBackendAutoRoutesRejectsInvalidIR(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "generated-backend")
	invalidIR := &gwdkir.Program{Routes: []gwdkir.Route{{
		Kind:   gwdkir.RouteSPA,
		Method: "GET",
		Path:   "/",
		PageID: "missing",
	}}}

	_, err := GenerateBackendWithOptions(appDir, Options{AutoRoutes: true, IR: invalidIR})
	if err == nil || !strings.Contains(err.Error(), "invalid compiler IR: invalid IR") {
		t.Fatalf("expected invalid compiler IR error, got %v", err)
	}
}

func TestActionEndpointsInfersInputFieldsFromGPostForm(t *testing.T) {
	routes, err := actionEndpointsFromManifestFixture(gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: gwdkir.Blocks{
			ViewBody: `<form g:post={Subscribe}><input name="email" required minlength="5" maxlength="80" pattern="[a-z]+@[a-z]+[.][a-z]{2,4}" g:message:required="Email is required" g:message:pattern="Use a real email" /><textarea name="note"></textarea></form>`,
			Actions: []gwdkir.Action{{
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
	routes, err := actionEndpointsFromManifestFixture(gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "newsletter",
		Route: "/newsletter",
		Blocks: gwdkir.Blocks{
			ViewBody: `<form g:post={Subscribe}><input name="email" /><button name="intent" value="save">Save</button><button type="button" name="local">Local</button></form>`,
			Actions: []gwdkir.Action{{
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
	routes, err := actionEndpointsFromManifestFixture(gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: gwdkir.Blocks{
			ViewBody: `<form g:post={Refresh} g:target="#patients"><input name="query" /></form><section id="patients"></section>`,
			Actions: []gwdkir.Action{{
				Name:      "Refresh",
				InputName: "input",
				InputType: "PatientFilter",
				Fragments: []gwdkir.Fragment{{
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
	routes, err := fragmentEndpointsFromManifestFixture(gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "patients",
			Route:   "/patients",
			Package: "pages",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks: gwdkir.Blocks{
				Fragments: []gwdkir.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/patients/list",
					Target: "#patients",
					Body:   `<section><ui.PatientCard name="Updated & safe" /></section>`,
				}},
			},
		}},
		Components: []gwdkir.Component{{
			Name:    "PatientCard",
			Package: "components",
			Props:   []gwdkir.Prop{{Name: "name", Type: "string"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<article>{name}</article>`},
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

func TestActionEndpointsRejectsFileInputsWithoutMultipartWithPageContext(t *testing.T) {
	_, err := actionEndpointsFromManifestFixture(gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "profile",
		Route: "/profile",
		Blocks: gwdkir.Blocks{
			ViewBody: `<form g:post={save}><input name="avatar" type="file" /></form>`,
			Actions: []gwdkir.Action{{
				Name:     "save",
				Redirect: "/profile?ok=1",
			}},
		},
	}}})
	if err == nil {
		t.Fatal("expected file input error")
	}
	if !strings.Contains(err.Error(), `profile: file input "avatar" requires enctype="multipart/form-data"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionEndpointsInfersUploadFieldsFromMultipartForm(t *testing.T) {
	routes, err := actionEndpointsFromManifestFixture(gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "profile",
		Route: "/profile",
		Blocks: gwdkir.Blocks{
			ViewBody: `<form g:post={save} enctype="multipart/form-data"><input name="avatar" type="file" g:max-file-size="2048" g:max-files="1" accept="image/png,image/*" /></form>`,
			Actions: []gwdkir.Action{{
				Name:     "save",
				Redirect: "/profile?ok=1",
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || len(routes[0].UploadFields) != 1 {
		t.Fatalf("expected one upload field, got %#v", routes)
	}
	upload := routes[0].UploadFields[0]
	if upload.Field != "avatar" || upload.MaxFiles != 1 || upload.MaxBytes != 2048 || strings.Join(upload.AllowedContentTypes, ",") != "image/png,image/*" {
		t.Fatalf("unexpected upload field: %#v", upload)
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
	tests := []struct {
		redirect string
		message  string
	}{
		{redirect: "https://example.com", message: "must be a local absolute path"},
		{redirect: "//example.com", message: "must not be protocol-relative"},
		{redirect: "/login\nSet-Cookie: bad=true", message: "must not contain newlines"},
		{redirect: `/\evil.com`, message: "must not be protocol-relative"},
		{redirect: `\\evil.com`, message: "must be a local absolute path"},
		{redirect: `/foo\..\\evil.com`, message: "must not contain backslashes"},
	}
	for _, test := range tests {
		t.Run(test.redirect, func(t *testing.T) {
			root := t.TempDir()
			outputDir := filepath.Join(root, "dist")
			appDir := filepath.Join(root, "generated-app")
			writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

			_, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
				Guards:     []string{"public"},
				PageID:     "newsletter",
				ActionName: "Subscribe",
				Route:      "/newsletter",
				Redirect:   test.redirect,
			}}})
			if err == nil {
				t.Fatal("expected unsafe redirect error")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error to contain %q, got %v", test.message, err)
			}
		})
	}
}

func TestGenerateRejectsEmptyActionValidationRule(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), "<main>Newsletter</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{Actions: []ActionEndpoint{{
		Guards:          []string{"public"},
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
		Guards:     []string{"public"},
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

func TestGenerateRejectsInvalidContractRoutes(t *testing.T) {
	tests := []struct {
		name string
		ref  gwdkir.ContractReference
		want string
	}{
		{
			name: "external command path",
			ref: gwdkir.ContractReference{
				Kind:   gwdkir.ContractCommand,
				Name:   "patients.CreatePatient",
				Method: "POST",
				Path:   "https://example.com/pay",
			},
			want: `endpoint path "https://example.com/pay" must be a local absolute path`,
		},
		{
			name: "unsupported query method",
			ref: gwdkir.ContractReference{
				Kind:   gwdkir.ContractQuery,
				Name:   "patients.GetPatientPage",
				Method: "POST",
				Path:   "/patients",
			},
			want: "query contract routes require GET",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			outputDir := filepath.Join(root, "dist")
			appDir := filepath.Join(root, "generated-app")
			writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

			_, err := GenerateWithOptions(outputDir, appDir, Options{
				IR: &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{test.ref}},
			})
			if err == nil {
				t.Fatal("expected invalid contract route error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error to contain %q, got %v", test.want, err)
			}
		})
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

func TestGenerateRejectsTypedRestSSRRouteParam(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	_, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID: "docs.page",
		Route:  "/docs/{path:int...}",
		HTML:   "<main>Docs</main>",
	}}})
	if err == nil {
		t.Fatal("expected typed rest route parameter error")
	}
	if !strings.Contains(err.Error(), "rest route parameters are always strings") {
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

func TestGeneratedBinaryFailsFastWhenRequiredEnvIsMissing(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Env: gowdk.EnvConfig{
			Vars: []gowdk.EnvVar{
				{Name: "GOWDK_TEST_BACKEND_ORIGIN", Required: true},
			},
			Secrets: []gowdk.SecretEnv{
				{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binaryPath)
	command.Env = append(
		withoutEnv(os.Environ(), "GOWDK_TEST_BACKEND_ORIGIN", "GOWDK_TEST_DATABASE_URL"),
		"GOWDK_ADDR="+freeAddr(t),
		"GOWDK_TEST_BACKEND_ORIGIN=   ",
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("expected generated binary to fail before listening, got timeout with output:\n%s", output)
	}
	if err == nil {
		t.Fatalf("expected generated binary to fail for missing env, got output:\n%s", output)
	}
	for _, expected := range []string{
		"GOWDK_TEST_BACKEND_ORIGIN is required but is not set",
		"GOWDK_TEST_DATABASE_URL is required but is not set",
	} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("expected generated binary output to contain %q:\n%s", expected, output)
		}
	}
}

func TestGeneratedBinaryLoadsExplicitEnvFile(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Env: gowdk.EnvConfig{
			Vars: []gowdk.EnvVar{
				{Name: "GOWDK_TEST_BACKEND_ORIGIN", Required: true},
			},
			Secrets: []gowdk.SecretEnv{
				{Name: "GOWDK_TEST_DATABASE_URL", Required: true, MinBytes: 32},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, ".env.runtime")
	writeTestFile(t, envPath, "GOWDK_TEST_BACKEND_ORIGIN=http://backend.test\nGOWDK_TEST_DATABASE_URL=runtime-secret-value-32-bytes-long\n")
	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(
		withoutEnv(os.Environ(), "GOWDK_TEST_BACKEND_ORIGIN", "GOWDK_TEST_DATABASE_URL"),
		"GOWDK_ADDR="+addr,
		"GOWDK_ENV_FILE="+envPath,
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	}()
	body, err := waitForHTTP("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "<main>Home</main>" {
		t.Fatalf("unexpected response body:\n%s", body)
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

func TestBuildWASMIgnoresLifecycleProviderImports(t *testing.T) {
	root := t.TempDir()
	repoRoot, ok := gowdkRuntimeModuleRoot()
	if !ok {
		t.Fatal("could not locate GOWDK module root")
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+filepath.ToSlash(repoRoot)+`
`)
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	wasmPath := filepath.Join(root, "site.wasm")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")
	writeTestFile(t, filepath.Join(outputDir, "gowdk-assets.json"), `{"version":1,"files":{}}`)
	writeTestFile(t, filepath.Join(root, "services", "services.go"), `//go:build !js

package services

import gowdkapp "github.com/cssbruno/gowdk/runtime/app"

func Services() ([]gowdkapp.Service, error) {
	return []gowdkapp.Service{gowdkapp.ServiceHooks{ServiceName: "noop"}}, nil
}
`)
	t.Chdir(root)

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: gowdk.Config{
		Lifecycle: gowdk.LifecycleConfig{Services: []gowdk.ServiceRef{{
			ImportPath: "example.com/site/services",
			Function:   "Services",
		}}},
	}}); err != nil {
		t.Fatal(err)
	}
	built, err := BuildWASM(appDir, wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if built != wasmPath {
		t.Fatalf("unexpected wasm path: %q", built)
	}
}

func TestGeneratedAppGoEnvDisablesParentWorkspace(t *testing.T) {
	env := generatedAppGoEnv([]string{"PATH=/bin", "GOWORK=/repo/go.work", "GOOS=linux"})
	if !containsString(env, "PATH=/bin") || !containsString(env, "GOOS=linux") {
		t.Fatalf("expected unrelated env vars to be preserved: %#v", env)
	}
	if containsString(env, "GOWORK=/repo/go.work") {
		t.Fatalf("expected parent GOWORK to be removed: %#v", env)
	}
	if !containsString(env, "GOWORK=off") {
		t.Fatalf("expected generated app builds to disable workspace mode: %#v", env)
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
		Guards: []string{"public"},
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
		Guards: []string{"public"},
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
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadError,
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

import "github.com/cssbruno/gowdk/runtime/ssr"

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

func TestGeneratedBinaryExecutesTypedSSRLoadUserLogic(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "dashboard",
		Route:   "/dashboard",
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadStructError,
			ResultType:   "DashboardData",
			ResultFields: []source.BackendResultField{
				{Path: "user", Selector: "User"},
				{Path: "user.name", Selector: "User.Name"},
				{Path: "count", Selector: "Count"},
			},
		},
		HTML: `<main><h1>__USER__</h1><p>__COUNT__</p></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.name", Placeholder: "__USER__"},
			{Path: "count", Placeholder: "__COUNT__"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "dashboard", "dashboard.go"), `package dashboard

import "github.com/cssbruno/gowdk/runtime/ssr"

type DashboardData struct {
	User  UserData `+"`json:\"user\"`"+`
	Count int      `+"`json:\"count\"`"+`
}

type UserData struct {
	Name string `+"`json:\"name\"`"+`
}

func LoadDashboard(ctx ssr.LoadContext) (DashboardData, error) {
	return DashboardData{User: UserData{Name: "Ada <admin>"}, Count: 3}, nil
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

	body, _, err := waitForHTTPResponse("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Ada &lt;admin&gt;", "<p>3</p>"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected typed SSR load response to contain %q, got %s", expected, body)
		}
	}
}

func TestGeneratedBinaryExecutesInlineSSRScriptLoad(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	sourceDir := filepath.Join(root, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:      "dashboard",
		Package: "pages",
		Source:  filepath.Join(sourceDir, "dashboard.page.gwdk"),
		Route:   "/dashboard",
		Render:  gowdk.SSR,
		Guards:  []string{"public"},
		Imports: []gwdkir.Import{{
			Alias: "ssr",
			Path:  "github.com/cssbruno/gowdk/runtime/ssr",
		}},
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { user.name, request.path }`,
			View:       true,
			ViewBody:   `<main><h1>{user.name}</h1><p>{request.path}</p></main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "server",
				Body: `func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return map[string]any{
		"user": map[string]any{"name": "Inline Ada"},
		"request": map[string]any{"path": ctx.Request.URL.Path},
	}, nil
}`,
			}},
		},
	}}}
	ir := gwdkanalysis.BuildProgram(config, app)
	compiler.BindBackendHandlers(&ir)

	if _, err := GenerateWithOptions(outputDir, appDir, Options{AutoRoutes: true, Config: config, IR: &ir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "gowdk_go", "pages", "go.go")); err != nil {
		t.Fatalf("expected generated inline go block package: %v", err)
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

	body, _, err := waitForHTTPResponse("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Inline Ada", "<p>/dashboard</p>"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in SSR response body:\n%s", expected, body)
		}
	}
}

func TestGeneratedBinaryRendersRequestAwareHybridLayouts(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	sourceDir := filepath.Join(root, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "dashboard",
			Package: "pages",
			Source:  filepath.Join(sourceDir, "dashboard.page.gwdk"),
			Route:   "/dashboard",
			Render:  gowdk.Hybrid,
			Layouts: []string{"shell"},
			Guards:  []string{"public"},
			Imports: []gwdkir.Import{{
				Alias: "ssr",
				Path:  "github.com/cssbruno/gowdk/runtime/ssr",
			}},
			Blocks: gwdkir.Blocks{
				Server:     true,
				ServerBody: `=> { request.path }`,
				View:       true,
				ViewBody:   `<main>{request.path}</main>`,
				GoBlocks: []gwdkir.GoBlock{{
					Target: "server",
					Body: `func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return map[string]any{
		"request": map[string]any{"path": ctx.Request.URL.Path},
	}, nil
}`,
				}},
			},
		}},
		Layouts: []gwdkir.Layout{{
			ID:      "shell",
			Package: "pages",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<section><header>{request.path}</header><slot /></section>`,
			},
		}},
	}
	ir := gwdkanalysis.BuildProgram(config, app)
	compiler.BindBackendHandlers(&ir)

	result, err := GenerateWithOptions(outputDir, appDir, Options{AutoRoutes: true, Config: config, IR: &ir})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`Render: "hybrid"`,
		`Layouts: []string{"shell"}`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated app source to contain %q:\n%s", expected, source)
		}
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

	body, _, err := waitForHTTPResponse("http://" + addr + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<section><header>/dashboard</header><main>/dashboard</main></section>") {
		t.Fatalf("expected request-aware hybrid layout response, got:\n%s", body)
	}
}

func TestGenerateWritesStaticInlineGoBlockPackages(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Imports: []gwdkir.Import{{Alias: "strings", Path: "strings"}},
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{
				{
					Body: `func HomeTitle() string {
	return strings.ToUpper("gowdk")
}`,
				},
				{
					Body: `func HomeSeed() string {
	return "static"
}`,
				},
			}},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "Badge",
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
				Body: `func BadgeSeed() string {
	return "badge"
}`,
			}}},
		}},
		Layouts: []gwdkir.Layout{{
			Package: "layouts",
			ID:      "root",
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
				Body: `func LayoutName() string {
	return "root"
}`,
			}}},
		}},
	}

	result, err := GenerateWithOptions(outputDir, appDir, Options{IR: &ir})
	if err != nil {
		t.Fatal(err)
	}
	expectedFiles := []string{
		filepath.Join("gowdk_go", "pages", "go.go"),
		filepath.Join("gowdk_go", "components", "go.go"),
		filepath.Join("gowdk_go", "layouts", "go.go"),
	}
	for _, relPath := range expectedFiles {
		if !containsString(result.Files, relPath) {
			t.Fatalf("expected result files to include %s, got %#v", relPath, result.Files)
		}
		payload, err := os.ReadFile(filepath.Join(appDir, relPath))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(payload), "func ") {
			t.Fatalf("expected generated go block file to contain functions:\n%s", payload)
		}
	}
	pagePayload, err := os.ReadFile(filepath.Join(appDir, "gowdk_go", "pages", "go.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`package pages`, `import strings "strings"`, `func HomeTitle`, `func HomeSeed`} {
		if !strings.Contains(string(pagePayload), expected) {
			t.Fatalf("expected page go block file to contain %q:\n%s", expected, pagePayload)
		}
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = appDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("expected generated app go block packages to pass go test ./...: %v\n%s", err, output)
	}
}

func TestInlineGoBlockImportsParticipateInModuleDetection(t *testing.T) {
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Imports: []gwdkir.Import{{Alias: "copy", Path: "example.com/site/content"}},
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
				Body: `import local "example.com/site/local"

func HomeTitle() string {
	return copy.Title() + local.Suffix()
}`,
			}}},
		}},
	}
	imports := inlineGoBlockImports(&ir)
	for _, expected := range []string{"example.com/site/content", "example.com/site/local"} {
		if !imports[expected] {
			t.Fatalf("expected inline go block imports to include %s, got %#v", expected, imports)
		}
	}
	if !optionsUsesModuleImports(Options{IR: &ir}, "example.com/site") {
		t.Fatalf("expected inline go blocks to mark example.com/site as used")
	}
}

func TestLifecycleServiceImportsParticipateInModuleDetection(t *testing.T) {
	options := Options{Config: gowdk.Config{
		Lifecycle: gowdk.LifecycleConfig{Services: []gowdk.ServiceRef{{
			ImportPath: "example.com/site/services",
			Function:   "Services",
		}}},
	}}
	if !appHasLocalModuleImports(options) {
		t.Fatal("expected lifecycle service imports to require app module wiring")
	}
	if !optionsUsesModuleImports(options, "example.com/site") {
		t.Fatal("expected lifecycle service imports to mark example.com/site as used")
	}
}

func TestIsLocalModuleImportPath(t *testing.T) {
	cases := map[string]bool{
		"context":                   false,
		"net/http":                  false,
		"github.com/cssbruno/gowdk": false,
		"github.com/cssbruno/gowdk/runtime/response": false,
		"example.com/site/content":                   true,
		"github.com/acme/app/pages":                  true,
		"":                                           false,
	}
	for path, want := range cases {
		if got := isLocalModuleImportPath(path); got != want {
			t.Fatalf("isLocalModuleImportPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestAppHasLocalModuleImportsFromInlineGoBlock(t *testing.T) {
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
				Body: `import local "example.com/site/local"

func HomeTitle() string { return local.Suffix() }`,
			}}},
		}},
	}
	if !appHasLocalModuleImports(Options{IR: &ir}) {
		t.Fatal("expected an app importing example.com/site/local to need the local module")
	}

	empty := gwdkir.Program{Pages: []gwdkir.Page{{Package: "pages", ID: "home", Route: "/"}}}
	if appHasLocalModuleImports(Options{IR: &empty}) {
		t.Fatal("expected an app with no app-owned imports not to need the local module")
	}
}

func TestModuleSourceFailsLoudlyWhenAppModuleUndeterminedWithLocalImports(t *testing.T) {
	// Run outside any Go module so `go list -m -json` fails with a clear reason
	// (no go.mod). When the generated app imports an app-owned package we cannot
	// emit its require/replace without the module path, so that failure must
	// surface here instead of producing an app that later fails to build with an
	// opaque "cannot find package" error.
	t.Chdir(t.TempDir())

	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
				Body: `import local "example.com/site/local"

func HomeTitle() string { return local.Suffix() }`,
			}}},
		}},
	}

	if _, err := moduleSource(Options{IR: &ir}); err == nil {
		t.Fatal("expected moduleSource to fail when the app module is undetermined and the app imports app-owned packages")
	} else if !strings.Contains(err.Error(), "cannot determine the app Go module") {
		t.Fatalf("expected the app-module determination failure to be surfaced, got: %v", err)
	}
}

func TestModuleSourceToleratesUndeterminedAppModuleWithoutLocalImports(t *testing.T) {
	// The same `go list -m` failure is non-fatal when the generated app imports
	// nothing app-owned: there is no require/replace to add, so a missing main
	// module is irrelevant and module generation must still succeed.
	t.Chdir(t.TempDir())

	empty := gwdkir.Program{Pages: []gwdkir.Page{{Package: "pages", ID: "home", Route: "/"}}}
	source, err := moduleSource(Options{IR: &empty})
	if err != nil {
		t.Fatalf("expected moduleSource to tolerate an undetermined app module without app-owned imports, got: %v", err)
	}
	if !strings.Contains(source, "module gowdk-generated-app") {
		t.Fatalf("expected a valid generated go.mod, got:\n%s", source)
	}
}

func TestParseCurrentAppModuleSelectsWorkspaceModule(t *testing.T) {
	output := []byte(`{
	"Path": "example.com/root",
	"Main": true,
	"Dir": "/repo",
	"GoMod": "/repo/go.mod"
}
{
	"Path": "example.com/root/adapter",
	"Main": true,
	"Dir": "/repo/runtime/adapter",
	"GoMod": "/repo/runtime/adapter/go.mod"
}
`)

	info, err := parseCurrentAppModule(output, "/repo/runtime/adapter/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != "example.com/root/adapter" || info.Dir != "/repo/runtime/adapter" {
		t.Fatalf("expected adapter module, got %#v", info)
	}
}

func TestParseCurrentAppModuleRejectsAmbiguousWorkspaceOutput(t *testing.T) {
	output := []byte(`{
	"Path": "example.com/root",
	"Main": true,
	"Dir": "/repo",
	"GoMod": "/repo/go.mod"
}
{
	"Path": "example.com/root/adapter",
	"Main": true,
	"Dir": "/repo/runtime/adapter",
	"GoMod": "/repo/runtime/adapter/go.mod"
}
`)

	_, err := parseCurrentAppModule(output, "/other/go.mod")
	if err == nil || !strings.Contains(err.Error(), "workspace modules") {
		t.Fatalf("expected ambiguous workspace output error, got %v", err)
	}
}

func TestGenerateWritesAddonGoBlockConsumerFiles(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	program := gwdkir.Program{Pages: []gwdkir.Page{{
		ID:      "patients",
		Package: "pages",
		Source:  "patients.page.gwdk",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
			Target: "addon.contracts",
			Body:   `func RegisterContracts() {}`,
		}}},
	}}}

	result, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{appgenGoBlockAddon{}}},
		IR:     &program,
	})
	if err != nil {
		t.Fatal(err)
	}
	generatedPath := filepath.Join(appDir, "contracts", "generated.go")
	payload, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `func RegisteredContract() string`) {
		t.Fatalf("expected addon-generated go block file, got:\n%s", payload)
	}
	if !containsString(result.Files, filepath.Join("contracts", "generated.go")) {
		t.Fatalf("expected result files to include addon go block file, got %#v", result.Files)
	}
}

func TestGenerateRejectsUnsupportedAddonGoBlockTarget(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	program := gwdkir.Program{Pages: []gwdkir.Page{{
		ID:      "patients",
		Package: "pages",
		Source:  "patients.page.gwdk",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{GoBlocks: []gwdkir.GoBlock{{
			Target: "addon.contracts",
			Body:   `func RegisterContracts() {}`,
		}}},
	}}}

	_, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("contracts", gowdk.FeatureContracts)}},
		IR:     &program,
	})
	if err == nil {
		t.Fatal("expected unsupported addon go block target error")
	}
	if !strings.Contains(err.Error(), "requires an enabled addon implementing gowdk.GoBlockConsumer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type appgenGoBlockAddon struct{}

func (addon appgenGoBlockAddon) Name() string {
	return "contracts"
}

func (addon appgenGoBlockAddon) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureContracts}
}

func (addon appgenGoBlockAddon) GoBlockTargets() []string {
	return []string{"addon.contracts"}
}

func (addon appgenGoBlockAddon) ValidateGoBlock(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	return nil
}

func (addon appgenGoBlockAddon) GeneratedGo(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return []gowdk.GoBlockFile{{
		Path:   filepath.Join("contracts", "generated.go"),
		Source: "package contracts\n\nfunc RegisteredContract() string { return " + strconv.Quote(target.OwnerID) + " }\n",
	}}, nil
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
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
		Guards:    []string{"public"},
		ErrorPage: "errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoadError,
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

	"github.com/cssbruno/gowdk/runtime/ssr"
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
		Guards:    []string{"public"},
		ErrorPage: "errors/dashboard.html",
		HasLoad:   true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/dashboard",
			PackageName:  "dashboard",
			FunctionName: "LoadDashboard",
			Signature:    source.BackendSignatureLoad,
		},
		HTML: `<main><h1>__USER__</h1></main>`,
		LoadReplacements: []SSRLoadReplacement{
			{Path: "user.name", Placeholder: "__USER__"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "dashboard", "dashboard.go"), `package dashboard

import "github.com/cssbruno/gowdk/runtime/ssr"

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
		Guards:    []string{"public"},
		PageID:    "status",
		APIName:   "Health",
		Method:    http.MethodGet,
		Route:     "/api/health",
		ErrorPage: "errors/health.html",
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/status",
			PackageName:  "status",
			FunctionName: "Health",
			Signature:    source.BackendSignatureAPI,
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
		Config: csrfDisabledConfig(),
		Actions: []ActionEndpoint{{
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     http.MethodPost,
			Route:      "/newsletter",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Subscribe",
				Signature:    source.BackendSignatureAction0,
			},
		}, {
			Guards:     []string{"public"},
			PageID:     "newsletter",
			ActionName: "Explode",
			Method:     http.MethodPost,
			Route:      "/explode",
			ErrorPage:  "errors/missing.html",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Explode",
				Signature:    source.BackendSignatureAction0,
			},
		}},
		APIs: []APIEndpoint{{
			Guards:  []string{"public"},
			PageID:  "session",
			APIName: "Session",
			Method:  http.MethodGet,
			Route:   "/api/session",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Session",
				Signature:    source.BackendSignatureAPI,
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

func TestGeneratedBinarySSRGuardRequiresBackingCode(t *testing.T) {
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
	if _, err := BuildBinary(appDir, binaryPath); err == nil || !strings.Contains(err.Error(), "GOWDKGuardRegistry") {
		t.Fatalf("expected missing GOWDKGuardRegistry compile error, got %v", err)
	}
}

func TestGeneratedBinaryAuthAddonSuppliesGuardBackingCode(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Addons: []gowdk.Addon{authaddon.Addon(authaddon.Options{
			SecretEnv: "GOWDK_TEST_AUTH_SECRET",
		})}},
		SSR: []SSRRoute{{
			PageID: "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required", "role:user"},
			HTML:   "<main><h1>Request Dashboard</h1></main>",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatalf("expected auth addon to satisfy generated guard hooks, got %v", err)
	}
}

func TestGeneratedBinaryBackendGuardsRequireBackingCode(t *testing.T) {
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
	if _, err := BuildBinary(appDir, binaryPath); err == nil || !strings.Contains(err.Error(), "GOWDKGuardRegistry") {
		t.Fatalf("expected missing GOWDKGuardRegistry compile error, got %v", err)
	}
}

func TestGeneratedBinaryContractFallbacksAreExplicitNoStore(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:      gwdkir.ContractCommand,
		Name:      "patients.CreatePatient",
		Type:      "CreatePatient",
		Method:    "POST",
		Path:      "/patients",
		Status:    gwdkir.ContractBindingMissing,
		Message:   "command patients.CreatePatient is not registered",
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Guards:    []string{"public"},
	}}}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ana")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected missing contract response status 501, got %d: %s", response.StatusCode, payload)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON missing contract response, got content type %q: %s", contentType, payload)
	}
	if strings.TrimSpace(string(payload)) != `{"error":"command patients.CreatePatient is not registered"}` {
		t.Fatalf("expected explicit JSON missing contract response, got %s", payload)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on missing contract response, got %q", cacheControl)
	}
}

func TestGeneratedBinaryServesPageAndExecutesContractQuery(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractQuery,
		Name:        "patients.GetPatientPage",
		ImportAlias: "patients",
		ImportPath:  "gowdk-generated-app/patients",
		Type:        "GetPatientPage",
		Result:      "PatientPageData",
		InputFields: []source.BackendInputField{{FieldName: "Filter", FormName: "filter", Type: "string"}},
		Method:      http.MethodGet,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "LoadPatientPage",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
		Guards:      []string{"public"},
	}}}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatientPage struct {
	Filter string
}

type PatientPageData struct {
	Filter string `+"`json:\"filter\"`"+`
	Source string `+"`json:\"source\"`"+`
}

func Register(registry *contracts.Registry) {
	contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{Filter: query.Filter, Source: "contract-query"}, nil
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

	body, headers, err := waitForHTTPResponse("http://" + addr + "/patients")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "<main>Patients page</main>") {
		t.Fatalf("expected normal page response, got %s", body)
	}
	if headers.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("expected HTML page content type, got %q", headers.Get("Content-Type"))
	}

	response, err := waitForHTTPStatusWithHeaders("http://"+addr+"/patients?filter=active", http.MethodGet, "", map[string]string{
		"Accept": "application/json",
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
		t.Fatalf("expected query response status 200, got %d: %s", response.StatusCode, payload)
	}
	for _, expected := range []string{`"filter":"active"`, `"source":"contract-query"`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected query response to contain %q, got %s", expected, payload)
		}
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on query response, got %q", cacheControl)
	}
}

func TestGeneratedBinaryCommandContractUsesRegisteredEventSink(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	eventPath := filepath.Join(root, "events.log")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "gowdk-generated-app/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
		Method:      http.MethodPost,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
		Guards:      []string{"public"},
	}}}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

type PatientCreated struct {
	ID string
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{ID: "patient-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{ID: "patient-1"}, nil
}
`)
	writeTestFile(t, filepath.Join(appDir, appPackageDirName, "contract_sink_register.go"), `package gowdkapp

import (
	"context"
	"os"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type testContractEventSink struct{}

func (testContractEventSink) HandleCommandEvents(ctx context.Context, registry *contracts.Registry, role contracts.Role, events []contracts.EventEnvelope) error {
	path := os.Getenv("GOWDK_TEST_EVENT_SINK")
	payload := ""
	for _, event := range events {
		payload += string(role) + "|" + string(event.Category) + "|" + event.Type + "\n"
	}
	return os.WriteFile(path, []byte(payload), 0600)
}

func init() {
	RegisterContractEventSink(testContractEventSink{})
}
`)
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr, "GOWDK_TEST_EVENT_SINK="+eventPath)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d: %s", response.StatusCode, payload)
	}
	if !strings.Contains(string(payload), `"id":"patient-1"`) {
		t.Fatalf("expected command response result, got %s", payload)
	}
	eventPayload, err := os.ReadFile(eventPath)
	if err != nil {
		t.Fatalf("read event sink output: %v", err)
	}
	for _, expected := range []string{"web|domain|", "patients.PatientCreated"} {
		if !strings.Contains(string(eventPayload), expected) {
			t.Fatalf("expected event sink output to contain %q, got %s", expected, eventPayload)
		}
	}
	// Without query invalidations the single-flight refresh header is absent.
	if got := response.Header.Get("X-GOWDK-Queries"); got != "" {
		t.Fatalf("expected no X-GOWDK-Queries header without invalidations, got %q", got)
	}
}

func TestGeneratedBinaryCommandSetsInvalidatedQueriesHeader(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "gowdk-generated-app/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		QueryInvalidations: []gwdkir.QueryInvalidation{{
			Query:         "patients.GetPatientPage",
			QueryType:     "gowdk-generated-app/patients.GetPatientPage",
			Event:         "gowdk-generated-app/patients.PatientCreated",
			EventType:     "gowdk-generated-app/patients.PatientCreated",
			EventCategory: "domain",
			Status:        gwdkir.ContractBindingBound,
			OwnerKind:     gwdkir.SourcePage,
			OwnerID:       "patients",
		}},
	}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

type PatientCreated struct {
	ID string
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{ID: "patient-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{ID: "patient-1"}, nil
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d", response.StatusCode)
	}
	// The command emits PatientCreated, which the invalidation edge maps to
	// GetPatientPage. The single-flight write path names that region and the
	// matching event ID in response headers so the browser can coordinate the
	// command fallback refresh with realtime invalidation fanout.
	if got := response.Header.Get("X-GOWDK-Queries"); got != "gowdk-generated-app/patients.GetPatientPage" {
		t.Fatalf("expected X-GOWDK-Queries to name the invalidated query, got %q", got)
	}
	if got := response.Header.Get("X-GOWDK-Events"); got == "" {
		t.Fatal("expected X-GOWDK-Events to name the invalidating event ID")
	} else if parts := strings.Split(got, ","); len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		t.Fatalf("expected one X-GOWDK-Events event ID, got %q", got)
	}
}

func TestGeneratedBinaryCommandEmbedsSingleFlightRegionHTML(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "board", "index.html"), "<main>Board page</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "gowdk-generated-app/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		QueryInvalidations: []gwdkir.QueryInvalidation{{
			Query:         "patients.GetPatientPage",
			QueryType:     "gowdk-generated-app/patients.GetPatientPage",
			Event:         "gowdk-generated-app/patients.PatientCreated",
			EventType:     "gowdk-generated-app/patients.PatientCreated",
			EventCategory: "domain",
			Status:        gwdkir.ContractBindingBound,
			OwnerKind:     gwdkir.SourcePage,
			OwnerID:       "patients",
		}},
	}
	// The board page renders a parameterless g:query region bound to
	// GetPatientPage. Its standalone render recipe lets the command adapter embed
	// the refreshed region HTML in its response (true single-flight).
	ssrRoute := SSRRoute{
		PageID:  "board",
		Route:   "/board",
		Render:  gowdk.SSR,
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/board",
			PackageName:  "board",
			FunctionName: "LoadBoard",
			Signature:    source.BackendSignatureLoad,
		},
		HTML: `<main><section data-gowdk-query-type="gowdk-generated-app/patients.GetPatientPage"><ul>__GOWDK_SSR_LIST_board__</ul></section></main>`,
		ListSpecs: []SSRListSpec{{
			Placeholder: "__GOWDK_SSR_LIST_board__",
			SourcePath:  "patients",
			RowTemplate: "<li>__GOWDK_SSR_FIELD_name__</li>",
			Fields:      []SSRListField{{Placeholder: "__GOWDK_SSR_FIELD_name__", Path: "Name"}},
		}},
		QueryRegions: []SSRQueryRegion{{
			QueryType: "gowdk-generated-app/patients.GetPatientPage",
			Template:  `<section data-gowdk-query-type="gowdk-generated-app/patients.GetPatientPage"><ul>__GOWDK_SSR_LIST_board__</ul></section>`,
			ListSpecs: []SSRListSpec{{
				Placeholder: "__GOWDK_SSR_LIST_board__",
				SourcePath:  "patients",
				RowTemplate: "<li>__GOWDK_SSR_FIELD_name__</li>",
				Fields:      []SSRListField{{Placeholder: "__GOWDK_SSR_FIELD_name__", Path: "Name"}},
			}},
		}},
	}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program, SSR: []SSRRoute{ssrRoute}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

type PatientCreated struct {
	ID string
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{ID: "patient-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{ID: "patient-1"}, nil
}
`)
	writeTestFile(t, filepath.Join(appDir, "board", "board.go"), `package board

import "github.com/cssbruno/gowdk/runtime/ssr"

func LoadBoard(ctx ssr.LoadContext) map[string]any {
	return map[string]any{"patients": []map[string]any{{"Name": "Ada"}, {"Name": "Linus"}}}
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d", response.StatusCode)
	}
	if got := response.Header.Get("X-GOWDK-Queries"); got != "gowdk-generated-app/patients.GetPatientPage" {
		t.Fatalf("expected X-GOWDK-Queries to name the invalidated query, got %q", got)
	}
	if got := response.Header.Get("X-GOWDK-Patches"); got != "" {
		t.Fatalf("non-browser command callers must keep raw JSON, got X-GOWDK-Patches=%q", got)
	}
	if body := strings.TrimSpace(string(bodyBytes)); body != `{"id":"patient-1"}` {
		t.Fatalf("non-browser command callers must keep raw result JSON, got %s", body)
	}

	browserResponse, err := waitForHTTPStatusWithHeaders("http://"+addr+"/patients", http.MethodPost, "name=Ada", map[string]string{
		"X-GOWDK-Command": "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	browserBodyBytes, err := io.ReadAll(browserResponse.Body)
	_ = browserResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if browserResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected browser command response status 200, got %d", browserResponse.StatusCode)
	}
	if got := browserResponse.Header.Get("X-GOWDK-Queries"); got != "gowdk-generated-app/patients.GetPatientPage" {
		t.Fatalf("expected browser X-GOWDK-Queries to name the invalidated query, got %q", got)
	}
	// The adapter rendered the invalidated region standalone and returned it
	// inline, signalled by X-GOWDK-Patches. The body is a {result, patches}
	// envelope carrying the refreshed rows, so the client applies them with no
	// second page fetch.
	if got := browserResponse.Header.Get("X-GOWDK-Patches"); got != "1" {
		t.Fatalf("expected X-GOWDK-Patches header, got %q", got)
	}
	body := string(browserBodyBytes)
	if !strings.Contains(body, `"patches"`) || !strings.Contains(body, `"result"`) {
		t.Fatalf("expected single-flight envelope body, got %s", body)
	}
	if !strings.Contains(body, "gowdk-generated-app/patients.GetPatientPage") {
		t.Fatalf("expected patch to name the query, got %s", body)
	}
	if !strings.Contains(body, "Ada") || !strings.Contains(body, "Linus") {
		t.Fatalf("expected rendered region rows in patch HTML, got %s", body)
	}
	if !strings.Contains(body, `"id":"patient-1"`) {
		t.Fatalf("expected command result in envelope, got %s", body)
	}
}

func TestGeneratedBinaryRealtimeFanoutStreamsSubscribedPresentationEvents(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "gowdk-generated-app/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
			Method:      http.MethodPost,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		}},
		RealtimeSubscriptions: []gwdkir.RealtimeSubscription{{
			Query:           "patients.GetPatientPage",
			Event:           "patients.PatientNotice",
			EventImportPath: "gowdk-generated-app/patients",
			EventType:       "PatientNotice",
			Status:          gwdkir.ContractBindingBound,
			OwnerKind:       gwdkir.SourcePage,
			OwnerID:         "patients",
		}},
	}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

type PatientNotice struct {
	ID string `+"`json:\"id\"`"+`
}

type OtherNotice struct {
	ID string `+"`json:\"id\"`"+`
}

type PatientCreated struct {
	ID string
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitPresentation(ctx, PatientNotice{ID: "patient-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	if err := contracts.EmitPresentation(ctx, OtherNotice{ID: "other-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	if err := contracts.EmitDomain(ctx, PatientCreated{ID: "domain-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{ID: "patient-1"}, nil
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
	if _, err := waitForHTTPStatus("http://"+addr+"/_gowdk/health", http.MethodGet, ""); err != nil {
		t.Fatal(err)
	}

	streamCtx, cancelStream := context.WithCancel(context.Background())
	defer cancelStream()
	streamRequest, err := http.NewRequestWithContext(streamCtx, http.MethodGet, "http://"+addr+"/_gowdk/realtime/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamResponse, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer streamResponse.Body.Close()
	if streamResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected realtime stream status 200, got %d", streamResponse.StatusCode)
	}
	lines := make(chan string, 8)
	readErrs := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(streamResponse.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				readErrs <- err
				return
			}
			lines <- line
		}
	}()

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d: %s", response.StatusCode, payload)
	}

	deadline := time.After(5 * time.Second)
	var dataLine string
	for dataLine == "" {
		select {
		case line := <-lines:
			if strings.HasPrefix(line, "data: ") {
				dataLine = line
			}
		case err := <-readErrs:
			t.Fatalf("read realtime stream: %v", err)
		case <-deadline:
			t.Fatal("timed out waiting for realtime event")
		}
	}
	if !strings.Contains(dataLine, `"Category":"presentation"`) ||
		!strings.Contains(dataLine, `"Type":"gowdk-generated-app/patients.PatientNotice"`) ||
		!strings.Contains(dataLine, `"id":"patient-1"`) {
		t.Fatalf("expected subscribed presentation event, got %s", dataLine)
	}
	for _, unexpected := range []string{"OtherNotice", "PatientCreated", "domain-1"} {
		if strings.Contains(dataLine, unexpected) {
			t.Fatalf("realtime stream included unsubscribed event %q in %s", unexpected, dataLine)
		}
	}
}

func TestGeneratedBinaryRealtimeStreamGuardDenialClosesStream(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "dashboard", "index.html"), "<main>Dashboard</main>")

	program := &gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Guards: []string{"auth.required"},
		}},
		RealtimeSubscriptions: []gwdkir.RealtimeSubscription{{
			Query:           "patients.GetPatientPage",
			Event:           "patients.PatientNotice",
			EventImportPath: "gowdk-generated-app/patients",
			EventType:       "PatientNotice",
			Guards:          []string{"auth.required"},
			Status:          gwdkir.ContractBindingBound,
			OwnerKind:       gwdkir.SourcePage,
			OwnerID:         "dashboard",
		}},
	}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, appPackageDirName, "guards_register.go"), `package gowdkapp

import (
	"errors"

	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
)

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return errors.New("denied")
		},
	}
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
	if _, err := waitForHTTPStatus("http://"+addr+"/_gowdk/health", http.MethodGet, ""); err != nil {
		t.Fatal(err)
	}

	response, err := waitForHTTPStatus("http://"+addr+"/_gowdk/realtime/events?path=/dashboard", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected realtime stream guard denial to return 403, got %d: %s", response.StatusCode, payload)
	}
	if response.Header.Get("Content-Type") == "text/event-stream" {
		t.Fatalf("guard-denied realtime stream must not open SSE response: headers=%v body=%s", response.Header, payload)
	}
	if cache := response.Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected guard-denied realtime stream to be no-store, got %q", cache)
	}
	if !strings.Contains(string(payload), "403 forbidden") || strings.Contains(string(payload), "auth.required") {
		t.Fatalf("expected guard denial response, got %s", payload)
	}
}

func TestGeneratedBinaryContractAdaptersReturnJSONErrors(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{
		{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "gowdk-generated-app/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			InputFields: []source.BackendInputField{
				{FieldName: "Name", FormName: "name", Type: "string"},
				{FieldName: "Age", FormName: "age", Type: "int"},
			},
			Method:    http.MethodPost,
			Path:      "/patients",
			Status:    gwdkir.ContractBindingBound,
			Handler:   "HandleCreatePatient",
			Register:  "Register",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Guards:    []string{"public"},
		},
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "gowdk-generated-app/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			InputFields: []source.BackendInputField{{FieldName: "Filter", FormName: "filter", Type: "string"}},
			Method:      http.MethodGet,
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "LoadPatientPage",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Guards:      []string{"public"},
		},
	}}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), IR: program}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"
	"errors"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/contracts"
	gowdkresponse "github.com/cssbruno/gowdk/runtime/response"
)

type CreatePatient struct {
	Name string
	Age int
}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

type GetPatientPage struct {
	Filter string
}

type PatientPageData struct {
	Filter string `+"`json:\"filter\"`"+`
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, errors.New("secret command failure")
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{}, gowdkresponse.NewHandlerError(http.StatusBadRequest, "invalid filter", nil)
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

	commandResponse, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
	if err != nil {
		t.Fatal(err)
	}
	commandPayload, err := io.ReadAll(commandResponse.Body)
	_ = commandResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if commandResponse.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected command error status 500, got %d: %s", commandResponse.StatusCode, commandPayload)
	}
	if commandResponse.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected command JSON error content type, got %q", commandResponse.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(commandPayload)) != `{"error":"Internal Server Error"}` {
		t.Fatalf("unexpected command JSON error payload: %s", commandPayload)
	}
	if strings.Contains(string(commandPayload), "secret") {
		t.Fatalf("command JSON error leaked handler detail: %s", commandPayload)
	}

	commandParseResponse, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "%zz")
	if err != nil {
		t.Fatal(err)
	}
	commandParsePayload, err := io.ReadAll(commandParseResponse.Body)
	_ = commandParseResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if commandParseResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected command parse error status 400, got %d: %s", commandParseResponse.StatusCode, commandParsePayload)
	}
	if commandParseResponse.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected command parse JSON error content type, got %q", commandParseResponse.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(commandParsePayload)) != `{"error":"invalid form"}` {
		t.Fatalf("unexpected command parse JSON error payload: %s", commandParsePayload)
	}

	commandDecodeResponse, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada&age=not-int")
	if err != nil {
		t.Fatal(err)
	}
	commandDecodePayload, err := io.ReadAll(commandDecodeResponse.Body)
	_ = commandDecodeResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if commandDecodeResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected command decode error status 400, got %d: %s", commandDecodeResponse.StatusCode, commandDecodePayload)
	}
	if commandDecodeResponse.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected command decode JSON error content type, got %q", commandDecodeResponse.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(commandDecodePayload)) != `{"error":"invalid form"}` {
		t.Fatalf("unexpected command decode JSON error payload: %s", commandDecodePayload)
	}

	queryResponse, err := waitForHTTPStatusWithHeaders("http://"+addr+"/patients?filter=bad", http.MethodGet, "", map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	queryPayload, err := io.ReadAll(queryResponse.Body)
	_ = queryResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if queryResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected query error status 400, got %d: %s", queryResponse.StatusCode, queryPayload)
	}
	if queryResponse.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected query JSON error content type, got %q", queryResponse.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(queryPayload)) != `{"error":"invalid filter"}` {
		t.Fatalf("unexpected query JSON error payload: %s", queryPayload)
	}

	queryDecodeResponse, err := waitForHTTPStatusWithHeaders("http://"+addr+"/patients?filter=bad&role=admin", http.MethodGet, "", map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	queryDecodePayload, err := io.ReadAll(queryDecodeResponse.Body)
	_ = queryDecodeResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if queryDecodeResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected query decode error status 400, got %d: %s", queryDecodeResponse.StatusCode, queryDecodePayload)
	}
	if queryDecodeResponse.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected query decode JSON error content type, got %q", queryDecodeResponse.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(queryDecodePayload)) != `{"error":"invalid form"}` {
		t.Fatalf("unexpected query decode JSON error payload: %s", queryDecodePayload)
	}
}

func TestGeneratedBinaryContractCommandCSRFReturnsJSONErrorByDefault(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients page</main>")

	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:        gwdkir.ContractCommand,
		Name:        "patients.CreatePatient",
		ImportAlias: "patients",
		ImportPath:  "gowdk-generated-app/patients",
		Type:        "CreatePatient",
		Result:      "CreatePatientResult",
		Method:      http.MethodPost,
		Path:        "/patients",
		Status:      gwdkir.ContractBindingBound,
		Handler:     "HandleCreatePatient",
		Register:    "Register",
		OwnerKind:   gwdkir.SourcePage,
		OwnerID:     "patients",
		Guards:      []string{"public"},
	}}}
	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		IR: program,
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}

type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

func Register(registry *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{ID: "patient-1"}, nil
}
`)
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients", http.MethodPost, "name=Ada")
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
	if response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected csrf JSON error content type, got %q", response.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(payload)) != `{"error":"invalid csrf token"}` {
		t.Fatalf("unexpected csrf JSON error payload: %s", payload)
	}
	if cache := response.Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store on invalid csrf response, got %q", cache)
	}
}

func TestGeneratedBinaryRegisteredGuardsAllowRequestTimeRoutes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: csrfDisabledConfig(),
		Actions: []ActionEndpoint{{
			PageID:     "newsletter",
			ActionName: "Subscribe",
			Method:     "POST",
			Route:      "/newsletter",
			Guards:     []string{"auth.required"},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Subscribe",
				Signature:    source.BackendSignatureAction0,
			},
		}},
		APIs: []APIEndpoint{{
			PageID:  "session",
			APIName: "Session",
			Method:  "GET",
			Route:   "/api/session",
			Guards:  []string{"auth.required"},
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "gowdk-generated-app/backend",
				PackageName:  "backend",
				FunctionName: "Session",
				Signature:    source.BackendSignatureAPI,
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

import gowdkssr "github.com/cssbruno/gowdk/runtime/ssr"

func GOWDKGuardRegistry() gowdkssr.GuardRegistry {
	return gowdkssr.GuardRegistry{
		"auth.required": func(ctx gowdkssr.LoadContext) error {
			return nil
		},
	}
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

func TestGeneratedBinaryGuardCanRedirectRequestTimeRoute(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
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

import gowdkguard "github.com/cssbruno/gowdk/runtime/guard"

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return gowdkguard.RedirectTo("/login")
		},
	}
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
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login" || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store guard redirect, got status=%d headers=%v", response.StatusCode, response.Header)
	}
}

func TestGeneratedBinaryNativeRBACGuardUsesRegisteredAuthProvider(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		SSR: []SSRRoute{{
			PageID: "admin",
			Route:  "/admin",
			Guards: []string{"role:admin", "permission:admin.read"},
			HTML:   "<main><h1>Admin</h1></main>",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, appPackageDirName, "auth_provider_register.go"), `package gowdkapp

import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func GOWDKAuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			ID: "user-1",
			Roles: []string{"admin"},
			Permissions: []string{"admin.read"},
		}, nil
	})
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

	body, _, err := waitForHTTPResponse("http://" + addr + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Admin") {
		t.Fatalf("expected native RBAC guard to allow SSR route, got %s", body)
	}
}

func TestGeneratedBinaryAppliesRegisteredRateLimiter(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: withCSRFDisabled(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ratelimit", gowdk.FeatureRateLimit)}}),
		Actions: []ActionEndpoint{{
			Guards:      []string{"public"},
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

	gowdkratelimit "github.com/cssbruno/gowdk/runtime/ratelimit"
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
		Guards: []string{"public"},
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

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), Actions: []ActionEndpoint{{
		Guards:           []string{"public"},
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
		Guards:     []string{"public"},
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
	for _, expected := range []string{`<div data-gowdk-validation role="alert" aria-live="polite">`, `data-gowdk-field="email"`, `Email is required`} {
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
	for _, expected := range []string{`<div data-gowdk-validation role="alert" aria-live="polite">`, `data-gowdk-field="email"`, `Use a real email address`} {
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

func TestGeneratedBinaryValidatesCSRFByDefault(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "newsletter", "index.html"), `<main><form method="post" action="/newsletter"><input name="email"></form></main>`)

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		Actions: []ActionEndpoint{{
			Guards:      []string{"public"},
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
	if !strings.HasPrefix(cookie, "gowdk-csrf=") {
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

func TestGeneratedBinaryInjectsCSRFIntoSSRForms(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{
		Config: gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{
			SecretEnv: "GOWDK_TEST_CSRF_SECRET",
			Insecure:  true,
		}}},
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Route:       "/newsletter",
			Guards:      []string{"public"},
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
		}},
		SSR: []SSRRoute{{
			PageID: "newsletter",
			Route:  "/newsletter",
			Guards: []string{"public"},
			HTML:   `<main><form method="post" action="/newsletter"><input name="email"></form></main>`,
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
		t.Fatalf("expected SSR form csrf token, got %s", body)
	}
	cookie := cookieHeader(headers.Get("Set-Cookie"))
	if !strings.HasPrefix(cookie, "gowdk-csrf=") {
		t.Fatalf("expected csrf cookie, got %q", headers.Get("Set-Cookie"))
	}
	if cache := headers.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store on csrf-personalized SSR HTML, got %q", cache)
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
}

func TestGeneratedBinaryServesPartialActionFragment(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), Actions: []ActionEndpoint{{
		Guards:      []string{"public"},
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
		Guards:       []string{"public"},
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
		Guards:       []string{"public"},
		PageID:       "patients",
		FragmentName: "List",
		Method:       "GET",
		Route:        "/patients/list",
		Target:       "#patients",
		HTML:         "<section><p>Static fallback</p></section>",
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/patients",
			PackageName:  "patients",
			FunctionName: "List",
			Signature:    source.BackendSignatureFragment,
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

func TestGeneratedBinaryExecutesDynamicFragmentUserHook(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "patients", "index.html"), "<main>Patients</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Fragments: []FragmentEndpoint{{
		Guards:       []string{"public"},
		PageID:       "patients",
		FragmentName: "Vitals",
		Method:       "GET",
		Route:        "/patients/{id:int}/vitals",
		Target:       "#vitals",
		HTML:         "<section><p>Static fallback</p></section>",
		Binding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/patients",
			PackageName:  "patients",
			FunctionName: "Vitals",
			Signature:    source.BackendSignatureFragment,
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "patients", "patients.go"), `package patients

import (
	"context"
	"fmt"

	gowdkapp "github.com/cssbruno/gowdk/runtime/app"
	"github.com/cssbruno/gowdk/runtime/response"
)

func Vitals(ctx context.Context) (response.Response, error) {
	params := gowdkapp.TypedParams(ctx)
	id, ok := params["id"].(int)
	if !ok {
		return response.HTMLBody(500, "missing typed id"), nil
	}
	return response.FragmentFor("#vitals", fmt.Sprintf("<section><p>Patient %d</p></section>", id)), nil
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

	response, err := waitForHTTPStatus("http://"+addr+"/patients/42/vitals", http.MethodGet, "")
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
	if strings.TrimSpace(string(payload)) != "<section><p>Patient 42</p></section>" {
		t.Fatalf("expected runtime fragment hook response, got %s", payload)
	}
	if response.Header.Get("X-GOWDK-Fragment-Target") != "#vitals" {
		t.Fatalf("unexpected fragment target: %q", response.Header.Get("X-GOWDK-Fragment-Target"))
	}

	invalid, err := waitForHTTPStatus("http://"+addr+"/patients/not-int/vitals", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	invalidPayload, err := io.ReadAll(invalid.Body)
	_ = invalid.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid typed route param status 400, got %d: %s", invalid.StatusCode, invalidPayload)
	}
	if !strings.Contains(string(invalidPayload), "invalid route parameter id") {
		t.Fatalf("expected invalid route parameter body, got %s", invalidPayload)
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

	if _, err := GenerateWithOptions(outputDir, appDir, Options{Config: csrfDisabledConfig(), Actions: []ActionEndpoint{{
		Guards:         []string{"public"},
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

// The endpoint derivation tests keep their manifest fixtures and lower them
// through the production manifest->IR path, asserting against exactly what the
// generated-app pipeline derives.
func actionEndpointsFromManifestFixture(app gwdkanalysis.Sources) ([]ActionEndpoint, error) {
	return actionEndpointsFromIR(gowdk.Config{}, gwdkanalysis.BuildProgram(gowdk.Config{}, app))
}

func apiEndpointsFromManifestFixture(app gwdkanalysis.Sources) ([]APIEndpoint, error) {
	return apiEndpointsFromIR(gwdkanalysis.BuildProgram(gowdk.Config{}, app))
}

func fragmentEndpointsFromManifestFixture(app gwdkanalysis.Sources) ([]FragmentEndpoint, error) {
	return fragmentEndpointsFromIR(gwdkanalysis.BuildProgram(gowdk.Config{}, app))
}

func TestDeniedPageRoutesSelectsGuardlessStaticPages(t *testing.T) {
	options := Options{
		IR: &gwdkir.Program{Pages: []gwdkir.Page{
			{ID: "home", Route: "/"},                                   // guardless static -> denied
			{ID: "about", Route: "/about", Guards: []string{"public"}}, // public -> served
			{ID: "dashboard", Route: "/dashboard"},                     // guardless, but request-time below
		}},
		SSR: []SSRRoute{{PageID: "dashboard", Route: "/dashboard"}}, // SSR handler denies this one
	}

	denied := deniedPageRoutes(options)
	if len(denied) != 1 || denied[0] != "/" {
		t.Fatalf("expected only the guardless static route /, got %#v", denied)
	}
}

func TestDeniedPageRoutesLocalizeGuardlessStaticPages(t *testing.T) {
	options := Options{
		Config: gowdk.Config{
			I18N: gowdk.I18NConfig{
				Locales: []gowdk.LocaleConfig{{Code: "en"}, {Code: "pt"}},
			},
		},
		IR: &gwdkir.Program{Pages: []gwdkir.Page{
			{ID: "home", Route: "/"},
			{ID: "dashboard", Route: "/dashboard"},
		}},
		SSR: []SSRRoute{{PageID: "dashboard", Route: "/en/dashboard"}},
	}

	denied := deniedPageRoutes(options)
	if strings.Join(denied, ",") != "/en,/pt" {
		t.Fatalf("expected localized guardless static routes, got %#v", denied)
	}
}

func TestDeniedPageRoutePatternsSelectsGuardlessDynamicPages(t *testing.T) {
	options := Options{
		IR: &gwdkir.Program{Pages: []gwdkir.Page{
			{ID: "home", Route: "/"},                                        // static -> exact deny
			{ID: "blog", Route: "/blog/{slug}"},                             // guardless dynamic -> pattern deny
			{ID: "docs", Route: "/docs/{slug}", Guards: []string{"public"}}, // public -> served
			{ID: "feed", Route: "/feed/{slug}"},                             // request-time below -> excluded
		}},
		SSR: []SSRRoute{{PageID: "feed", Route: "/feed/{slug}"}},
	}

	patterns := deniedPageRoutePatterns(options)
	if len(patterns) != 1 || patterns[0] != "/blog/{slug}" {
		t.Fatalf("expected only the guardless dynamic pattern /blog/{slug}, got %#v", patterns)
	}
	// Dynamic routes must not leak into the exact deny set.
	if denied := deniedPageRoutes(options); len(denied) != 1 || denied[0] != "/" {
		t.Fatalf("expected only the static route / in exact deny set, got %#v", denied)
	}
}

func TestGuardlessActionAndAPIAreDeniedByOmission(t *testing.T) {
	const deny = `gowdkresponse.WriteNoStoreError(response, http.StatusForbidden, "403 forbidden")`
	const denyJSON = `gowdkresponse.WriteNoStoreJSONError(response, http.StatusForbidden, "403 forbidden")`

	actionSrc, err := actionHandlerSource([]ActionEndpoint{{PageID: "p", ActionName: "Sub", Route: "/sub"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(actionSrc, deny) {
		t.Fatalf("guardless action must be denied by omission:\n%s", actionSrc)
	}

	apiSrc, err := apiHandlerSource([]APIEndpoint{{PageID: "p", APIName: "Show", Method: "GET", Route: "/show"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(apiSrc, deny) {
		t.Fatalf("guardless api must be denied by omission:\n%s", apiSrc)
	}

	fragmentAdapter := backendAdapterIR(Options{Fragments: []FragmentEndpoint{{
		PageID:       "p",
		FragmentName: "Refresh",
		Method:       http.MethodPost,
		Route:        "/fragment",
		Target:       "#target",
		HTML:         "<p>Updated</p>",
	}}})
	fragmentSrc, err := printActionDecls([]ast.Decl{fragmentFuncDecl(fragmentAdapter.Fragments, false)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragmentSrc, deny) {
		t.Fatalf("guardless fragment must be denied by omission:\n%s", fragmentSrc)
	}

	contractsAdapter := backendAdapterIR(Options{IR: &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{
		{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Method:      "POST",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
		},
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			Method:      "GET",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "LoadPatientPage",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
		},
	}}})
	contractSrc, err := printActionDecls(contractHandlerDecls(contractsAdapter.ContractExposures, false, false, false, false))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(contractSrc, denyJSON); got != 2 {
		t.Fatalf("guardless command/query contracts must be denied by omission, got %d JSON denies:\n%s", got, contractSrc)
	}
	if strings.Contains(contractSrc, "contractRegistry") {
		t.Fatalf("guardless denied contract handlers must not require a registry:\n%s", contractSrc)
	}
	if decoders := contractDecoderDecls(contractsAdapter.ContractExposures); len(decoders) != 0 {
		decoderSrc, err := printActionDecls(decoders)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("guardless denied contracts must not emit unused decoders:\n%s", decoderSrc)
	}

	fallbackAdapter := backendAdapterIR(Options{IR: &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{{
		Kind:      gwdkir.ContractCommand,
		Name:      "patients.CreatePatient",
		Method:    http.MethodPost,
		Path:      "/patients",
		Status:    gwdkir.ContractBindingMissing,
		Message:   "command patients.CreatePatient is not registered",
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
	}}}})
	fallbackSrc, err := printActionDecls(contractHandlerDecls(fallbackAdapter.ContractExposures, false, false, false, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallbackSrc, denyJSON) || strings.Contains(fallbackSrc, "StatusNotImplemented") || strings.Contains(fallbackSrc, "not registered") {
		t.Fatalf("guardless fallback contract handler must deny by omission instead of exposing fallback JSON:\n%s", fallbackSrc)
	}

	// An endpoint that declares `guard public` is intentionally reachable and
	// must NOT be denied by omission.
	publicSrc, err := actionHandlerSource([]ActionEndpoint{{PageID: "p", ActionName: "Sub", Route: "/sub", Guards: []string{"public"}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(publicSrc, deny) {
		t.Fatalf("public action must not be denied by omission:\n%s", publicSrc)
	}
}
