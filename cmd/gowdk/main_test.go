package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/internal/addonregistry"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestVersionCommandSupportsJSON(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"version", "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected JSON version output, got %q: %v", stdout, err)
	}
	if decoded.Version != version {
		t.Fatalf("expected version %q, got %q", version, decoded.Version)
	}
}

func TestExplainCommandPrintsDiagnosticExplanation(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"explain", "missing_ssr_addon"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"missing_ssr_addon",
		"Area: rendering",
		"Stability: stable",
		"Next steps:",
		"ssr.Addon()",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected %q in explanation:\n%s", expected, stdout)
		}
	}
}

func TestExplainCommandSupportsJSON(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"explain", "--json", "missing_ssr_addon"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Code      string   `json:"code"`
		Area      string   `json:"area"`
		Stability string   `json:"stability"`
		NextSteps []string `json:"nextSteps"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected JSON explanation, got %q: %v", stdout, err)
	}
	if decoded.Code != "missing_ssr_addon" || decoded.Area != "rendering" || decoded.Stability != "stable" || len(decoded.NextSteps) == 0 {
		t.Fatalf("unexpected JSON explanation: %#v", decoded)
	}
}

func TestExplainCommandSuggestsUnknownCodes(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"explain", "missing_ssr_adon"})
	})
	if err == nil {
		t.Fatal("expected unknown diagnostic code error")
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected direct run to avoid stdout/stderr, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), `unknown diagnostic code "missing_ssr_adon"`) || !strings.Contains(err.Error(), "missing_ssr_addon") {
		t.Fatalf("expected close-code suggestion, got %v", err)
	}
}

func TestDoctorCommandSupportsJSON(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Healthy</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"doctor", "--json", "--config", config, filepath.Join(root, "home.page.gwdk")})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Version int    `json:"version"`
		Status  string `json:"status"`
		Checks  []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected JSON doctor output, got %q: %v", stdout, err)
	}
	if decoded.Version != 1 || decoded.Status != "ok" {
		t.Fatalf("unexpected doctor report: %#v", decoded)
	}
	if !doctorReportHasCheck(decoded.Checks, "language_check", "ok") || !doctorReportHasCheck(decoded.Checks, "routes", "ok") {
		t.Fatalf("expected language and routes checks in report: %#v", decoded.Checks)
	}
}

func TestDoctorCommandReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"doctor", "--json"})
		})
		if err == nil {
			t.Fatal("expected missing config error")
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		var decoded struct {
			Status  string `json:"status"`
			Summary struct {
				Errors int `json:"errors"`
			} `json:"summary"`
			Checks []struct {
				ID      string `json:"id"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"checks"`
		}
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("expected JSON doctor output, got %q: %v", stdout, err)
		}
		if decoded.Status != "error" || decoded.Summary.Errors == 0 || !doctorReportHasCheck(decoded.Checks, "config", "error") {
			t.Fatalf("expected config error report: %#v", decoded)
		}
	})
}

func TestDoctorCommandReportsValidMinimalProject(t *testing.T) {
	root := t.TempDir()
	writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Healthy</main>
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"doctor"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		for _, expected := range []string{
			"GOWDK doctor: OK",
			"Summary:",
			"gowdk_cli",
			"go_toolchain",
			"config",
			"sources",
			"language_check",
			"routes",
		} {
			if !strings.Contains(stdout, expected) {
				t.Fatalf("expected %q in doctor output:\n%s", expected, stdout)
			}
		}
	})
}

func TestDoctorCommandReportsLanguageErrors(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	source := filepath.Join(root, "bad.page.gwdk")
	writeCLIFile(t, source, `package app

page bad
route "/bad"

load {
  => { title: "needs ssr" }
}

view {
  <main>{title}</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"doctor", "--json", "--config", config, source})
	})
	if err == nil {
		t.Fatal("expected language check error")
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Status string `json:"status"`
		Checks []struct {
			ID        string   `json:"id"`
			Status    string   `json:"status"`
			NextSteps []string `json:"nextSteps"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected JSON doctor output, got %q: %v", stdout, err)
	}
	if decoded.Status != "error" || !doctorReportHasCheck(decoded.Checks, "language_check", "error") || !doctorReportHasCheck(decoded.Checks, "routes", "skipped") {
		t.Fatalf("expected language error and skipped routes: %#v", decoded)
	}
}

func TestDoctorCommandWarnsForRelevantMissingOptionalTool(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{Input: "styles/app.css"}),
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "styles", "app.css"), `@import "tailwindcss";
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Styled</main>
}
`)

	withWorkingDir(t, root, func() {
		pathDir := t.TempDir()
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(goPath, filepath.Join(pathDir, "go")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", pathDir)
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"doctor", "--json"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		var decoded struct {
			Status string `json:"status"`
			Checks []struct {
				ID      string `json:"id"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"checks"`
		}
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("expected JSON doctor output, got %q: %v", stdout, err)
		}
		if decoded.Status != "warning" || !doctorReportHasCheck(decoded.Checks, "optional_tools", "warning") {
			t.Fatalf("expected optional tool warning: %#v", decoded)
		}
		if !strings.Contains(stdout, "tailwindcss is not available on PATH") {
			t.Fatalf("expected tailwind warning in output:\n%s", stdout)
		}
	})
}

func TestBuildCommandWritesIndexHTML(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"

view {
  <main>
    <h1>GOWDK & friends</h1>
  </main>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, source}); err != nil {
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

func TestBuildCommandPrerendersSupportedStaticSlice(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "docs.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page docs.post
route "/docs/{slug}"

paths {
  => { slug: "getting-started" }
}

build {
  => { title: "Getting Started", tagline: "Portable Go web compiler" }
}

view {
  <main>
    <Hero title="{title}" tagline="{tagline}" />
    <p>{param("slug")}</p>
  </main>
}
`)
	writeCLIFile(t, component, `package app

component Hero

props {
  title string
  tagline string
}

view {
  <section><h1>{title}</h1><p>{tagline}</p></section>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, page, component}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(outputDir, "docs", "getting-started", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<section><h1>Getting Started</h1><p>Portable Go web compiler</p></section>`,
		`<p>getting-started</p>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in prerendered output:\n%s", expected, output)
		}
	}

	routeManifest, err := os.ReadFile(filepath.Join(outputDir, "gowdk-routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routeManifest), `"route": "/docs/getting-started"`) ||
		!strings.Contains(string(routeManifest), `"path": "docs/getting-started/index.html"`) {
		t.Fatalf("unexpected route manifest:\n%s", routeManifest)
	}
	for _, artifact := range []string{"gowdk-assets.json", "gowdk-build-report.json"} {
		if _, err := os.Stat(filepath.Join(outputDir, artifact)); err != nil {
			t.Fatalf("expected %s artifact: %v", artifact, err)
		}
	}
}

func TestBuildCommandDebugPrintsBuildgenReport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"

view {
  <main>Debuggable</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"build", "--config", config, "--debug", "--out", outputDir, source})
	})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(outputDir, "gowdk-build-report.json")
	if !strings.Contains(stdout, reportPath) {
		t.Fatalf("expected stdout to include build report path %q, got:\n%s", reportPath, stdout)
	}
	securityPath := filepath.Join(root, ".gowdk", "reports", "dist", "gowdk-security.json")
	if !strings.Contains(stdout, securityPath) {
		t.Fatalf("expected stdout to include security report path %q, got:\n%s", securityPath, stdout)
	}
	if !strings.Contains(stderr, "gowdk build report (build):") {
		t.Fatalf("expected debug report header on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "validate/ir_valid") || !strings.Contains(stderr, "complete/build_complete") {
		t.Fatalf("expected validation and completion events on stderr, got:\n%s", stderr)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected build report artifact: %v", err)
	}
	if _, err := os.Stat(securityPath); err != nil {
		t.Fatalf("expected external security report artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "gowdk-security.json")); !os.IsNotExist(err) {
		t.Fatalf("security report must not be written to served output root, stat err=%v", err)
	}
}

func TestBuildCommandDoesNotWriteTimingsByDefault(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>No timings</main>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, source}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, buildTimingsFile)); !os.IsNotExist(err) {
		t.Fatalf("expected timings sidecar to be disabled by default, stat err=%v", err)
	}
	reportPayload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(reportPayload), "duration") || strings.Contains(string(reportPayload), "timing") {
		t.Fatalf("expected deterministic build report without timing fields:\n%s", reportPayload)
	}
}

func TestBuildCommandWritesTimingsSidecar(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>Timed</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"build", "--config", config, "--timings", "--out", outputDir, source})
	})
	if err != nil {
		t.Fatal(err)
	}
	timingsPath := filepath.Join(outputDir, buildTimingsFile)
	if strings.Contains(stdout, timingsPath) || strings.Contains(stderr, timingsPath) {
		t.Fatalf("expected timings path to stay out of CLI streams, stdout=%q stderr=%q", stdout, stderr)
	}
	payload, err := os.ReadFile(timingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var report buildTimingReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("invalid timings JSON: %v\n%s", err, payload)
	}
	if report.Version != 1 || report.Mode != "build" || report.OutputDir != outputDir {
		t.Fatalf("unexpected timings report: %#v", report)
	}
	for _, phase := range []string{"config_load", "parse_lower", "go_binding", "ir_validation", "output_plan_writes"} {
		if !hasTimingPhase(report, phase) {
			t.Fatalf("expected timing phase %q in %#v", phase, report.Phases)
		}
	}
	if report.Counters["source_files"] != 1 || report.Counters["artifacts"] == 0 || report.Counters["files_written"] == 0 {
		t.Fatalf("unexpected timing counters: %#v", report.Counters)
	}
	reportPayload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(reportPayload), "durationMs") {
		t.Fatalf("expected build report to stay duration-free:\n%s", reportPayload)
	}
}

func TestBuildCommandWritesCustomTimingsPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	timingsPath := filepath.Join(root, "profiles", "build.json")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>Custom timings</main>
}
`)

	if err := run([]string{"build", "--config", config, "--timings=" + timingsPath, "--out", outputDir, source}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(timingsPath); err != nil {
		t.Fatalf("expected custom timings path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, buildTimingsFile)); !os.IsNotExist(err) {
		t.Fatalf("expected default timings path to be skipped when custom path is used, stat err=%v", err)
	}
}

func hasTimingPhase(report buildTimingReport, name string) bool {
	for _, phase := range report.Phases {
		if phase.Name == name {
			return true
		}
	}
	return false
}

func TestBuildCommandReportsBoundContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

page patients
route "/patients"

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, page, pageSource)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--config", config, "--out", outputDir, page}); err != nil {
			t.Fatal(err)
		}
	})

	payload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Events []struct {
			Stage string            `json:"stage"`
			Kind  string            `json:"kind"`
			Data  map[string]string `json:"data"`
		} `json:"events"`
	}
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("invalid build report: %v\n%s", err, payload)
	}
	for _, event := range report.Events {
		if event.Stage != "bind" || event.Kind != "contract_reference" {
			continue
		}
		if event.Data["name"] != "patients.CreatePatient" || event.Data["status"] != "bound" || event.Data["handler"] != "HandleCreatePatient" {
			t.Fatalf("unexpected contract reference event: %#v", event.Data)
		}
		if event.Data["type"] != "CreatePatient" || event.Data["result"] != "CreatePatientResult" {
			t.Fatalf("unexpected command type/result metadata: %#v", event.Data)
		}
		if event.Data["method"] != "POST" || event.Data["path"] != "/patients" {
			t.Fatalf("unexpected command method/path: %#v", event.Data)
		}
		wantColumn := strings.Index(testSourceLine(pageSource, 8), "g:command") + 1
		if event.Data["line"] != "9" || event.Data["column"] != strconv.Itoa(wantColumn) {
			t.Fatalf("unexpected command source location: %#v", event.Data)
		}
		return
	}
	t.Fatalf("missing contract_reference event in report: %s", payload)
}

func TestBuildReportsBoundQueryContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

page patients
route "/patients"

view {
  <main>
    <section g:query="patients.GetPatientPage">
      <h1>Patients</h1>
    </section>
  </main>
}
`
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, page, pageSource)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatientPage struct{}
type PatientPageData struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterQuery[GetPatientPage, PatientPageData](r, LoadPatientPage)
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{}, nil
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--config", config, "--out", outputDir, page}); err != nil {
			t.Fatal(err)
		}
	})

	payload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Events []struct {
			Stage string            `json:"stage"`
			Kind  string            `json:"kind"`
			Data  map[string]string `json:"data"`
		} `json:"events"`
	}
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("invalid build report: %v\n%s", err, payload)
	}
	for _, event := range report.Events {
		if event.Stage != "bind" || event.Kind != "contract_reference" {
			continue
		}
		if event.Data["kind"] != "query" || event.Data["name"] != "patients.GetPatientPage" || event.Data["status"] != "bound" || event.Data["handler"] != "LoadPatientPage" {
			t.Fatalf("unexpected contract reference event: %#v", event.Data)
		}
		if event.Data["type"] != "GetPatientPage" || event.Data["result"] != "PatientPageData" {
			t.Fatalf("unexpected query type/result metadata: %#v", event.Data)
		}
		wantColumn := strings.Index(testSourceLine(pageSource, 8), "g:query") + 1
		if event.Data["line"] != "9" || event.Data["column"] != strconv.Itoa(wantColumn) {
			t.Fatalf("unexpected query source location: %#v", event.Data)
		}
		return
	}
	t.Fatalf("missing contract_reference event in report: %s", payload)
}

func TestBuildWritesOpenAPIAndAsyncAPIReports(t *testing.T) {
	root := t.TempDir()
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/gowdk-specs

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, filepath.ToSlash(moduleRoot)))
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, page, `package pages

page newsletter
route "/newsletter"
guard public

act Subscribe POST "/newsletter"
api Health GET "/api/health"

view {
  <main>
    <form g:post={Subscribe}>
      <input name="email" />
    </form>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
    <section g:query="patients.GetPatientPage">
      <h2>Patients</h2>
    </section>
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "handlers.go"), `package pages

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

type SubscribeForm struct {
	Email string `+"`form:\"email\"`"+`
}

func Subscribe(ctx context.Context, input SubscribeForm) (response.Response, error) {
	return response.JSONValue(http.StatusOK, map[string]any{"ok": true})
}

func Health(ctx context.Context, request *http.Request) (response.Response, error) {
	return response.JSONValue(http.StatusOK, map[string]any{"ok": true})
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string `+"`form:\"name\"`"+`
}
type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}
type GetPatientPage struct {
	Search string `+"`form:\"search\"`"+`
}
type PatientPageData struct{}
type PatientSynced struct {
	ID string `+"`json:\"id\"`"+`
	Tags []string `+"`json:\"tags\"`"+`
}
type PatientCreated struct {
	ID string `+"`json:\"id\"`"+`
}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterQuery[GetPatientPage, PatientPageData](r, LoadPatientPage, contracts.RoleWeb)
	contracts.RegisterIntegrationEvent[PatientSynced](r, HandlePatientSynced, contracts.RoleWorker)
	contracts.RegisterDomainEvent[PatientCreated](r, HandlePatientCreated, contracts.RoleWorker)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{ID: command.Name}, nil
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{}, nil
}

func HandlePatientSynced(ctx context.Context, event PatientSynced) error {
	return nil
}

func HandlePatientCreated(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	var stdout string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"build", "--config", config, "--out", outputDir, page})
		})
		if err != nil {
			t.Fatal(err)
		}
		stdout = captured
	})
	openAPIPath := filepath.Join(outputDir, "openapi.json")
	asyncAPIPath := filepath.Join(outputDir, "asyncapi.json")
	if !strings.Contains(stdout, openAPIPath) || !strings.Contains(stdout, asyncAPIPath) {
		t.Fatalf("expected build stdout to include spec paths %q and %q, got:\n%s", openAPIPath, asyncAPIPath, stdout)
	}

	openAPI, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"openapi": "3.1.0"`,
		`"/newsletter"`,
		`"application/x-www-form-urlencoded"`,
		`"email"`,
		`"/api/health"`,
		`"patients.CreatePatient"`,
		`"CreatePatientResult"`,
		`"patients.GetPatientPage"`,
	} {
		if !strings.Contains(string(openAPI), expected) {
			t.Fatalf("expected OpenAPI report to contain %q:\n%s", expected, openAPI)
		}
	}

	asyncAPI, err := os.ReadFile(asyncAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"asyncapi": "3.0.0"`,
		`"integration.PatientSynced"`,
		`"PatientSynced"`,
		`"id"`,
		`"tags"`,
		`"cloudEvents"`,
	} {
		if !strings.Contains(string(asyncAPI), expected) {
			t.Fatalf("expected AsyncAPI report to contain %q:\n%s", expected, asyncAPI)
		}
	}
	if strings.Contains(string(asyncAPI), "PatientCreated") {
		t.Fatalf("domain events should be excluded from the default AsyncAPI report:\n%s", asyncAPI)
	}
}

func TestCheckJSONReportsMissingContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

page patients
route "/patients"

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	writeCLIFile(t, page, pageSource)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err == nil {
		t.Fatal("expected missing contract reference to fail check")
	}
	if !strings.Contains(output, `"code": "contract_reference_missing"`) ||
		!strings.Contains(output, "command patients.CreatePatient has no scanned Go registration") ||
		!strings.Contains(output, `"line": 9`) {
		t.Fatalf("expected missing contract diagnostic with source span, got:\n%s", output)
	}
}

func TestCheckJSONReportsVersionedSchema(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, page, `package pages

page home
route "/"
guard public

view {
  <main>Home</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"check", "--config", config, "--json", page})
	})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Version     int `json:"version"`
		Diagnostics []struct {
			File     string `json:"file"`
			Code     string `json:"code"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid check JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("expected check JSON schema version 1, got %d", report.Version)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", report.Diagnostics)
	}
}

func TestCheckJSONReportsRouteMethodConflictWithRelatedLocation(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "status.page.gwdk")
	writeCLIFile(t, page, `package pages

page status
route "/status"
guard public

api Health GET "/status"

view {
  <main>Status</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"check", "--config", config, "--json", page})
	})
	if err == nil {
		t.Fatal("expected route-method conflict to fail check")
	}
	var report struct {
		Version     int `json:"version"`
		Diagnostics []struct {
			Code  string `json:"code"`
			Range *struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
			Related []struct {
				File    string `json:"file"`
				Message string `json:"message"`
			} `json:"related"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid check JSON: %v\n%s", err, output)
	}
	if report.Version != 1 || len(report.Diagnostics) == 0 {
		t.Fatalf("expected versioned diagnostics, got %#v", report)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code != "route_method_conflict" {
			continue
		}
		if diagnostic.Range == nil || diagnostic.Range.Start.Line != 7 {
			t.Fatalf("expected conflict range on API declaration, got %#v", diagnostic.Range)
		}
		if len(diagnostic.Related) != 1 || !strings.Contains(diagnostic.Related[0].Message, "page status first declared here") {
			t.Fatalf("expected related page route location, got %#v", diagnostic.Related)
		}
		return
	}
	t.Fatalf("missing route_method_conflict diagnostic:\n%s", output)
}

func TestCheckJSONReportsInvalidContractRoute(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

page patients
route "/patients"

view {
  <main>
    <form method="post" action="https://example.com/pay" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	writeCLIFile(t, page, pageSource)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err == nil {
		t.Fatal("expected invalid contract route to fail check")
	}
	if !strings.Contains(output, `"code": "contract_route_invalid"`) ||
		!strings.Contains(output, `endpoint path \"https://example.com/pay\" must be a local absolute path`) ||
		!strings.Contains(output, `"line": 9`) {
		t.Fatalf("expected invalid contract route diagnostic with source span, got:\n%s", output)
	}
}

func TestCheckJSONReportsInvalidGoContractRegistration(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	writeCLIFile(t, page, `package pages

page patients
route "/patients"

view {
  <main>Patients</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
}

func HandleCreatePatient(ctx context.Context, command string) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err == nil {
		t.Fatal("expected invalid Go contract registration to fail check")
	}
	if !strings.Contains(output, `"code": "contract_handler_invalid"`) ||
		!strings.Contains(output, "second parameter must be CreatePatient") ||
		!strings.Contains(output, "contracts/patients.go") {
		t.Fatalf("expected invalid contract handler diagnostic, got:\n%s", output)
	}
}

func TestCheckJSONReportsMalformedGoEndpointComment(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "home.page.gwdk")
	writeCLIFile(t, page, `package pages

page home
route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "handlers.go"), `package pages

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

//gowdk:route GET /api/health
func Health(context.Context, *http.Request) (response.Response, error) {
	return response.Response{}, nil
}
`)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err == nil {
		t.Fatal("expected malformed Go endpoint comment to fail check")
	}
	if !strings.Contains(output, `"code": "malformed_go_endpoint_comment"`) ||
		!strings.Contains(output, "supported endpoint kinds are act and api") ||
		!strings.Contains(output, `"line": 10`) {
		t.Fatalf("expected malformed Go endpoint diagnostic with source span, got:\n%s", output)
	}
}

func TestBuildFailsForMissingContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

page patients
route "/patients"

view {
  <main g:query="patients.GetPatientPage">
    <h1>Patients</h1>
  </main>
}
`
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, page, pageSource)

	var err error
	withWorkingDir(t, root, func() {
		err = run([]string{"build", "--config", config, "--out", outputDir, page})
	})
	if err == nil {
		t.Fatal("expected missing query contract reference to fail build")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "gowdk-build-report.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected build to stop before writing output, stat err=%v", statErr)
	}
}

func TestBuildFailsForDuplicateCommandOwner(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, page, `package pages

page patients
route "/patients"

view {
  <main>Patients</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatientAgain)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}

func HandleCreatePatientAgain(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	var err error
	withWorkingDir(t, root, func() {
		err = run([]string{"build", "--config", config, "--out", outputDir, page})
	})
	if err == nil {
		t.Fatal("expected duplicate command owner to fail build")
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "gowdk-build-report.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected build to stop before writing output, stat err=%v", statErr)
	}
}

func TestInitCommandScaffoldsBuildableProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	if err := run([]string{"init", root}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		".gitignore",
		"gowdk.config.go",
		"src/pages/home.page.gwdk",
		"src/components/hero.cmp.gwdk",
		"styles/global.css",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected scaffold file %s: %v", path, err)
		}
	}
	ignorePayload, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"gowdk_cache/", ".gowdk/", "bin/"} {
		if !strings.Contains(string(ignorePayload), expected) {
			t.Fatalf("expected scaffold .gitignore to contain %q, got:\n%s", expected, ignorePayload)
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
	payload, err := os.ReadFile(filepath.Join(root, ".gowdk", "output", "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "Hello from GOWDK") {
		t.Fatalf("unexpected initialized build output:\n%s", payload)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "site")); err != nil {
		t.Fatalf("expected initialized build to create binary: %v", err)
	}
	if payload, err := os.ReadFile(filepath.Join(root, "gowdk.config.go")); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(payload), "Output:") {
		t.Fatalf("expected scaffold config to omit inferred Output:\n%s", payload)
	}
}

func TestInitCommandSupportsMinimalTemplate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "minimal")
	if err := run([]string{"init", "--template", "minimal", root}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		".gitignore",
		"gowdk.config.go",
		"src/pages/home.page.gwdk",
		"styles/global.css",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected scaffold file %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "src", "components", "hero.cmp.gwdk")); !os.IsNotExist(err) {
		t.Fatalf("minimal template should not create hero component, got %v", err)
	}

	withWorkingDir(t, root, func() {
		if err := run([]string{"build"}); err != nil {
			t.Fatal(err)
		}
	})
	payload, err := os.ReadFile(filepath.Join(root, ".gowdk", "output", "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "<h1>GOWDK</h1>") {
		t.Fatalf("unexpected minimal build output:\n%s", payload)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "site")); err != nil {
		t.Fatalf("expected minimal build to create binary: %v", err)
	}
}

func TestInitCommandSupportsOptionalTestScaffold(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	if err := run([]string{"init", "--tests", root}); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(root, "tests", "gowdk_smoke_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"package gowdktest",
		`os.Getenv("GOWDK_BIN")`,
		`t.Skip("set GOWDK_BIN=/path/to/gowdk to run generated app smoke tests")`,
		`exec.Command(gowdkBin, "build")`,
		`filepath.Join(projectRoot, "bin", "site")`,
		`cmd.Dir = projectRoot`,
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected optional test scaffold to contain %q:\n%s", expected, payload)
		}
	}
}

func TestInitCommandRejectsUnknownTemplate(t *testing.T) {
	err := run([]string{"init", "--template", "admin", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), `unknown init template "admin"`) {
		t.Fatalf("expected unknown template error, got %v", err)
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

func TestAddCommandListsKnownAddons(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "--list"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"Available addons:",
		"actions",
		"api",
		"auth",
		"contracts",
		"css",
		"db",
		"embed",
		"partial",
		"ratelimit",
		"seo",
		"realtime",
		"ssr",
		"static",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected %q in addon list:\n%s", expected, stdout)
		}
	}
}

func TestAddCommandListsRegistryMetadata(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "--list", "--registry"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, expected := range []string{
		"Addon registry (metadata only",
		"KIND",
		"LIFECYCLE",
		"COMPAT",
		"actions",
		"built-in",
		"stable",
		"compatible",
		"tailwind",
		"experimental",
		"no",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected %q in registry list:\n%s", expected, stdout)
		}
	}
}

func TestAddCommandListsRegistryJSON(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "--list", "--registry", "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded addonregistry.Registry
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected registry JSON, got %q: %v", stdout, err)
	}
	var found bool
	for _, entry := range decoded.Addons {
		if entry.Name == "seo" && entry.Constructor.Addable && entry.Constructor.OptionsCLI == "--base-url" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected SEO registry metadata in JSON: %#v", decoded.Addons)
	}
}

func TestAddonRegistryListDistinguishesDiscoveryCategories(t *testing.T) {
	var builder strings.Builder
	writeAddonRegistryList(&builder, []addonregistry.Entry{
		{
			Name:          "builtin",
			Kind:          "built-in",
			Lifecycle:     "stable",
			Compatibility: "compatible",
			Summary:       "builtin addon",
			Constructor:   addonregistry.Constructor{Addable: true},
		},
		{
			Name:          "external",
			Kind:          "documented-external",
			Lifecycle:     "experimental",
			Compatibility: "compatible",
			Summary:       "external addon",
		},
		{
			Name:          "old",
			Kind:          "built-in",
			Lifecycle:     "deprecated",
			Compatibility: "incompatible",
			Summary:       "old addon",
		},
	})
	output := builder.String()
	for _, expected := range []string{
		"built-in",
		"documented-external",
		"experimental",
		"deprecated",
		"incompatible",
		"yes",
		"no",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in registry list:\n%s", expected, output)
		}
	}
}

func TestAddCommandWiresAddonIntoConfig(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "ssr", "--config", config})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `added addon "ssr"`) {
		t.Fatalf("expected add confirmation, got:\n%s", stdout)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`"github.com/cssbruno/gowdk/addons/ssr"`,
		"Addons: []gowdk.Addon{ssr.Addon()}",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected updated config to contain %q:\n%s", expected, source)
		}
	}
}

func TestAddCommandWiresSEOAddonWithOptions(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "seo", "--base-url", "https://example.com", "--config", config})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `added addon "seo"`) {
		t.Fatalf("expected add confirmation, got:\n%s", stdout)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`"github.com/cssbruno/gowdk/addons/seo"`,
		`Addons: []gowdk.Addon{seo.Addon(seo.Options{BaseURL: "https://example.com"})}`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected updated config to contain %q:\n%s", expected, source)
		}
	}
}

func TestAddCommandRejectsSEOAddonWithoutBaseURL(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "seo", "--config", config})
	})
	if err == nil || !strings.Contains(err.Error(), "gowdk add seo requires --base-url <url>") {
		t.Fatalf("expected missing SEO base URL error, got %v", err)
	}
	payload, readErr := os.ReadFile(config)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(payload), "seo.Addon") {
		t.Fatalf("seo addon should not be written on missing base URL:\n%s", payload)
	}
}

func TestAddCommandWiresRealtimeAddonIntoConfig(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "realtime", "--config", config})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `added addon "realtime"`) {
		t.Fatalf("expected add confirmation, got:\n%s", stdout)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		`"github.com/cssbruno/gowdk/addons/realtime"`,
		"Addons: []gowdk.Addon{realtime.Addon()}",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected updated config to contain %q:\n%s", expected, source)
		}
	}
}

func TestAddCommandSkipsExistingAliasedAddon(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import (
	"github.com/cssbruno/gowdk"
	gowdkssr "github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		gowdkssr.Addon(),
	},
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "ssr", "--config", config})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `addon "ssr" already present`) {
		t.Fatalf("expected already-present message, got:\n%s", stdout)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	if strings.Count(source, "github.com/cssbruno/gowdk/addons/ssr") != 1 {
		t.Fatalf("expected one ssr import after idempotent add:\n%s", source)
	}
	if strings.Count(source, ".Addon()") != 1 {
		t.Fatalf("expected one addon constructor after idempotent add:\n%s", source)
	}
}

func TestAddCommandRejectsNonLiteralAddonsField(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var existingAddons []gowdk.Addon

var Config = gowdk.Config{
	Addons: existingAddons,
}
`)

	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"add", "ssr", "--config", config})
	})
	if err == nil || !strings.Contains(err.Error(), "Config.Addons must be a []gowdk.Addon literal") {
		t.Fatalf("expected non-literal Addons error, got %v", err)
	}
}

func TestAddCommandSupportsConfigEqualsFlag(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "custom.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	if err := run([]string{"add", "partial", "--config=" + config}); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "partial.Addon()") {
		t.Fatalf("expected partial addon in config:\n%s", payload)
	}
}

func TestAddCommandRejectsUnknownFlag(t *testing.T) {
	err := run([]string{"add", "--unknown"})
	if err == nil || !strings.Contains(err.Error(), `unknown add flag "--unknown"`) {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestDevRejectsInvalidInterval(t *testing.T) {
	err := run([]string{"dev", "--interval", "0s", "--out", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "dev interval must be positive") {
		t.Fatalf("expected invalid interval error, got %v", err)
	}
}

func TestBuildCommandRequiresProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	withWorkingDir(t, root, func() {
		err := run([]string{"build", "--out", "dist", "home.page.gwdk"})
		if err == nil || !strings.Contains(err.Error(), "gowdk.config.go is required") {
			t.Fatalf("expected required config error, got %v", err)
		}
	})
}

func TestParseDevOptions(t *testing.T) {
	options, err := parseDevOptions([]string{"--addr", "127.0.0.1:8090", "--interval=250ms", "--out", "dist", "home.page.gwdk"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Addr != "127.0.0.1:8090" {
		t.Fatalf("unexpected addr: %q", options.Addr)
	}
	if options.Interval != 250*time.Millisecond {
		t.Fatalf("unexpected interval: %s", options.Interval)
	}
	if strings.Join(options.BuildArgs, " ") != "--out dist home.page.gwdk" {
		t.Fatalf("unexpected build args: %#v", options.BuildArgs)
	}
}

func TestDevOutputDirUsesConfiguredTarget(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "admin", Output: "dist/admin"},
		},
	},
}
`)

	withWorkingDir(t, root, func() {
		outputDir, err := devOutputDir([]string{"--target", "admin"})
		if err != nil {
			t.Fatal(err)
		}
		if outputDir != "dist/admin" {
			t.Fatalf("unexpected dev output dir: %q", outputDir)
		}
	})
}

func TestDevOutputDirDefaultsToCache(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{Output: "dist/site"},
}
`)

	withWorkingDir(t, root, func() {
		outputDir, err := devOutputDir(nil)
		if err != nil {
			t.Fatal(err)
		}
		if outputDir != "gowdk_cache" {
			t.Fatalf("unexpected dev output dir: %q", outputDir)
		}
		buildArgs, outputDir, err := devBuildArgs(nil)
		if err != nil {
			t.Fatal(err)
		}
		if outputDir != "gowdk_cache" {
			t.Fatalf("unexpected dev build output dir: %q", outputDir)
		}
		if strings.Join(buildArgs, " ") != "--out gowdk_cache" {
			t.Fatalf("unexpected dev build args: %#v", buildArgs)
		}
	})
}

func TestDevOutputDirDefaultsToCacheWithConfiguredTargets(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "admin", Output: "dist/admin"},
			{Name: "public", Output: "dist/public"},
		},
	},
}
`)

	withWorkingDir(t, root, func() {
		outputDir, err := devOutputDir(nil)
		if err != nil {
			t.Fatal(err)
		}
		if outputDir != "gowdk_cache" {
			t.Fatalf("unexpected dev output dir: %q", outputDir)
		}
	})
}

func TestDevRuntimePlanAddsBinaryForAppBuild(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)

	withWorkingDir(t, root, func() {
		runtime, args, err := devRuntimePlan([]string{"--out", "dist", "--app", ".gowdk/app"}, "dist")
		if err != nil {
			t.Fatal(err)
		}
		if !runtime.Enabled {
			t.Fatal("expected dev runtime to be enabled")
		}
		if runtime.AppDir != ".gowdk/app" {
			t.Fatalf("unexpected app dir: %q", runtime.AppDir)
		}
		if runtime.BinaryPath != filepath.Join("dist", ".gowdk", "dev", "app") {
			t.Fatalf("unexpected binary path: %q", runtime.BinaryPath)
		}
		if strings.Join(args, " ") != "--out dist --app .gowdk/app" {
			t.Fatalf("unexpected dev build args: %#v", args)
		}
	})
}

func TestDevRuntimePlanUsesConfiguredTargetApp(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "site", Output: "dist/site", App: ".gowdk/site"},
		},
	},
}
`)

	withWorkingDir(t, root, func() {
		runtime, args, err := devRuntimePlan([]string{"--target", "site"}, "dist/site")
		if err != nil {
			t.Fatal(err)
		}
		if !runtime.Enabled {
			t.Fatal("expected dev runtime to be enabled")
		}
		if runtime.AppDir != ".gowdk/site" {
			t.Fatalf("unexpected app dir: %q", runtime.AppDir)
		}
		if runtime.BinaryPath != filepath.Join("dist", "site", ".gowdk", "dev", "app") {
			t.Fatalf("unexpected binary path: %q", runtime.BinaryPath)
		}
		if strings.Join(args, " ") != "--target site" {
			t.Fatalf("unexpected dev build args: %#v", args)
		}
	})
}

func TestDevStartupAndRebuildLogLinesDescribeStaticAndRuntimeModes(t *testing.T) {
	staticState := devBuildState{}
	staticDir := filepath.Join("tmp", "gowdk-build")
	if got := devStartupLine(staticState, staticDir, "127.0.0.1:8080", ""); got != "Static dev server: serving "+staticDir+" at http://127.0.0.1:8080" {
		t.Fatalf("unexpected static startup line: %q", got)
	}
	if got := devRebuildCompleteLine(staticState, staticDir, "127.0.0.1:8080", ""); got != "Dev rebuild complete: static output refreshed at "+staticDir {
		t.Fatalf("unexpected static rebuild line: %q", got)
	}

	runtimeState := devBuildState{runtime: devRuntime{Enabled: true, BinaryPath: filepath.Join("dist", ".gowdk", "dev", "app")}}
	if got := devStartupLine(runtimeState, staticDir, "127.0.0.1:8080", "127.0.0.1:39001"); got != "Generated app runtime: proxy http://127.0.0.1:8080 -> http://127.0.0.1:39001 (binary "+runtimeState.runtime.BinaryPath+")" {
		t.Fatalf("unexpected runtime startup line: %q", got)
	}
	if got := devRebuildCompleteLine(runtimeState, staticDir, "127.0.0.1:8080", "127.0.0.1:39002"); got != "Dev rebuild complete: generated app restarted: proxy http://127.0.0.1:8080 -> http://127.0.0.1:39002 (binary "+runtimeState.runtime.BinaryPath+")" {
		t.Fatalf("unexpected runtime rebuild line: %q", got)
	}
}

func TestDevRuntimeProcessRestartWaitsForPreviousAppExit(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	writeCLIFile(t, source, `package main

func main() {
	select {}
}
`)
	binary := filepath.Join(root, "app")
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper app: %v\n%s", err, output)
	}

	process := &devRuntimeProcess{
		plan: devRuntime{Enabled: true, BinaryPath: binary},
		addr: "127.0.0.1:0",
	}
	if err := process.restart(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(process.stop)
	first := activeDevRuntimeCommand(t, process)

	if err := process.restart(); err != nil {
		t.Fatal(err)
	}
	if first.ProcessState == nil {
		t.Fatal("expected restart to wait for the previous app process to exit")
	}
	second := activeDevRuntimeCommand(t, process)
	if second == first {
		t.Fatal("expected restart to launch a new app process")
	}

	process.stop()
	if second.ProcessState == nil {
		t.Fatal("expected stop to wait for the active app process to exit")
	}
	process.mu.Lock()
	active := process.cmd
	waitDone := process.waitDone
	process.mu.Unlock()
	if active != nil || waitDone != nil {
		t.Fatalf("expected stopped process state to be cleared, got cmd=%v waitDone=%v", active, waitDone)
	}
}

func TestDevServeStateRebindsStaticOutputDir(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "dist-one")
	secondDir := filepath.Join(root, "dist-two")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatal(err)
	}
	firstAbs, err := filepath.Abs(firstDir)
	if err != nil {
		t.Fatal(err)
	}
	secondAbs, err := filepath.Abs(secondDir)
	if err != nil {
		t.Fatal(err)
	}

	serve := newDevServeState("127.0.0.1:0")
	t.Cleanup(serve.close)
	serve.useStatic(firstAbs)
	firstServer := serve.server
	if firstServer == nil || serve.staticDir != firstAbs {
		t.Fatalf("expected initial static server for %q, got server=%v dir=%q", firstAbs, firstServer, serve.staticDir)
	}

	serve.useStatic(firstAbs)
	if serve.server != firstServer {
		t.Fatal("expected unchanged static output dir to keep the existing server")
	}

	serve.useStatic(secondAbs)
	if serve.server == nil || serve.staticDir != secondAbs {
		t.Fatalf("expected rebound static server for %q, got server=%v dir=%q", secondAbs, serve.server, serve.staticDir)
	}
	if serve.server == firstServer {
		t.Fatal("expected output dir change to replace the static server")
	}
}

func TestDevServeStateStartsRuntimeAfterStaticMode(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "dist")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "app")
	writeCLIFile(t, filepath.Join(appDir, "go.mod"), "module example.com/devapp\n\ngo 1.24\n")
	writeCLIFile(t, filepath.Join(appDir, "cmd", "server", "main.go"), `package main

func main() {
	select {}
}
`)
	binaryPath := filepath.Join(root, "bin", "devapp")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}

	serve := newDevServeState("127.0.0.1:0")
	t.Cleanup(serve.close)
	serve.useStatic(staticDir)
	if serve.server == nil {
		t.Fatal("expected static server before runtime transition")
	}

	runtime := devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}
	if err := serve.useRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	if serve.server == nil || serve.staticDir != "" {
		t.Fatalf("expected runtime proxy server after runtime transition, got server=%v dir=%q", serve.server, serve.staticDir)
	}
	if serve.process == nil {
		t.Fatal("expected runtime process to start after runtime transition")
	}
	if serve.process.plan != runtime {
		t.Fatalf("unexpected runtime plan: %#v", serve.process.plan)
	}
	command := activeDevRuntimeCommand(t, serve.process)
	if serve.process.addr == serve.addr || serve.process.addr == "" {
		t.Fatalf("expected generated app to use an internal runtime address, got %q", serve.process.addr)
	}
	if !hasEnvValue(command.Env, "GOWDK_ADDR="+serve.process.addr) {
		t.Fatalf("expected generated app GOWDK_ADDR=%q, env=%#v", serve.process.addr, command.Env)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("expected runtime binary to be built: %v", err)
	}
}

func hasEnvValue(env []string, value string) bool {
	for _, item := range env {
		if item == value {
			return true
		}
	}
	return false
}

func activeDevRuntimeCommand(t *testing.T, process *devRuntimeProcess) *exec.Cmd {
	t.Helper()
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.cmd == nil {
		t.Fatal("expected active dev runtime command")
	}
	return process.cmd
}

func TestParsePreviewOptions(t *testing.T) {
	options, err := parsePreviewOptions([]string{"--addr=127.0.0.1:9090", "--hot", "--out", "preview-dist", "home.page.gwdk"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Addr != "127.0.0.1:9090" {
		t.Fatalf("unexpected addr: %q", options.Addr)
	}
	if !options.Hot {
		t.Fatal("expected hot preview")
	}
	if options.OutputDir != "preview-dist" {
		t.Fatalf("unexpected preview output dir: %q", options.OutputDir)
	}
	if strings.Join(options.BuildArgs, " ") != "--out preview-dist home.page.gwdk" {
		t.Fatalf("unexpected preview build args: %#v", options.BuildArgs)
	}
}

func TestPreviewBuildArgsInjectsOutput(t *testing.T) {
	args := previewBuildArgs([]string{"home.page.gwdk"}, "preview")
	if strings.Join(args, " ") != "home.page.gwdk --out preview" {
		t.Fatalf("unexpected preview build args: %#v", args)
	}
	args = previewBuildArgs([]string{"--out", "custom", "home.page.gwdk"}, "preview")
	if strings.Join(args, " ") != "--out custom home.page.gwdk" {
		t.Fatalf("unexpected explicit preview build args: %#v", args)
	}
}

func TestPlaygroundPolicyPrintsSandboxDefaults(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"playground", "policy", "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		HostedExecutionEnabled bool `json:"hostedExecutionEnabled"`
		Limits                 struct {
			MaxFiles int `json:"maxFiles"`
		} `json:"limits"`
		Environment []string `json:"environment"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected playground policy JSON, got %q: %v", stdout, err)
	}
	if decoded.HostedExecutionEnabled || decoded.Limits.MaxFiles == 0 {
		t.Fatalf("unexpected playground policy: %#v", decoded)
	}
	if !strings.Contains(stdout, "GOPROXY=off") {
		t.Fatalf("expected network-disabled Go environment in policy:\n%s", stdout)
	}
}

func TestPlaygroundExportArchivesSourceProjectOnly(t *testing.T) {
	root := t.TempDir()
	writeMinimalPlaygroundProject(t, root)
	writeCLIFile(t, filepath.Join(root, ".env"), "SECRET=value")
	writeCLIFile(t, filepath.Join(root, ".gowdk", "app", "main.go"), "package main\n")
	writeCLIFile(t, filepath.Join(root, "dist", "index.html"), "<html></html>")
	archivePath := filepath.Join(t.TempDir(), "project.zip")

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"playground", "export", "--dir", root, "--out", archivePath, "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Archive string `json:"archive"`
		Files   []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected export JSON, got %q: %v", stdout, err)
	}
	if decoded.Archive != archivePath {
		t.Fatalf("unexpected archive path: %#v", decoded)
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	joined := strings.Join(names, ",")
	for _, expected := range []string{"gowdk.config.go", "src/pages/home.page.gwdk", "styles/global.css"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %s in playground archive: %#v", expected, names)
		}
	}
	for _, forbidden := range []string{".env", ".gowdk", "dist"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("did not expect %s in playground archive: %#v", forbidden, names)
		}
	}
}

func TestPlaygroundRunRequiresExplicitExecutionOptIn(t *testing.T) {
	root := t.TempDir()
	writeMinimalPlaygroundProject(t, root)
	err := run([]string{"playground", "run", "--dir", root, "--out", filepath.Join(t.TempDir(), "dist")})
	if err == nil || !strings.Contains(err.Error(), "disabled by default") {
		t.Fatalf("expected disabled playground execution error, got %v", err)
	}
}

func TestPlaygroundRunBuildsFromStagedWorkspace(t *testing.T) {
	root := t.TempDir()
	writeMinimalPlaygroundProject(t, root)
	outputDir := filepath.Join(t.TempDir(), "dist")
	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"playground", "run", "--dir", root, "--out", outputDir, "--allow-hosted-execution"})
	})
	if err != nil {
		t.Fatalf("playground run failed: %v\n%s", err, stdout)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "index.html")); err != nil {
		t.Fatalf("expected playground output index.html: %v\n%s", err, stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("playground run should not write generated output into source root, err=%v", err)
	}
}

func writeMinimalPlaygroundProject(t *testing.T, root string) {
	t.Helper()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "pages", "home.page.gwdk"), `package app

route "/"
guard public
css default

view {
  <main>Hello playground</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "styles", "global.css"), `body {
  font-family: system-ui, sans-serif;
}
`)
}

func TestDevInputCacheFreshRequiresMatchingSnapshotAndOutput(t *testing.T) {
	outputDir := t.TempDir()
	snapshot := inputSnapshot{"/tmp/home.page.gwdk": "abc"}
	if err := writeDevInputCache(outputDir, snapshot); err != nil {
		t.Fatal(err)
	}
	if devInputCacheFresh(outputDir, snapshot) {
		t.Fatal("expected cache miss without generated output files")
	}
	writeCLIFile(t, filepath.Join(outputDir, "index.html"), "<main>ok</main>")
	if !devInputCacheFresh(outputDir, snapshot) {
		t.Fatal("expected cache hit with matching snapshot and output")
	}
	if devInputCacheFresh(outputDir, inputSnapshot{"/tmp/home.page.gwdk": "changed"}) {
		t.Fatal("expected cache miss for changed snapshot")
	}
}

func TestBuildInputSnapshotDetectsFileChanges(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"

view {
  <main>Before</main>
}
`)

	first, err := buildInputSnapshot([]string{"--config", config, "--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	writeCLIFile(t, source, `package app

page home
route "/"

view {
  <main>After</main>
}
`)
	second, err := buildInputSnapshot([]string{"--config", config, "--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	if second.same(first) {
		t.Fatalf("expected changed source snapshot: first=%#v second=%#v", first, second)
	}
}

func TestBuildInputSnapshotIgnoresUnchangedFileTouches(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	content := `package app

page home
route "/"

view {
  <main>Same</main>
}
`
	writeCLIFile(t, source, content)

	first, err := buildInputSnapshot([]string{"--config", config, "--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	writeCLIFile(t, source, content)
	second, err := buildInputSnapshot([]string{"--config", config, "--out", filepath.Join(root, "dist"), source})
	if err != nil {
		t.Fatal(err)
	}
	if !second.same(first) {
		t.Fatalf("expected unchanged source snapshot: first=%#v second=%#v", first, second)
	}
}

func TestBuildInputSnapshotDetectsConfiguredCSSChanges(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{Include: []string{"*.gwdk"}},
	Build: gowdk.BuildConfig{Output: "dist/site"},
	CSS: gowdk.CSSConfig{Include: []string{"styles/*.css"}},
}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Styled</main>
}
`)
	stylesheet := filepath.Join(root, "styles", "login.css")
	writeCLIFile(t, stylesheet, ".login { color: red; }\n")

	withWorkingDir(t, root, func() {
		first, err := buildInputSnapshot(nil)
		if err != nil {
			t.Fatal(err)
		}
		writeCLIFile(t, stylesheet, ".login { color: blue; }\n")
		second, err := buildInputSnapshot(nil)
		if err != nil {
			t.Fatal(err)
		}
		if second.same(first) {
			t.Fatalf("expected changed css snapshot: first=%#v second=%#v", first, second)
		}
	})
}

func TestBuildInputSnapshotExcludesGeneratedCSSOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{Include: []string{"*.gwdk"}},
	Build: gowdk.BuildConfig{Output: "dist/site"},
}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Styled</main>
}
`)
	generatedCSS := filepath.Join(root, "dist", "site", "assets", "gowdk", "home.css")
	writeCLIFile(t, generatedCSS, ".generated { color: red; }\n")

	withWorkingDir(t, root, func() {
		snapshot, err := buildInputSnapshot(nil)
		if err != nil {
			t.Fatal(err)
		}
		absGenerated, err := filepath.Abs(generatedCSS)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := snapshot[absGenerated]; ok {
			t.Fatalf("expected generated css output to be excluded from snapshot: %#v", snapshot)
		}
	})
}

func TestDevBuildStateConfigChangeRequestsReload(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{Include: []string{"*.gwdk"}},
}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	withWorkingDir(t, root, func() {
		state, err := newDevBuildState(nil)
		if err != nil {
			t.Fatal(err)
		}
		first, err := state.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{Include: []string{"pages/*.gwdk"}},
}
`)
		second, err := state.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		change := second.diff(first)
		if !state.configChanged(change) {
			t.Fatalf("expected config change to request a dev plan reload: %#v", change)
		}
	})
}

func TestDevInputTrackerDetectsAddedSourceBeforeRediscovery(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{Include: []string{"*.gwdk"}},
}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	withWorkingDir(t, root, func() {
		state, err := newDevBuildState(nil)
		if err != nil {
			t.Fatal(err)
		}
		first, err := state.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		about := filepath.Join(root, "about.page.gwdk")
		writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>About</main>
}
`)
		second, err := waitForSnapshotChange(state, first)
		if err != nil {
			t.Fatal(err)
		}
		change := second.diff(first)
		if len(change.Changed) == 0 {
			t.Fatalf("expected cached tracker to detect a directory change for the added source: %#v", change)
		}

		refreshed, err := newDevInputTracker(state.plan)
		if err != nil {
			t.Fatal(err)
		}
		refreshedSnapshot, err := refreshed.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		absAbout, err := canonicalInputPath(about)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := refreshedSnapshot[absAbout]; !ok {
			t.Fatalf("expected refreshed tracker to include added source %q: %#v", absAbout, refreshedSnapshot)
		}
	})
}

func waitForSnapshotChange(state devBuildState, previous inputSnapshot) (inputSnapshot, error) {
	deadline := time.Now().Add(time.Second)
	for {
		current, err := state.snapshot()
		if err != nil {
			return nil, err
		}
		if !current.same(previous) {
			return current, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("snapshot did not change before deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestInputSnapshotDiffReportsChangedAddedAndRemovedPaths(t *testing.T) {
	current := inputSnapshot{
		"/tmp/b.page.gwdk": "changed",
		"/tmp/c.page.gwdk": "added",
	}
	previous := inputSnapshot{
		"/tmp/a.page.gwdk": "removed",
		"/tmp/b.page.gwdk": "before",
	}

	change := current.diff(previous)
	if strings.Join(change.Changed, ",") != "/tmp/b.page.gwdk" {
		t.Fatalf("unexpected changed paths: %#v", change.Changed)
	}
	if strings.Join(change.Added, ",") != "/tmp/c.page.gwdk" {
		t.Fatalf("unexpected added paths: %#v", change.Added)
	}
	if strings.Join(change.Removed, ",") != "/tmp/a.page.gwdk" {
		t.Fatalf("unexpected removed paths: %#v", change.Removed)
	}
}

func TestInputChangeDetailsReportsChangedAddedAndRemovedPaths(t *testing.T) {
	root := t.TempDir()
	changed := filepath.Join(root, "changed.page.gwdk")
	added := filepath.Join(root, "added.page.gwdk")
	removed := filepath.Join(root, "removed.page.gwdk")

	withWorkingDir(t, root, func() {
		change := inputChange{
			Changed: []string{changed},
			Added:   []string{added},
			Removed: []string{removed},
		}
		details := strings.Join(change.details(), "\n")
		for _, expected := range []string{
			"changed: changed.page.gwdk",
			"added: added.page.gwdk",
			"removed: removed.page.gwdk",
		} {
			if !strings.Contains(details, expected) {
				t.Fatalf("expected change details to contain %q, got:\n%s", expected, details)
			}
		}
	})
}

func TestInputChangeDetailsRelativizesSymlinkedWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "real")
	linkRoot := filepath.Join(root, "link")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	changed := filepath.Join(linkRoot, "changed.page.gwdk")

	withWorkingDir(t, realRoot, func() {
		change := inputChange{Changed: []string{changed}}
		details := strings.Join(change.details(), "\n")
		if !strings.Contains(details, "changed: changed.page.gwdk") {
			t.Fatalf("expected symlinked change detail to be relative, got:\n%s", details)
		}
	})
}

func TestBuildIncrementalSPAUsesChangedPageSources(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, home, `package app

page home
route "/"

view {
  <main>Before</main>
}
`)
	writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>Stable</main>
}
`)

	args := []string{"--config", config, "--out", outputDir, home, about}
	if err := build(args); err != nil {
		t.Fatal(err)
	}
	aboutPath := filepath.Join(outputDir, "about", "index.html")
	aboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	writeCLIFile(t, home, `package app

page home
route "/"

view {
  <main>After</main>
}
`)
	used, err := buildIncrementalSPA(args, inputChange{Changed: []string{home}})
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected incremental spa build to handle page source change")
	}
	homePayload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homePayload), "After") {
		t.Fatalf("expected changed home output:\n%s", homePayload)
	}
	afterAboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterAboutInfo.ModTime().Equal(aboutInfo.ModTime()) {
		t.Fatalf("expected unchanged about output mod time: before=%s after=%s", aboutInfo.ModTime(), afterAboutInfo.ModTime())
	}
}

func TestBuildIncrementalSPAUsesComponentDependencies(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main><Hero title="GOWDK" /></main>
}
`)
	writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>Stable</main>
}
`)
	writeCLIFile(t, component, `package app

component Hero

props {
  title string
}

view {
  <h1>{title}</h1>
}
`)

	args := []string{"--config", config, "--timings", "--out", outputDir, page, about, component}
	if err := build(args); err != nil {
		t.Fatal(err)
	}
	aboutPath := filepath.Join(outputDir, "about", "index.html")
	aboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	writeCLIFile(t, component, `package app

component Hero

props {
  title string
}

view {
  <h1>{title} after</h1>
}
`)
	used, err := buildIncrementalSPA(args, inputChange{Changed: []string{component}})
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected incremental spa build to handle component dependency change")
	}
	homePayload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homePayload), "GOWDK after") {
		t.Fatalf("expected changed component output:\n%s", homePayload)
	}
	afterAboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterAboutInfo.ModTime().Equal(aboutInfo.ModTime()) {
		t.Fatalf("expected unchanged about output mod time: before=%s after=%s", aboutInfo.ModTime(), afterAboutInfo.ModTime())
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, buildTimingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var timings buildTimingReport
	if err := json.Unmarshal(payload, &timings); err != nil {
		t.Fatalf("invalid timings JSON: %v\n%s", err, payload)
	}
	if timings.Counters["incremental_component_changes"] != 1 || timings.Counters["incremental_affected_pages"] != 1 {
		t.Fatalf("expected incremental dependency counters, got %#v", timings.Counters)
	}
	if _, ok := timings.Counters["files_written"]; !ok {
		t.Fatalf("expected incremental write counters, got %#v", timings.Counters)
	}
}

func TestDevComponentHMRPayloadUsesLayoutComponentDependencies(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	layout := filepath.Join(root, "root.layout.gwdk")
	component := filepath.Join(root, "brand.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"
layout root

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, layout, `package app

view {
  <section><Brand /><slot /></section>
}
`)
	writeCLIFile(t, component, `package app

component Brand

client {
  func Toggle() {}
}

view {
  <button g:on:click={Toggle()}>Brand</button>
}
`)

	plan, err := loadBuildOptions([]string{"--config", config, "--out", outputDir, page, layout, component})
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := devComponentHMRPayloadLoaded(plan, inputChange{Changed: []string{component}})
	if !ok {
		t.Fatal("expected component HMR payload for layout component dependency")
	}
	var decoded devComponentHMRPayload
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("invalid HMR payload JSON: %v\n%s", err, payload)
	}
	if len(decoded.Components) != 1 || decoded.Components[0].Name != "Brand" || decoded.Components[0].ID != "app.Brand" {
		t.Fatalf("unexpected HMR components: %#v", decoded.Components)
	}
	if strings.Join(decoded.Routes, ",") != "/" {
		t.Fatalf("unexpected HMR routes: %#v", decoded.Routes)
	}
	if decoded.Generated == "" {
		t.Fatalf("expected generated timestamp in payload: %#v", decoded)
	}
}

func TestDevComponentHMRFallsBackForChangedPageAndKeepsStaleRouteCleanup(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/old"

view {
  <main>Old</main>
}
`)
	args := []string{"--config", config, "--out", outputDir, page}
	if err := build(args); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(outputDir, "old", "index.html")
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal(err)
	}

	writeCLIFile(t, page, `package app

page home
route "/new"

view {
  <main>New</main>
}
`)
	plan, err := loadBuildOptions(args)
	if err != nil {
		t.Fatal(err)
	}
	if payload, ok := devComponentHMRPayloadLoaded(plan, inputChange{Changed: []string{page}}); ok {
		t.Fatalf("expected changed page to fall back to reload, got HMR payload %s", payload)
	}
	used, err := buildIncrementalSPA(args, inputChange{Changed: []string{page}})
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected incremental SPA build to handle changed page route")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old route output to be removed, stat err: %v", err)
	}
	newPayload, err := os.ReadFile(filepath.Join(outputDir, "new", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newPayload), "New") {
		t.Fatalf("expected new route output:\n%s", newPayload)
	}
}

func TestBuildIncrementalSPAUsesLayoutComponentDependencies(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	layout := filepath.Join(root, "root.layout.gwdk")
	component := filepath.Join(root, "brand.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"
layout root

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>Stable</main>
}
`)
	writeCLIFile(t, layout, `package app

view {
  <section><Brand /><slot /></section>
}
`)
	writeCLIFile(t, component, `package app

component Brand

view {
  <strong>Before</strong>
}
`)

	args := []string{"--config", config, "--out", outputDir, page, about, layout, component}
	if err := build(args); err != nil {
		t.Fatal(err)
	}
	aboutPath := filepath.Join(outputDir, "about", "index.html")
	aboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	writeCLIFile(t, component, `package app

component Brand

view {
  <strong>After</strong>
}
`)
	used, err := buildIncrementalSPA(args, inputChange{Changed: []string{component}})
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected incremental spa build to handle layout-only component dependency change")
	}
	homePayload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homePayload), "<strong>After</strong>") {
		t.Fatalf("expected changed layout component output:\n%s", homePayload)
	}
	afterAboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterAboutInfo.ModTime().Equal(aboutInfo.ModTime()) {
		t.Fatalf("expected unchanged about output mod time: before=%s after=%s", aboutInfo.ModTime(), afterAboutInfo.ModTime())
	}
}

func TestBuildIncrementalSPAUsesLayoutDependencies(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	layout := filepath.Join(root, "root.layout.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"
layout root

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>Stable</main>
}
`)
	writeCLIFile(t, layout, `package app

view {
  <section class="before"><slot /></section>
}
`)

	args := []string{"--config", config, "--out", outputDir, page, about, layout}
	if err := build(args); err != nil {
		t.Fatal(err)
	}
	aboutPath := filepath.Join(outputDir, "about", "index.html")
	aboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	writeCLIFile(t, layout, `package app

view {
  <section class="after"><slot /></section>
}
`)
	used, err := buildIncrementalSPA(args, inputChange{Changed: []string{layout}})
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected incremental spa build to handle layout dependency change")
	}
	homePayload, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homePayload), `class="after"`) {
		t.Fatalf("expected changed layout output:\n%s", homePayload)
	}
	afterAboutInfo, err := os.Stat(aboutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterAboutInfo.ModTime().Equal(aboutInfo.ModTime()) {
		t.Fatalf("expected unchanged about output mod time: before=%s after=%s", aboutInfo.ModTime(), afterAboutInfo.ModTime())
	}
}

func TestBuildIncrementalSPAFallsBackForConfigChanges(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	used, err := buildIncrementalSPA([]string{"--config", config, "--out", outputDir, page}, inputChange{Changed: []string{config}})
	if err != nil {
		t.Fatal(err)
	}
	if used {
		t.Fatal("expected config change to fall back to full build")
	}
}

func TestBuildCommandWritesComponentExpandedHTML(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>
    <Hero title="GOWDK" />
  </main>
}
`)
	writeCLIFile(t, component, `package app

component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, page, component}); err != nil {
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
	writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>
    <Hero title="Discovered" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `package app

component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`)
	writeCLIFile(t, filepath.Join(root, "dist", "stale.page.gwdk"), `package app

page stale
route "/stale"

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
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>
    <Hero title="Configured" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `package app

component Hero

props {
  title string
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "ignored.page.gwdk"), `package app

page ignored
route "/ignored"

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

func TestBuildCommandObfuscateAssetsFlag(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	outputDir := filepath.Join(root, "dist")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main><a href="/docs">Docs</a></main>
}
`)
	writeCLIFile(t, filepath.Join(root, "docs.page.gwdk"), `package app

page docs
route "/docs"

view {
  <main>Docs</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--config", config, "--obfuscate-assets", "--out", outputDir, "home.page.gwdk", "docs.page.gwdk"}); err != nil {
			t.Fatal(err)
		}
	})

	payload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(payload, &assets); err != nil {
		t.Fatal(err)
	}
	if assets.Version != runtimeasset.ManifestVersion {
		t.Fatalf("expected asset manifest version %d, got %d", runtimeasset.ManifestVersion, assets.Version)
	}
	if !assets.IsObfuscated("assets/gowdk/gowdk.js") {
		t.Fatalf("expected obfuscation metadata in manifest: %s", payload)
	}
	reportPayload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reportPayload), `"kind": "asset_obfuscated"`) {
		t.Fatalf("expected asset obfuscation event in report:\n%s", reportPayload)
	}
}

func TestBuildCommandBuildsActionExampleWithImportedComponents(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	config := filepath.Join(repoRoot, "gowdk.config.go")
	files := []string{
		filepath.Join(repoRoot, "examples", "actions", "newsletter.page.gwdk"),
		filepath.Join(repoRoot, "examples", "components", "base", "button.cmp.gwdk"),
		filepath.Join(repoRoot, "examples", "components", "base", "text-field.cmp.gwdk"),
	}
	args := append([]string{"build", "--config", config, "--ssr", "--out", outputDir}, files...)
	if err := run(args); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(outputDir, "newsletter", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(payload)
	for _, expected := range []string{
		`<label class="gowdk-field"><span>Email</span><input name="email" type="email"></label>`,
		`<button class="gowdk-button" type="submit">Subscribe</button>`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in newsletter output:\n%s", expected, output)
		}
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
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

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
	manifestPayload, err := os.ReadFile(filepath.Join(root, "dist", "gowdk-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(manifestPayload, &assets); err != nil {
		t.Fatal(err)
	}
	emittedCSS := assets.Resolve("assets/tw.css")
	if emittedCSS == "" || emittedCSS == "assets/tw.css" {
		t.Fatalf("expected hashed tailwind css manifest entry, got %q in %s", emittedCSS, manifestPayload)
	}
	if !strings.Contains(string(html), `<link rel="stylesheet" href="/`+emittedCSS+`">`) {
		t.Fatalf("expected tailwind stylesheet link:\n%s", html)
	}
	css, err := os.ReadFile(filepath.Join(root, "dist", filepath.FromSlash(emittedCSS)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), ".font-bold{font-weight:700;}") {
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "ui2", "second.page.gwdk"), `package app

page second
route "/second"

view {
  <main>Frontend Two</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

page admin
route "/admin"

view {
  <main>Backend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "ignored.page.gwdk"), `package app

page ignored
route "/ignored"

view {
  <main>Ignored</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "other", "stray.page.gwdk"), `package app

page stray
route "/stray"

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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

page admin
route "/admin"

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

func TestBuildCommandRunsConfiguredBuildTargets(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "public"},
		{Name: "admin"},
		{Name: "api"},
	},
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{
				Name: "public",
				Modules: []string{"public"},
				Output: "dist/public",
				App: ".gowdk/public",
				Binary: "bin/public",
				DeployRecipes: []string{"static"},
			},
			{
				Name: "admin-api",
				Modules: []string{"admin", "api"},
				Output: "dist/admin-api",
				App: ".gowdk/admin-api",
			},
		},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "public", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Public module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

page dashboard
route "/admin"

view {
  <main>Admin module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "api", "status.page.gwdk"), `package app

page status
route "/api/status"

view {
  <main>API module</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build"}); err != nil {
			t.Fatal(err)
		}
	})

	for _, path := range []string{
		filepath.Join(root, "dist", "public", "index.html"),
		filepath.Join(root, "dist", "public", "deploy", "static-host.md"),
		filepath.Join(root, ".gowdk", "public", "gowdkapp", "app", "index.html"),
		filepath.Join(root, "bin", "public"),
		filepath.Join(root, "dist", "admin-api", "admin", "index.html"),
		filepath.Join(root, "dist", "admin-api", "api", "status", "index.html"),
		filepath.Join(root, ".gowdk", "admin-api", "gowdkapp", "app", "admin", "index.html"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "dist", "public", "admin", "index.html"),
		filepath.Join(root, ".gowdk", "public", "gowdkapp", "app", "admin", "index.html"),
		filepath.Join(root, "dist", "admin-api", "index.html"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected unselected target route %s to be absent, stat err: %v", path, err)
		}
	}
}

func TestBuildCommandRunsSelectedConfiguredTargetOnly(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "public"},
		{Name: "admin"},
	},
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "public", Modules: []string{"public"}, Output: "dist/public"},
			{Name: "admin", Modules: []string{"admin"}, Output: "dist/admin"},
		},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "public", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Public module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

page dashboard
route "/admin"

view {
  <main>Admin module</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--target", "admin"}); err != nil {
			t.Fatal(err)
		}
	})

	if _, err := os.Stat(filepath.Join(root, "dist", "admin", "admin", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "public", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected unselected public target to be skipped, stat err: %v", err)
	}
}

func TestBuildCommandInfersConfiguredTargetOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "site", App: ".gowdk/site", Binary: "bin/site"},
		},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Inferred output</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build"}); err != nil {
			t.Fatal(err)
		}
	})

	for _, path := range []string{
		filepath.Join(root, ".gowdk", "output", "site", "index.html"),
		filepath.Join(root, ".gowdk", "site", "gowdkapp", "app", "index.html"),
		filepath.Join(root, "bin", "site"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected inferred target artifact %s: %v", path, err)
		}
	}
}

func TestBuildCommandRunsConfiguredWASMTarget(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Modules: []gowdk.ModuleConfig{
		{Name: "public"},
	},
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "public", Modules: []string{"public"}, Output: "dist/public", App: ".gowdk/public", WASM: "bin/public.wasm"},
		},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "public", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Public WASM module</main>
}
`)

	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--target", "public"}); err != nil {
			t.Fatal(err)
		}
	})

	info, err := os.Stat(filepath.Join(root, "bin", "public.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected configured wasm artifact to be non-empty")
	}
}

func TestBuildCommandRejectsTargetWithAdHocBuildFlags(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{Name: "public", Output: "dist/public"},
		},
	},
}
`)

	withWorkingDir(t, root, func() {
		err := run([]string{"build", "--target", "public", "--out", "dist/override"})
		if err == nil {
			t.Fatal("expected target and ad hoc output to fail")
		}
		if !strings.Contains(err.Error(), "--target cannot be combined") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
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
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `package app

page home
route "/"

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
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

page home
route "/"

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

func TestContractsCommandReportsGoRegistrations(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"

	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatient struct{}
type PatientPage struct{}
type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterQuery[GetPatient, PatientPage](r, LoadPatient, contracts.RoleWeb)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail, contracts.RoleWorker)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"contracts", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	for _, expected := range []string{
		"COMMAND CreatePatient",
		"handler: HandleCreatePatient",
		"result: CreatePatientResult",
		"DOMAIN EVENT PatientCreated",
		"QUERY GetPatient",
		"source: patients.go:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected contracts output to contain %q:\n%s", expected, output)
		}
	}
}

func TestGraphCommandReportsCommandEventEdges(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"

	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail, contracts.RoleWorker)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"graph", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	for _, expected := range []string{
		"COMMAND CreatePatient",
		"emits:",
		"- DOMAIN EVENT PatientCreated",
		"DOMAIN EVENT PatientCreated",
		"subscribers:",
		"- SendWelcomeEmail",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected graph output to contain %q:\n%s", expected, output)
		}
	}
}

func TestTraceCommandReportsHandlersEmitsAndSubscribers(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"

	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail, contracts.RoleWorker)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"trace", "patients.CreatePatient", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	for _, expected := range []string{
		"COMMAND CreatePatient (patients.CreatePatient)",
		"handler: HandleCreatePatient",
		"roles: web",
		"emits:",
		"- DOMAIN EVENT PatientCreated",
		"subscribers:",
		"- SendWelcomeEmail",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected trace output to contain %q:\n%s", expected, output)
		}
	}
}

func TestTraceCommandJSONReportsEmitsAndSubscribers(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"

	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail, contracts.RoleWorker)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	output, _, err := captureCLIOutput(t, func() error {
		return run([]string{"trace", "CreatePatient", "--json", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Target  string `json:"target"`
		Matches []struct {
			Contract struct {
				Kind string `json:"kind"`
				Type string `json:"type"`
			} `json:"contract"`
			Emits []struct {
				Type        string `json:"type"`
				Subscribers []struct {
					Handler string `json:"handler"`
				} `json:"subscribers"`
			} `json:"emits"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid trace JSON: %v\n%s", err, output)
	}
	if report.Target != "CreatePatient" || len(report.Matches) != 1 || report.Matches[0].Contract.Kind != "command" {
		t.Fatalf("unexpected trace JSON: %s", output)
	}
	if len(report.Matches[0].Emits) != 1 || report.Matches[0].Emits[0].Type != "PatientCreated" {
		t.Fatalf("unexpected trace emits JSON: %s", output)
	}
	if len(report.Matches[0].Emits[0].Subscribers) != 1 || report.Matches[0].Emits[0].Subscribers[0].Handler != "SendWelcomeEmail" {
		t.Fatalf("unexpected trace subscriber JSON: %s", output)
	}
}

func TestListCommandsJSONFiltersContracts(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "patients.go"), `package patients

import contracts "github.com/cssbruno/gowdk/runtime/contracts"

type GetPatient struct{}
type PatientPage struct{}
type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterQuery[GetPatient, PatientPage](r, LoadPatient, contracts.RoleWeb)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
}
`)

	output, _, err := captureCLIOutput(t, func() error {
		return run([]string{"list", "commands", "--json", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Contracts []struct {
			Kind string `json:"kind"`
			Type string `json:"type"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid contracts JSON: %v\n%s", err, output)
	}
	if len(report.Contracts) != 1 || report.Contracts[0].Kind != "command" || report.Contracts[0].Type != "CreatePatient" {
		t.Fatalf("unexpected command report: %s", output)
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
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page bad
route "/bad"
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"check", "--config", config, "--json", source})
	})
	if err == nil {
		t.Fatal("expected check to fail")
	}
	if !strings.Contains(output, `"diagnostics"`) || !strings.Contains(output, "missing view") {
		t.Fatalf("expected JSON diagnostics, got:\n%s", output)
	}
}

func TestCheckCommandPromotesWarnings(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <img src="/hero.png" />
}
`)

	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"check", "--config", config, source})
	})
	if err != nil {
		t.Fatalf("check should allow warnings by default: %v", err)
	}
	_, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"check", "--warnings-as-errors", "--config", config, source})
	})
	if err == nil || !strings.Contains(stderr, "warning:") || !strings.Contains(stderr, "alt") {
		t.Fatalf("expected warnings-as-errors failure with accessibility warning, stderr=%q err=%v", stderr, err)
	}
}

func TestFixCommandMigratesOldActionSyntax(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "signup.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

page signup
route "/signup"
guard public

act submit {
}

view {
  <main>Signup</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"fix", "--config", config, source})
	})
	if err != nil {
		t.Fatalf("fix failed: %v", err)
	}
	if !strings.Contains(output, "applied 1 fix") {
		t.Fatalf("expected fix output, got %q", output)
	}
	fixed, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fixed), "act submit {") || !strings.Contains(string(fixed), `act Submit POST "/signup"`) {
		t.Fatalf("old action syntax was not migrated:\n%s", fixed)
	}
	if err := run([]string{"check", "--config", config, source}); err != nil {
		t.Fatalf("fixed source should validate: %v\n%s", err, fixed)
	}
}

func TestManifestCommandHandlesMultipleFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, home, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, about, `package app

page about
route "/about"

view {
  <main>About</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"manifest", "--config", config, home, about})
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
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Configured check</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `package app

page ignored
route "/"

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
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Manifest discovery</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `package app

page ignored
route "/ignored"

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

func TestManifestCommandDefaultDiscoverySkipsTestdata(t *testing.T) {
	root := t.TempDir()
	writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "internal", "lang", "testdata", "home.page.gwdk"), `package app

page home
route "/fixture"

view {
  <main>Fixture duplicate</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"manifest"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	if !strings.Contains(output, `"route": "/"`) {
		t.Fatalf("expected discovered home route in manifest: %s", output)
	}
	if strings.Contains(output, "fixture") || strings.Contains(output, "Fixture duplicate") {
		t.Fatalf("expected testdata page to be skipped: %s", output)
	}
}

func TestManifestCommandConfigSSRAddonEnablesSSR(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"pages/**/*.gwdk"},
	},
	Addons: []gowdk.Addon{
		ssr.Addon(),
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "dashboard.page.gwdk"), `package app

page dashboard
route "/dashboard"

go ssr {
}

view {
  <main>Dashboard</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"manifest"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	if !strings.Contains(output, `"render": "ssr"`) {
		t.Fatalf("expected SSR page in manifest: %s", output)
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

page admin
route "/admin"

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

func TestRoutesCommandPrintsRoutesAndEndpoints(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"routes", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "info: ssr_disabled: newsletter uses build-time page output; request-time page rendering is disabled for this route") {
		t.Fatalf("expected disabled SSR route info on stderr, got:\n%s", stderr)
	}

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("unexpected routes version: %d", report.Version)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("expected only page routes, got %#v", report.Routes)
	}
	if len(report.Endpoints) != 1 {
		t.Fatalf("expected one action endpoint, got %#v", report.Endpoints)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "spa",
		Method:  "GET",
		Route:   "/newsletter",
		PageID:  "newsletter",
		Handler: `embedded.SPA("pages/newsletter.html")`,
	})
	assertEndpointBinding(t, report.Endpoints, endpointBindingJSON{
		Kind:           "action",
		EndpointSource: "gwdk",
		Source:         page,
		Package:        "app",
		PackageName:    "app",
		Symbol:         "Subscribe",
		Method:         "POST",
		Route:          "/newsletter",
		Cache:          "no-store",
		Guards:         []string{"public"},
		CSRF:           true,
		PageID:         "newsletter",
		Handler:        "actions.NewsletterSubscribe",
		BindingStatus:  "missing",
		BackendBinding: &backendBindingJSON{
			Status:       "missing",
			PackageName:  "app",
			FunctionName: "Subscribe",
			Message:      "GOWDK action handler app.Subscribe is not implemented",
		},
	})
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 7 {
		t.Fatalf("expected endpoint source span for action declaration, got %#v", report.Endpoints[0].SourceSpan)
	}
	assertRouteInfo(t, report.Info, "ssr_disabled", "newsletter")
}

func TestEndpointsCommandPrintsEndpointMetadataOnly(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
}
`)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"
guard public

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"endpoints", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected endpoint report to keep route info off stderr, got:\n%s", stderr)
	}
	var report endpointMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid endpoints JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("unexpected endpoints version: %d", report.Version)
	}
	if len(report.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %#v", report.Endpoints)
	}
	assertEndpointBinding(t, report.Endpoints, endpointBindingJSON{
		Kind:           "action",
		EndpointSource: "gwdk",
		Source:         page,
		Package:        "app",
		PackageName:    "app",
		Symbol:         "Subscribe",
		Method:         "POST",
		Route:          "/newsletter",
		Cache:          "no-store",
		Guards:         []string{"public"},
		CSRF:           true,
		PageID:         "newsletter",
		Handler:        "actions.NewsletterSubscribe",
		BindingStatus:  "missing",
		BackendBinding: &backendBindingJSON{
			Status:       "missing",
			PackageName:  "app",
			FunctionName: "Subscribe",
			Message:      "GOWDK action handler app.Subscribe is not implemented",
		},
	})
}

func TestRoutesCommandPrintsSSRRouteKind(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "dashboard.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page dashboard
route "/dashboard"

load {
}

view {
  <main>Dashboard</main>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"routes", "--config", config, "--ssr", page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "spa_disabled: dashboard uses request-time page behavior") {
		t.Fatalf("expected spa disabled info for ssr route, got:\n%s", stderr)
	}

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "ssr",
		Method:  "GET",
		Route:   "/dashboard",
		PageID:  "dashboard",
		Handler: "ssr.RenderDashboard",
	})
}

func TestRoutesCommandPrintsBareHybridAsSSRRoute(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "dashboard.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page dashboard
route "/dashboard"

go ssr {
}

view {
  <main>Dashboard</main>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"routes", "--ssr", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "spa_disabled: dashboard uses request-time page behavior") {
		t.Fatalf("expected spa disabled info for hybrid route, got:\n%s", stderr)
	}

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "ssr",
		Method:  "GET",
		Route:   "/dashboard",
		PageID:  "dashboard",
		Handler: "ssr.RenderDashboard",
	})
}

func TestRoutesCommandPrintsDefaultHybridRouteKind(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "dashboard.page.gwdk")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Render: gowdk.RenderConfig{Default: gowdk.Hybrid},
}
`)
	writeCLIFile(t, page, `package app

page dashboard
route "/dashboard"

view {
  <main>Dashboard</main>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"routes", "--ssr", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr, "spa_disabled") || strings.Contains(stderr, "ssr_disabled") {
		t.Fatalf("expected no disabled route info for default hybrid route, got:\n%s", stderr)
	}

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "hybrid",
		Method:  "GET",
		Route:   "/dashboard",
		PageID:  "dashboard",
		Handler: "hybrid.RenderDashboard",
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

page admin
route "/admin"

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

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("expected selected backend route only, got %#v", report.Routes)
	}
	assertRouteBinding(t, report.Routes, routeBindingJSON{
		Kind:    "spa",
		Method:  "GET",
		Route:   "/admin",
		PageID:  "admin",
		Handler: `embedded.SPA("pages/admin.html")`,
	})
}

func TestRoutesCommandPrintsAPIBinding(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "status.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page status
route "/status"

api Health GET "/api/health"

view {
  <main>Status</main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"routes", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("expected status page route only, got %#v", report.Routes)
	}
	if len(report.Endpoints) != 1 {
		t.Fatalf("expected one API endpoint, got %#v", report.Endpoints)
	}
	assertEndpointBinding(t, report.Endpoints, endpointBindingJSON{
		Kind:           "api",
		EndpointSource: "gwdk",
		Source:         page,
		Package:        "app",
		PackageName:    "app",
		Symbol:         "Health",
		Method:         "GET",
		Route:          "/api/health",
		Cache:          "no-store",
		Guards:         []string{"public"},
		PageID:         "status",
		Handler:        "api.StatusHealth",
		BindingStatus:  "missing",
		BackendBinding: &backendBindingJSON{
			Status:       "missing",
			PackageName:  "app",
			FunctionName: "Health",
			Message:      "GOWDK API handler app.Health is not implemented",
		},
	})
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 7 {
		t.Fatalf("expected endpoint source span for API declaration, got %#v", report.Endpoints[0].SourceSpan)
	}
}

func TestInspectIRCommandPrintsCompilerIR(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"inspect", "ir", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
	}

	var report struct {
		Packages []struct {
			Name  string `json:"Name"`
			Files []struct {
				Path string `json:"Path"`
				Kind string `json:"Kind"`
				Name string `json:"Name"`
			} `json:"Files"`
		} `json:"Packages"`
		Pages []struct {
			ID     string   `json:"ID"`
			Route  string   `json:"Route"`
			Guards []string `json:"Guards"`
		} `json:"Pages"`
		Routes []struct {
			Kind   string `json:"Kind"`
			Method string `json:"Method"`
			Path   string `json:"Path"`
			PageID string `json:"PageID"`
		} `json:"Routes"`
		Endpoints []struct {
			Kind    string `json:"Kind"`
			Symbol  string `json:"Symbol"`
			Method  string `json:"Method"`
			Path    string `json:"Path"`
			PageID  string `json:"PageID"`
			Binding struct {
				Status       string `json:"Status"`
				FunctionName string `json:"FunctionName"`
			} `json:"Binding"`
		} `json:"Endpoints"`
		Templates []struct {
			OwnerKind string `json:"OwnerKind"`
			OwnerID   string `json:"OwnerID"`
			Body      string `json:"Body"`
		} `json:"Templates"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid inspect ir JSON: %v\n%s", err, output)
	}
	if len(report.Pages) != 1 || report.Pages[0].ID != "newsletter" || report.Pages[0].Route != "/newsletter" {
		t.Fatalf("unexpected page IR: %#v", report.Pages)
	}
	if len(report.Pages[0].Guards) != 1 || report.Pages[0].Guards[0] != "public" {
		t.Fatalf("expected public guard in page IR: %#v", report.Pages[0].Guards)
	}
	if len(report.Routes) != 1 || report.Routes[0].Kind != "spa" || report.Routes[0].Path != "/newsletter" {
		t.Fatalf("unexpected route IR: %#v", report.Routes)
	}
	if len(report.Endpoints) != 1 || report.Endpoints[0].Kind != "action" || report.Endpoints[0].Symbol != "Subscribe" {
		t.Fatalf("unexpected endpoint IR: %#v", report.Endpoints)
	}
	if report.Endpoints[0].Binding.Status != "missing" || report.Endpoints[0].Binding.FunctionName != "Subscribe" {
		t.Fatalf("expected backend binding metadata in endpoint IR: %#v", report.Endpoints[0].Binding)
	}
	if len(report.Templates) != 1 || report.Templates[0].OwnerID != "newsletter" || !strings.Contains(report.Templates[0].Body, "g:post") {
		t.Fatalf("unexpected template IR: %#v", report.Templates)
	}
	if len(report.Packages) != 1 || report.Packages[0].Name != "app" || len(report.Packages[0].Files) != 1 || report.Packages[0].Files[0].Path != page {
		t.Fatalf("unexpected package IR: %#v", report.Packages)
	}
}

func TestInspectTreeCommandPrintsSourceLinkedViewTree(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <main class="shell">
    <form g:post={Subscribe}>
      <input name="email" required />
      <button type="submit">Subscribe</button>
    </form>
  </main>
}
`)

	output, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"inspect", "tree", "--json", "--config", config, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
	}

	var report inspectTreeGolden
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid inspect tree JSON: %v\n%s", err, output)
	}
	if report.Version != 1 || report.Root.Kind != "program" {
		t.Fatalf("unexpected tree root: %#v", report)
	}
	pageNode, ok := findInspectNode(report.Root, "page", "newsletter")
	if !ok || pageNode.Source != page {
		t.Fatalf("expected source-linked page node, got %#v", pageNode)
	}
	formNode, ok := findInspectNode(report.Root, "element", "form")
	if !ok {
		t.Fatalf("expected form element node in tree:\n%s", output)
	}
	if formNode.Span == nil || formNode.Span.Start.Line != 11 {
		t.Fatalf("expected form source span on line 11, got %#v", formNode.Span)
	}
	if !inspectPropListContains(formNode.Props, "directives", "g:post") {
		t.Fatalf("expected form g:post directive in props, got %#v", formNode.Props)
	}
	if _, ok := findInspectNode(report.Root, "text", "Subscribe"); !ok {
		t.Fatalf("expected button text node in tree:\n%s", output)
	}
}

func TestInspectEndpointGraphCommandPrintsEndpointEdges(t *testing.T) {
	root := t.TempDir()
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/gowdk-endpoint-graph

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, filepath.ToSlash(moduleRoot)))
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
}
`)
	writeCLIFile(t, page, `package pages

page patients
route "/patients"

act Save POST "/patients/save"

view {
  <main>
    <form g:post={Save}>
      <button type="submit">Save</button>
    </form>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string `+"`json:\"name\" form:\"name\"`"+`
}
type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{ID: command.Name}, nil
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "handlers.go"), `package pages

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/response"
)

//gowdk:api GET /api/patients
func ListPatients(context.Context, *http.Request) (response.Response, error) {
	return response.JSONValue(http.StatusOK, []string{"Ada"})
}
`)

	var output string
	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"inspect", "endpoint-graph", "--json", "--config", config, page})
		})
		if err != nil {
			t.Fatalf("%v\nstderr:\n%s", err, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
		}
		output = stdout
	})

	var report endpointGraphGolden
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid endpoint graph JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("unexpected endpoint graph version: %d", report.Version)
	}
	pageGraphNode, ok := findGraphNode(report.Nodes, "page", "patients")
	if !ok {
		t.Fatalf("expected patients page node:\n%s", output)
	}
	actionNode, ok := findGraphNode(report.Nodes, "endpoint", "Save")
	if !ok || !graphPropBool(actionNode.Props, "csrf") || !graphPropListContains(actionNode.Props, "guards", "public") {
		t.Fatalf("expected CSRF/guarded action endpoint node, got %#v", actionNode)
	}
	contractNode, ok := findGraphNode(report.Nodes, "contract", "patients.CreatePatient")
	if !ok {
		t.Fatalf("expected contract node:\n%s", output)
	}
	if _, ok := findGraphNode(report.Nodes, "handler", "contracts.command.patients.CreatePatient"); !ok {
		t.Fatalf("expected generated contract handler node:\n%s", output)
	}
	standaloneNode, ok := findGraphNode(report.Nodes, "endpoint", "ListPatients")
	if !ok || standaloneNode.Source != filepath.Join(root, "pages", "handlers.go") || standaloneNode.Props["endpointSource"] != "go" {
		t.Fatalf("expected standalone Go API endpoint node, got %#v", standaloneNode)
	}
	if !hasGraphEdge(report.Edges, pageGraphNode.ID, actionNode.ID, "owns_endpoint") {
		t.Fatalf("expected page -> action edge in %#v", report.Edges)
	}
	if !hasGraphEdgeToKind(report.Edges, report.Nodes, actionNode.ID, "handler", "handled_by") {
		t.Fatalf("expected action -> handler edge in %#v", report.Edges)
	}
	if !hasGraphEdgeTo(report.Edges, contractNode.ID, "references_contract") {
		t.Fatalf("expected endpoint -> contract edge in %#v", report.Edges)
	}
	if !hasGraphEdgeToKind(report.Edges, report.Nodes, actionNode.ID, "guard", "uses_guard") {
		t.Fatalf("expected action -> guard edge in %#v", report.Edges)
	}
	if !hasGraphEdgeToKind(report.Edges, report.Nodes, standaloneNode.ID, "handler", "handled_by") {
		t.Fatalf("expected standalone endpoint -> handler edge in %#v", report.Edges)
	}
}

func TestInspectGoBindingsCommandPrintsBindingReport(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-go-bindings")
	page := filepath.Join(root, "pages", "dashboard.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page dashboard
route "/dashboard"

import interop "github.com/cssbruno/gowdk/examples/go-interop"

build {
  => interop.FeaturedCopyWithErrorForBuild()
}

load {
  => { user.name }
}

act Save POST "/dashboard/save"
api Session GET "/api/session"
fragment Summary GET "/dashboard/summary" "#summary" {
  <section>Summary</section>
}

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
    <section id="summary" g:query="patients.GetPatientPage">
      {title}
    </section>
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string `+"`form:\"name\"`"+`
}
type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}
type GetPatientPage struct{}
type PatientPageData struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterQuery[GetPatientPage, PatientPageData](r, LoadPatientPage, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{ID: command.Name}, nil
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{}, nil
}
`)

	var output string
	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"inspect", "go-bindings", "--json", "--ssr", "--config", config, page})
		})
		if err != nil {
			t.Fatalf("%v\nstderr:\n%s", err, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
		}
		output = stdout
	})

	var report goBindingsReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid go-bindings JSON: %v\n%s", err, output)
	}
	if report.Version != 1 {
		t.Fatalf("unexpected go-bindings version: %d", report.Version)
	}
	assertGoBinding(t, report.Bindings, "build", "FeaturedCopyWithErrorForBuild", "unverified")
	assertGoBinding(t, report.Bindings, "load", "LoadDashboard", "missing")
	assertGoBinding(t, report.Bindings, "action", "Save", "missing")
	assertGoBinding(t, report.Bindings, "api", "Session", "missing")
	assertGoBinding(t, report.Bindings, "fragment", "Summary", "unknown")
	assertGoBinding(t, report.Bindings, "command", "patients.CreatePatient", "bound")
	assertGoBinding(t, report.Bindings, "query", "patients.GetPatientPage", "bound")
	if binding, ok := findGoBinding(report.Bindings, "build", "FeaturedCopyWithErrorForBuild"); !ok || binding.PackagePath != "github.com/cssbruno/gowdk/examples/go-interop" {
		t.Fatalf("expected imported build package path, got %#v", binding)
	}
}

func TestGenerateStubsWritesMissingActionAndAPIHandlers(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-stubs")
	page := filepath.Join(root, "pages", "signup.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page signup
route "/signup"

act Submit POST "/signup"
api Session GET "/api/session"

view {
  <main>Signup</main>
}
`)

	var generatedPath string
	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"generate", "stubs", "--config", config, page})
		})
		if err != nil {
			t.Fatalf("%v\nstderr:\n%s", err, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected generate stubs to keep stderr empty, got:\n%s", stderr)
		}
		generatedPath = strings.TrimSpace(stdout)
	})
	if generatedPath == "" {
		t.Fatal("expected generated stub path on stdout")
	}
	payload, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	for _, expected := range []string{
		"package pages",
		"func Submit(gowdkcontext.Context) (gowdkresponse.Response, error)",
		"func Session(gowdkcontext.Context, *gowdkhttp.Request) (gowdkresponse.Response, error)",
		"GOWDK generated stub: implement Submit",
		"GOWDK generated stub: implement Session",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated stubs to contain %q:\n%s", expected, source)
		}
	}

	var output string
	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"inspect", "go-bindings", "--config", config, page})
		})
		if err != nil {
			t.Fatalf("%v\nstderr:\n%s", err, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
		}
		output = stdout
	})
	var report goBindingsReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid go-bindings JSON: %v\n%s", err, output)
	}
	assertGoBinding(t, report.Bindings, "action", "Submit", "bound")
	assertGoBinding(t, report.Bindings, "api", "Session", "bound")
}

func TestGenerateStubsRejectsJSONFlag(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-stubs-json")
	page := filepath.Join(root, "pages", "signup.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page signup
route "/signup"

act Submit POST "/signup"

view {
  <main>Signup</main>
}
`)

	withWorkingDir(t, root, func() {
		_, _, err := captureCLIOutput(t, func() error {
			return run([]string{"generate", "stubs", "--json", "--config", config, page})
		})
		if err == nil || !strings.Contains(err.Error(), `unknown generate stubs flag "--json"`) {
			t.Fatalf("expected generate stubs to reject --json, got %v", err)
		}
	})
}

func TestGenerateStubsRefusesToOverwriteExistingStubFile(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-stubs-overwrite")
	page := filepath.Join(root, "pages", "signup.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page signup
route "/signup"

act Submit POST "/signup"

view {
  <main>Signup</main>
}
`)
	existing := filepath.Join(root, "pages", "gowdk_stubs.go")
	writeCLIFile(t, existing, "package pages\n")

	withWorkingDir(t, root, func() {
		_, _, err := captureCLIOutput(t, func() error {
			return run([]string{"generate", "stubs", "--config", config, page})
		})
		if err == nil || !strings.Contains(err.Error(), "already exists; refusing to overwrite handler stubs") {
			t.Fatalf("expected generate stubs to refuse overwrite, got %v", err)
		}
	})
}

func TestCheckWarnsOnUnsupportedBackendSignature(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-unsupported-binding")
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "signup.page.gwdk")
	writeCLIFile(t, page, `package pages

page signup
route "/signup"

act Submit POST "/signup"

view {
  <main>Signup</main>
}
`)
	// Submit exists but with a signature GOWDK cannot bind as an action.
	writeCLIFile(t, filepath.Join(root, "pages", "handlers.go"), `package pages

func Submit() string { return "" }
`)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err != nil {
		t.Fatalf("expected an unsupported signature to be a non-fatal warning, got error: %v\n%s", err, output)
	}
	diagnostic := requireCheckDiagnostic(t, output, "unsupported_backend_signature")
	if diagnostic.Severity != "warning" {
		t.Fatalf("expected warning severity, got %q", diagnostic.Severity)
	}
	if !strings.Contains(diagnostic.Message, "Submit") {
		t.Fatalf("expected message to name the handler, got %q", diagnostic.Message)
	}
}

func TestCheckWarnsOnUnexportedBackendHandler(t *testing.T) {
	root := t.TempDir()
	writeCLITestModule(t, root, "example.com/gowdk-unexported-binding")
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "signup.page.gwdk")
	writeCLIFile(t, page, `package pages

page signup
route "/signup"

act Submit POST "/signup"

view {
  <main>Signup</main>
}
`)
	// The same-named function exists but is unexported, so binding cannot see it.
	writeCLIFile(t, filepath.Join(root, "pages", "handlers.go"), `package pages

func submit() {}
`)

	var output string
	var err error
	withWorkingDir(t, root, func() {
		output, err = captureCLIStdout(t, func() error {
			return run([]string{"check", "--config", config, "--json", page})
		})
	})
	if err != nil {
		t.Fatalf("expected an unexported near-miss to be a non-fatal warning, got error: %v\n%s", err, output)
	}
	diagnostic := requireCheckDiagnostic(t, output, "unexported_backend_handler")
	if diagnostic.Severity != "warning" {
		t.Fatalf("expected warning severity, got %q", diagnostic.Severity)
	}
	if !strings.Contains(diagnostic.Message, "export it as Submit") {
		t.Fatalf("expected message to suggest exporting the function, got %q", diagnostic.Message)
	}
}

type checkDiagnosticJSON struct {
	File     string `json:"file"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func requireCheckDiagnostic(t *testing.T, output, code string) checkDiagnosticJSON {
	t.Helper()
	var report struct {
		Diagnostics []checkDiagnosticJSON `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid check JSON: %v\n%s", err, output)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	t.Fatalf("expected a %q diagnostic, got %#v", code, report.Diagnostics)
	return checkDiagnosticJSON{}
}

func TestInspectIRCommandMatchesGoldenFixture(t *testing.T) {
	fixture := filepath.FromSlash("testdata/inspect_ir_golden")
	var output string
	withWorkingDir(t, fixture, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"inspect", "ir", "--config", "gowdk.config.go", "newsletter.page.gwdk"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if stderr != "" {
			t.Fatalf("expected no inspect diagnostics on stderr, got:\n%s", stderr)
		}
		output = stdout
	})

	expected, err := os.ReadFile(filepath.Join(fixture, "inspect-ir.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON := canonicalInspectIRGolden(t, expected)
	actualJSON := canonicalInspectIRGolden(t, []byte(output))
	if actualJSON != expectedJSON {
		t.Fatalf("inspect ir golden mismatch\nexpected:\n%s\nactual:\n%s", expectedJSON, actualJSON)
	}
}

func TestRoutesCommandMatchesGoldenFixture(t *testing.T) {
	fixture := filepath.FromSlash("testdata/routes_golden")
	var output string
	withWorkingDir(t, fixture, func() {
		stdout, _, err := captureCLIOutput(t, func() error {
			return run([]string{"routes", "--config", "gowdk.config.go", "newsletter.page.gwdk"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = stdout
	})

	expected, err := os.ReadFile(filepath.Join(fixture, "routes.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON := canonicalRoutesGolden(t, expected)
	actualJSON := canonicalRoutesGolden(t, []byte(output))
	if actualJSON != expectedJSON {
		t.Fatalf("routes golden mismatch\nexpected:\n%s\nactual:\n%s", expectedJSON, actualJSON)
	}
}

func canonicalRoutesGolden(t *testing.T, payload []byte) string {
	t.Helper()
	var report routeMetadataReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("invalid routes golden JSON: %v\n%s", err, payload)
	}
	canonical, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(canonical)
}

func TestInspectIRCommandDiscoversSelectedModuleOnly(t *testing.T) {
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

page admin
route "/admin"

view {
  <main>Backend</main>
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"inspect", "ir", "--module", "backend"})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	var report struct {
		Pages []struct {
			ID string `json:"ID"`
		} `json:"Pages"`
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid inspect ir JSON: %v\n%s", err, output)
	}
	if len(report.Pages) != 1 || report.Pages[0].ID != "admin" {
		t.Fatalf("expected selected backend page only, got %#v", report.Pages)
	}
}

func TestInspectCommandRejectsUnknownTarget(t *testing.T) {
	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"inspect", "assets"})
	})
	if err == nil {
		t.Fatal("expected unknown inspect target error")
	}
	if !strings.Contains(err.Error(), `unknown inspect target "assets"`) {
		t.Fatalf("unexpected inspect error: %v", err)
	}
}

func TestRoutesCommandPrintsContractEndpoints(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "patients.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page patients
route "/patients"

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string `+"`json:\"name\" form:\"name\"`"+`
}
type CreatePatientResult struct {
	ID string `+"`json:\"id\"`"+`
}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{ID: command.Name}, nil
}
`)

	var output string
	withWorkingDir(t, root, func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"routes", "--config", config, page})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertEndpointBinding(t, report.Endpoints, endpointBindingJSON{
		Kind:           "command",
		EndpointSource: "contract",
		Source:         page,
		Package:        "pages",
		PackagePath:    "",
		Symbol:         "patients.CreatePatient",
		Method:         "POST",
		Route:          "/patients",
		Cache:          "no-store",
		Guards:         []string{"public"},
		CSRF:           true,
		PageID:         "patients",
		Handler:        "contracts.command.patients.CreatePatient",
		Contract: &contractBindingJSON{
			Name:        "patients.CreatePatient",
			Kind:        "command",
			Status:      "bound",
			ImportAlias: "patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Roles:       []string{"web"},
			Handler:     "HandleCreatePatient",
			Register:    "Register",
		},
	})
	if len(report.Endpoints) != 1 {
		t.Fatalf("expected one contract endpoint, got %#v", report.Endpoints)
	}
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 9 {
		t.Fatalf("expected endpoint source span for contract reference, got %#v", report.Endpoints[0].SourceSpan)
	}
}

func TestRoutesCommandUsesExplicitConfigProjectRootForDiscoveryAndContracts(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

page patients
route "/patients"

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "contracts", "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct {
	Name string `+"`json:\"name\" form:\"name\"`"+`
}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	var output string
	withWorkingDir(t, t.TempDir(), func() {
		captured, err := captureCLIStdout(t, func() error {
			return run([]string{"routes", "--config", config})
		})
		if err != nil {
			t.Fatal(err)
		}
		output = captured
	})

	var report routeMetadataReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("invalid routes JSON: %v\n%s", err, output)
	}
	assertEndpointBinding(t, report.Endpoints, endpointBindingJSON{
		Kind:           "command",
		EndpointSource: "contract",
		Source:         page,
		Package:        "pages",
		Symbol:         "patients.CreatePatient",
		Method:         "POST",
		Route:          "/patients",
		Cache:          "no-store",
		Guards:         []string{"public"},
		CSRF:           true,
		PageID:         "patients",
		Handler:        "contracts.command.patients.CreatePatient",
		Contract: &contractBindingJSON{
			Name:        "patients.CreatePatient",
			Kind:        "command",
			Status:      "bound",
			ImportAlias: "patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Roles:       []string{"web"},
			Handler:     "HandleCreatePatient",
			Register:    "Register",
		},
	})
}

func TestBuildCommandWritesGeneratedEmbeddedApp(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>Generated app</main>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, "--app", appDir, page}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(appDir, "go.mod"),
		filepath.Join(appDir, "cmd", "server", "main.go"),
		filepath.Join(appDir, "gowdkapp", "app.go"),
		filepath.Join(appDir, "gowdkapp", "app", "index.html"),
		filepath.Join(appDir, "gowdkapp", "app", "gowdk-routes.json"),
		filepath.Join(appDir, "gowdkapp", "app", "gowdk-assets.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildCommandPrintsPartialRuntimeAsset(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "patients.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page patients
route "/patients"

act Refresh POST "/patients"

view {
  <main>
    <form g:post={Refresh} g:target="#patients">
      <input name="query" />
    </form>
    <section id="patients">Initial</section>
  </main>
}
`)

	output, err := captureCLIStdout(t, func() error {
		return run([]string{"build", "--config", config, "--out", outputDir, page})
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimePath := filepath.Join(outputDir, "assets", "gowdk", "gowdk.js")
	if !strings.Contains(output, runtimePath) {
		t.Fatalf("expected build output to print runtime asset %q, got:\n%s", runtimePath, output)
	}
	if _, err := os.Stat(runtimePath); err != nil {
		t.Fatalf("expected runtime asset to exist: %v", err)
	}
}

func TestBuildCommandBinRequiresGeneratedApp(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)

	err := run([]string{"build", "--config", config, "--out", t.TempDir(), "--bin", filepath.Join(t.TempDir(), "site")})
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
	err = run([]string{"build", "--config", config, "--out", t.TempDir(), "--wasm", filepath.Join(t.TempDir(), "site.wasm")})
	if err == nil {
		t.Fatal("expected --wasm without --app to fail")
	}
	if !strings.Contains(err.Error(), "--wasm requires --app") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := run([]string{"build", "--out", t.TempDir(), "--app", filepath.Join(t.TempDir(), "app"), "--wasm="}); err == nil {
		t.Fatal("expected empty --wasm to fail")
	}
}

func TestBuildCommandDockerRequiresBinary(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)

	err := run([]string{"build", "--config", config, "--out", t.TempDir(), "--app", filepath.Join(t.TempDir(), "app"), "--docker"})
	if err == nil {
		t.Fatal("expected --docker without --bin to fail")
	}
	if !strings.Contains(err.Error(), "--docker requires --bin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildCommandDockerBaseRequiresDocker(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)

	err := run([]string{"build", "--config", config, "--out", t.TempDir(), "--app", filepath.Join(t.TempDir(), "app"), "--bin", filepath.Join(t.TempDir(), "site"), "--docker-base", "scratch"})
	if err == nil {
		t.Fatal("expected --docker-base without --docker to fail")
	}
	if !strings.Contains(err.Error(), "--docker-base requires --docker") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDockerBinary(t *testing.T) {
	if err := validateDockerBinary(dockerBaseDistroless, dockerBinaryInfo{ELF: false, Static: true}); err == nil || !strings.Contains(err.Error(), "Linux ELF binary") {
		t.Fatalf("expected non-ELF rejection, got %v", err)
	}
	if err := validateDockerBinary(dockerBaseScratch, dockerBinaryInfo{ELF: true, Static: false}); err == nil || !strings.Contains(err.Error(), "statically linked") {
		t.Fatalf("expected scratch static-link rejection, got %v", err)
	}
	if err := validateDockerBinary(dockerBaseScratch, dockerBinaryInfo{ELF: true, Static: true}); err != nil {
		t.Fatalf("expected static ELF scratch binary to pass: %v", err)
	}
}

func TestBuildCommandEmitsDockerArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux cross-compiled test binary path handling is covered on Unix CI")
	}
	t.Setenv("GOOS", "linux")
	t.Setenv("CGO_ENABLED", "0")

	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "bin", "site")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>Dockerized</main>
}
`)

	stdout, err := captureCLIStdout(t, func() error {
		return run([]string{"build", "--config", config, "--out", outputDir, "--app", appDir, "--bin", binaryPath, "--docker", page})
	})
	if err != nil {
		t.Fatal(err)
	}

	dockerfilePath := filepath.Join(root, "bin", "Dockerfile")
	dockerignorePath := filepath.Join(root, "bin", ".dockerignore")
	for _, path := range []string{binaryPath, dockerfilePath, dockerignorePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
		if !strings.Contains(stdout, path) {
			t.Fatalf("expected stdout to include %s, got:\n%s", path, stdout)
		}
	}

	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"FROM gcr.io/distroless/base-debian12",
		`COPY ["site", "/app/site"]`,
		"ENV GOWDK_ADDR=0.0.0.0:8080",
		"USER nonroot:nonroot",
		`ENTRYPOINT ["/app/site"]`,
	} {
		if !strings.Contains(string(dockerfile), expected) {
			t.Fatalf("expected Dockerfile to contain %q:\n%s", expected, dockerfile)
		}
	}

	dockerignore, err := os.ReadFile(dockerignorePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"*", "!Dockerfile", "!.dockerignore", "!site"} {
		if !strings.Contains(string(dockerignore), expected) {
			t.Fatalf("expected .dockerignore to contain %q:\n%s", expected, dockerignore)
		}
	}

	reportPayload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"binary_built", "dockerfile_written", "dockerignore_written", dockerfilePath, dockerignorePath} {
		if !strings.Contains(string(reportPayload), filepath.ToSlash(expected)) {
			t.Fatalf("expected build report to contain %q:\n%s", expected, reportPayload)
		}
	}
}

func TestBuildCommandEmitsStaticDeployRecipe(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>Static recipe</main>
}
`)

	stdout, err := captureCLIStdout(t, func() error {
		return run([]string{"build", "--config", config, "--out", outputDir, "--deploy-recipe", "static", page})
	})
	if err != nil {
		t.Fatal(err)
	}

	recipePath := filepath.Join(outputDir, "deploy", "static-host.md")
	payload, err := os.ReadFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"starting point, not a production guarantee",
		outputDir,
		"gowdk serve --dir",
		"domains, TLS, CDN policy",
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected static recipe to contain %q:\n%s", expected, payload)
		}
	}
	if !strings.Contains(stdout, recipePath) {
		t.Fatalf("expected stdout to include %s, got:\n%s", recipePath, stdout)
	}
	reportPayload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"deploy_recipe_written", "static", recipePath} {
		if !strings.Contains(string(reportPayload), filepath.ToSlash(expected)) {
			t.Fatalf("expected build report to contain %q:\n%s", expected, reportPayload)
		}
	}
}

func TestWriteDeploymentRecipesEmitsBinaryAndSplitRecipes(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	binaryPath := filepath.Join(root, "bin", "site")
	backendBinaryPath := filepath.Join(root, "bin", "backend")

	artifacts, err := writeDeploymentRecipes(deploymentRecipeRequest{
		OutputDir:         outputDir,
		BinaryPath:        binaryPath,
		BackendBinaryPath: backendBinaryPath,
		Recipes:           []string{"systemd", "caddy", "nginx", "split"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 4 {
		t.Fatalf("expected four recipe artifacts, got %#v", artifacts)
	}
	for _, path := range []string{
		filepath.Join(root, "bin", "gowdk-site.service"),
		filepath.Join(root, "bin", "Caddyfile"),
		filepath.Join(root, "bin", "nginx.gowdk.conf"),
		filepath.Join(outputDir, "deploy", "split-frontend-backend.md"),
	} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected recipe %s: %v", path, err)
		}
		for _, expected := range []string{"Starting point", "secret"} {
			if !strings.Contains(strings.ToLower(string(payload)), strings.ToLower(expected)) {
				t.Fatalf("expected %s to contain %q:\n%s", path, expected, payload)
			}
		}
	}
}

func TestDeploymentRecipesRejectUnknownAndUnsupportedShapes(t *testing.T) {
	if _, err := normalizeDeploymentRecipes([]string{"kubernetes"}); err == nil || !strings.Contains(err.Error(), "unsupported deploy recipe") {
		t.Fatalf("expected unknown recipe rejection, got %v", err)
	}
	if _, err := writeDeploymentRecipes(deploymentRecipeRequest{Recipes: []string{"systemd"}, OutputDir: "dist"}); err == nil || !strings.Contains(err.Error(), "requires --bin") {
		t.Fatalf("expected systemd shape rejection, got %v", err)
	}
	if _, err := writeDeploymentRecipes(deploymentRecipeRequest{Recipes: []string{"split"}, OutputDir: "dist"}); err == nil || !strings.Contains(err.Error(), "requires --out") {
		t.Fatalf("expected split shape rejection, got %v", err)
	}
}

func TestBuildCommandBuildsRunnableEmbeddedBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>One binary</main>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
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

func TestBuildCommandBuildsWASMArtifact(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	wasmPath := filepath.Join(root, "site.wasm")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page home
route "/"

view {
  <main>WASM artifact</main>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, "--app", appDir, "--wasm", wasmPath, page}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected wasm artifact to be non-empty")
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

page dashboard
route "/admin"

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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

page dashboard
route "/admin"

view {
  <main>Admin module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "docs", "guide.page.gwdk"), `package app

page guide
route "/docs"

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
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page dashboard
route "/dashboard"

go ssr {
}

build {
  => { title: "Dashboard" }
}

view {
  <main>
    <h1>{title}</h1>
  </main>
}
`)

	if err := run([]string{"build", "--config", config, "--ssr", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "dashboard", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no spa SSR HTML artifact, stat err: %v", err)
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
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

page blog.post
route "/blog/{slug}"

go ssr {
}

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

	if err := run([]string{"build", "--config", config, "--ssr", "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
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

func TestBuildCommandBuildsActionBinaryReturns501ForMissingHandler(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	if err := run([]string{"build", "--config", config, "--out", outputDir, "--app", appDir, "--bin", binaryPath, page}); err != nil {
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
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d: %s", response.StatusCode, payload)
	}
	if !strings.Contains(string(payload), "GOWDK action handler app.Subscribe is not implemented") {
		t.Fatalf("unexpected missing handler body: %s", payload)
	}
}

func TestBuildCommandFailsProductionMissingBackendHandler(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Mode: gowdk.Production,
	},
}
`)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	err := run([]string{"build", "--config", config, "--out", outputDir, page})
	if err == nil {
		t.Fatal("expected production build to fail for missing backend handler")
	}
	if !strings.Contains(err.Error(), "production build requires a bound action handler Subscribe") {
		t.Fatalf("unexpected production backend binding error: %v", err)
	}
}

func TestBuildCommandAllowMissingBackendOverridesProductionConfig(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Mode: gowdk.Production,
	},
}
`)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	if err := run([]string{"build", "--config", config, "--allow-missing-backend", "--out", outputDir, page}); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "newsletter", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(payload)
	if !strings.Contains(html, `<form method="post" action="/newsletter">`) {
		t.Fatalf("expected production build output with explicit missing backend allowance:\n%s", html)
	}
}

func TestBuildCommandProductionBlocksInsecureAuditFindings(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Mode: gowdk.Production,
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	writeCLIFile(t, page, `package app

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"

view {
  <form g:post={Subscribe}>
    <input name="email" required />
    <button type="submit">Subscribe</button>
  </form>
}
`)

	err := run([]string{"build", "--config", config, "--allow-missing-backend", "--out", outputDir, page})
	if err == nil {
		t.Fatal("expected production build to be blocked by error-severity security findings")
	}
	if !strings.Contains(err.Error(), "build blocked by") {
		t.Fatalf("unexpected build audit error: %v", err)
	}

	// --allow-insecure downgrades the production gate to a warning so the build
	// proceeds for 0.x experimentation.
	if err := run([]string{"build", "--config", config, "--allow-missing-backend", "--allow-insecure", "--out", outputDir, page}); err != nil {
		t.Fatalf("expected --allow-insecure to override the security gate: %v", err)
	}
}

func TestBuildCommandBuildsBinaryWithFeatureBoundActionAndAPI(t *testing.T) {
	root := t.TempDir()
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/gowdk-bound

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, filepath.ToSlash(moduleRoot)))
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	writeCLIFile(t, filepath.Join(root, "features", "auth", "auth.page.gwdk"), `package auth

page auth
route "/login"

act Login POST "/login"

api Session GET "/api/session"

view {
  <form g:post={Login}>
    <input name="email" required />
    <button type="submit">Sign in</button>
  </form>
}
`)
	writeCLIFile(t, filepath.Join(root, "features", "auth", "auth.go"), `package auth

import (
	"context"
	"net/http"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

func Login(_ context.Context, values form.Values) (response.Response, error) {
	if values.First("email") != "demo@example.com" {
		return response.Response{}, response.NewHandlerError(http.StatusUnauthorized, "invalid login", nil)
	}
	return response.RedirectTo("/dashboard"), nil
}

func Session(_ context.Context, _ *http.Request) (response.Response, error) {
	return response.JSONValue(http.StatusOK, map[string]any{"authenticated": true})
}
`)
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, ".gowdk", "app")
	binaryPath := filepath.Join(root, "bin", "site")
	withWorkingDir(t, root, func() {
		if err := run([]string{"build", "--out", outputDir, "--app", appDir, "--bin", binaryPath, "features/auth/auth.page.gwdk"}); err != nil {
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

	response, err := waitForCLIStatus("http://"+addr+"/login", http.MethodPost, "email=demo%40example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/dashboard" {
		t.Fatalf("expected bound action redirect, status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on bound action redirect, got %q", cacheControl)
	}

	response, err = waitForCLIStatus("http://"+addr+"/api/session", http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected bound API status 200, got %d: %s", response.StatusCode, body)
	}
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected no-store on bound API response, got %q", cacheControl)
	}
	if !strings.Contains(string(body), `"authenticated":true`) {
		t.Fatalf("unexpected bound API body: %s", body)
	}
}

func TestOutputFileHandlerServesRootIndex(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), `<main>Home</main>`)

	response := httptest.NewRecorder()
	outputFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "<main>Home</main>") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestOutputFileHandlerServesExtensionlessNestedIndex(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "blog", "hello-gowdk", "index.html"), `<main>Post</main>`)

	response := httptest.NewRecorder()
	outputFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/blog/hello-gowdk", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "<main>Post</main>") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestOutputFileHandlerRejectsUnsupportedMethods(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), `<main>Home</main>`)

	response := httptest.NewRecorder()
	outputFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("unexpected Allow header: %q", response.Header().Get("Allow"))
	}
}

func TestOutputFileHandlerDoesNotListDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	outputFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", response.Code, response.Body.String())
	}
}

func TestOutputFileHandlerDoesNotServeSecurityManifest(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk-security.json"), `{"endpoints":[{"path":"/admin"}]}`)

	response := httptest.NewRecorder()
	outputFileHandler(root).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/gowdk-security.json", nil))

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

func TestLiveReloadFileHandlerInjectsScript(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), `<!doctype html><html><body><main>GOWDK</main></body></html>`)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	liveReloadFileHandler(root, newLiveReloadBroker()).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `new EventSource("/__gowdk/reload")`) {
		t.Fatalf("expected live reload script:\n%s", body)
	}
	for _, expected := range []string{
		`__gowdk-error-overlay`,
		`events.addEventListener("build-error"`,
		`events.addEventListener("component-hmr"`,
		`fetchFreshDocument`,
		`GOWDK build failed`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected browser overlay script to contain %q:\n%s", expected, body)
		}
	}
	if strings.Index(body, "<script>") > strings.Index(body, "</body>") {
		t.Fatalf("expected script before body close:\n%s", body)
	}
}

func TestDevRuntimeProxyHandlerInjectsScript(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(w, request)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html><body><main>Generated app</main></body></html>`)
	}))
	defer upstream.Close()

	targetAddr := strings.TrimPrefix(upstream.URL, "http://")
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	devRuntimeProxyHandler(targetAddr, newLiveReloadBroker()).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		`<main>Generated app</main>`,
		`new EventSource("/__gowdk/reload")`,
		`__gowdk-error-overlay`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected proxied HTML to contain %q:\n%s", expected, body)
		}
	}
}

func TestLiveReloadEventSerializesBuildErrors(t *testing.T) {
	var output strings.Builder
	writeLiveReloadEvent(&output, liveReloadEvent{
		Name: "build-error",
		Data: "pages/home.page.gwdk:12: invalid view\nbuild failed",
	})

	got := output.String()
	for _, expected := range []string{
		"event: build-error\n",
		"data: pages/home.page.gwdk:12: invalid view\n",
		"data: build failed\n",
		"\n",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected serialized event to contain %q, got:\n%s", expected, got)
		}
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
	content = withPublicGuardForPageFixture(path, content)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withPublicGuardForPageFixture(path, content string) string {
	if !strings.HasSuffix(path, ".page.gwdk") ||
		strings.Contains(content, "\nguard ") {
		return content
	}
	lines := strings.SplitAfter(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "route ") {
			lines[index] = line + "guard public\n"
			return strings.Join(lines, "")
		}
	}
	return content
}

func writeMinimalCLIConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, path, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{}
`)
	return path
}

func writeCLITestModule(t *testing.T, root string, modulePath string) {
	t.Helper()
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module %s

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, modulePath, filepath.ToSlash(moduleRoot)))
}

func assertGoBinding(t *testing.T, bindings []goBindingJSON, kind, symbol, status string) {
	t.Helper()
	binding, ok := findGoBinding(bindings, kind, symbol)
	if !ok {
		t.Fatalf("missing %s binding %s in %#v", kind, symbol, bindings)
	}
	if binding.Status != status {
		t.Fatalf("expected %s %s status %q, got %#v", kind, symbol, status, binding)
	}
}

func findGoBinding(bindings []goBindingJSON, kind, symbol string) (goBindingJSON, bool) {
	for _, binding := range bindings {
		if binding.Kind == kind && binding.Symbol == symbol {
			return binding, true
		}
	}
	return goBindingJSON{}, false
}

type inspectIRGolden struct {
	Version  int `json:"Version"`
	Packages []struct {
		Name       string   `json:"Name"`
		SourceDirs []string `json:"SourceDirs"`
		Files      []struct {
			Path    string `json:"Path"`
			Kind    string `json:"Kind"`
			Package string `json:"Package"`
			Name    string `json:"Name"`
		} `json:"Files"`
	} `json:"Packages"`
	Pages []struct {
		Source  string   `json:"Source"`
		Package string   `json:"Package"`
		ID      string   `json:"ID"`
		Route   string   `json:"Route"`
		Guards  []string `json:"Guards"`
	} `json:"Pages"`
	Routes []struct {
		Kind    string   `json:"Kind"`
		Method  string   `json:"Method"`
		Path    string   `json:"Path"`
		PageID  string   `json:"PageID"`
		Package string   `json:"Package"`
		Render  string   `json:"Render"`
		Guards  []string `json:"Guards"`
		Source  string   `json:"Source"`
	} `json:"Routes"`
	Endpoints []struct {
		Kind       string `json:"Kind"`
		Source     string `json:"Source"`
		Package    string `json:"Package"`
		PageID     string `json:"PageID"`
		Symbol     string `json:"Symbol"`
		Method     string `json:"Method"`
		Path       string `json:"Path"`
		SourceFile string `json:"SourceFile"`
		Binding    struct {
			Status       string `json:"Status"`
			PackageName  string `json:"PackageName"`
			FunctionName string `json:"FunctionName"`
		} `json:"Binding"`
	} `json:"Endpoints"`
	Templates []struct {
		OwnerKind string   `json:"OwnerKind"`
		OwnerID   string   `json:"OwnerID"`
		Package   string   `json:"Package"`
		Source    string   `json:"Source"`
		Route     string   `json:"Route"`
		Guards    []string `json:"Guards"`
		Body      string   `json:"Body"`
	} `json:"Templates"`
}

type inspectTreeGolden struct {
	Version int             `json:"version"`
	Root    inspectTreeNode `json:"root"`
}

type inspectTreeNode struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	Source   string            `json:"source"`
	Span     *sourceSpanJSON   `json:"span"`
	Props    map[string]any    `json:"props"`
	Children []inspectTreeNode `json:"children"`
}

type endpointGraphGolden struct {
	Version int                 `json:"version"`
	Nodes   []endpointGraphNode `json:"nodes"`
	Edges   []endpointGraphEdge `json:"edges"`
}

type endpointGraphNode struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Name   string          `json:"name"`
	Source string          `json:"source"`
	Span   *sourceSpanJSON `json:"span"`
	Props  map[string]any  `json:"props"`
}

type endpointGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

func canonicalInspectIRGolden(t *testing.T, payload []byte) string {
	t.Helper()
	var report inspectIRGolden
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("invalid inspect ir golden JSON: %v\n%s", err, payload)
	}
	canonical, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(canonical)
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

func doctorReportHasCheck(checks any, id, status string) bool {
	value := reflect.ValueOf(checks)
	if value.Kind() != reflect.Slice {
		return false
	}
	for index := 0; index < value.Len(); index++ {
		check := value.Index(index)
		if check.Kind() == reflect.Pointer {
			check = check.Elem()
		}
		if check.Kind() != reflect.Struct {
			continue
		}
		idField := check.FieldByName("ID")
		statusField := check.FieldByName("Status")
		if idField.IsValid() && statusField.IsValid() && idField.Kind() == reflect.String && statusField.Kind() == reflect.String &&
			idField.String() == id && statusField.String() == status {
			return true
		}
	}
	return false
}

func testSourceLine(source string, line int) string {
	lines := strings.Split(source, "\n")
	if line <= 0 || line > len(lines) {
		return ""
	}
	return lines[line-1]
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

func captureCLIOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	previousStdout := os.Stdout
	previousStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = previousStdout
		os.Stderr = previousStderr
	}()

	runErr := fn()
	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), runErr
}

func assertRouteBinding(t *testing.T, routes []routeBindingJSON, expected routeBindingJSON) {
	t.Helper()
	for _, route := range routes {
		if route.Kind != expected.Kind ||
			route.Method != expected.Method ||
			route.Route != expected.Route ||
			route.PageID != expected.PageID ||
			route.Handler != expected.Handler {
			continue
		}
		return
	}
	t.Fatalf("missing route %#v in %#v", expected, routes)
}

func assertEndpointBinding(t *testing.T, endpoints []endpointBindingJSON, expected endpointBindingJSON) {
	t.Helper()
	for _, endpoint := range endpoints {
		if endpoint.Kind != expected.Kind ||
			endpoint.EndpointSource != expected.EndpointSource ||
			endpoint.Source != expected.Source ||
			endpoint.Package != expected.Package ||
			endpoint.PackagePath != expected.PackagePath ||
			endpoint.PackageName != expected.PackageName ||
			endpoint.Symbol != expected.Symbol ||
			endpoint.Method != expected.Method ||
			endpoint.Route != expected.Route ||
			endpoint.Cache != expected.Cache ||
			!reflect.DeepEqual(endpoint.Guards, expected.Guards) ||
			endpoint.CSRF != expected.CSRF ||
			endpoint.PageID != expected.PageID ||
			endpoint.Handler != expected.Handler ||
			endpoint.BindingStatus != expected.BindingStatus ||
			endpoint.Signature != expected.Signature ||
			endpoint.InputType != expected.InputType {
			continue
		}
		if expected.BackendBinding != nil {
			if endpoint.BackendBinding == nil || *endpoint.BackendBinding != *expected.BackendBinding {
				continue
			}
		}
		if expected.Contract != nil {
			if endpoint.Contract == nil || !contractBindingEqual(endpoint.Contract, expected.Contract) {
				continue
			}
		}
		return
	}
	t.Fatalf("missing endpoint binding %#v in %#v", expected, endpoints)
}

func contractBindingEqual(actual *contractBindingJSON, expected *contractBindingJSON) bool {
	if actual.Name != expected.Name ||
		actual.Kind != expected.Kind ||
		actual.Status != expected.Status ||
		actual.Message != expected.Message ||
		actual.ImportAlias != expected.ImportAlias ||
		actual.ImportPath != expected.ImportPath ||
		actual.Type != expected.Type ||
		actual.Result != expected.Result ||
		actual.Handler != expected.Handler ||
		actual.Register != expected.Register ||
		len(actual.Roles) != len(expected.Roles) {
		return false
	}
	for index := range actual.Roles {
		if actual.Roles[index] != expected.Roles[index] {
			return false
		}
	}
	return true
}

func findInspectNode(node inspectTreeNode, kind string, name string) (inspectTreeNode, bool) {
	if node.Kind == kind && node.Name == name {
		return node, true
	}
	for _, child := range node.Children {
		if found, ok := findInspectNode(child, kind, name); ok {
			return found, true
		}
	}
	return inspectTreeNode{}, false
}

func inspectPropListContains(props map[string]any, key string, value string) bool {
	raw, ok := props[key]
	if !ok {
		return false
	}
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func findGraphNode(nodes []endpointGraphNode, kind string, name string) (endpointGraphNode, bool) {
	for _, node := range nodes {
		if node.Kind == kind && node.Name == name {
			return node, true
		}
	}
	return endpointGraphNode{}, false
}

func graphPropBool(props map[string]any, key string) bool {
	value, _ := props[key].(bool)
	return value
}

func graphPropListContains(props map[string]any, key string, value string) bool {
	raw, ok := props[key]
	if !ok {
		return false
	}
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func hasGraphEdge(edges []endpointGraphEdge, from string, to string, kind string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}

func hasGraphEdgeTo(edges []endpointGraphEdge, to string, kind string) bool {
	for _, edge := range edges {
		if edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}

func hasGraphEdgeToKind(edges []endpointGraphEdge, nodes []endpointGraphNode, from string, toKind string, kind string) bool {
	nodeKinds := map[string]string{}
	for _, node := range nodes {
		nodeKinds[node.ID] = node.Kind
	}
	for _, edge := range edges {
		if edge.From == from && edge.Kind == kind && nodeKinds[edge.To] == toKind {
			return true
		}
	}
	return false
}

func assertRouteInfo(t *testing.T, infos []routeInfoJSON, code string, pageID string) {
	t.Helper()
	for _, info := range infos {
		if info.Code == code && info.PageID == pageID {
			return
		}
	}
	t.Fatalf("missing route info code=%s page=%s in %#v", code, pageID, infos)
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

func TestLanguageServerConfigLoadsProjectConfigAddons(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		ssr.Addon(),
	},
}
`)

	withWorkingDir(t, root, func() {
		config, err := languageServerConfig(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(config.Addons) != 1 || config.Addons[0].Name() != "ssr" {
			t.Fatalf("expected config-declared ssr addon, got %#v", config.Addons)
		}
	})
}

func TestLanguageServerConfigSupportsConfigFlag(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "custom.config.go")
	writeCLIFile(t, configPath, `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		ssr.Addon(),
	},
}
`)

	for _, args := range [][]string{
		{"--config", configPath},
		{"--config=" + configPath},
	} {
		config, err := languageServerConfig(args)
		if err != nil {
			t.Fatalf("languageServerConfig(%v): %v", args, err)
		}
		if len(config.Addons) != 1 || config.Addons[0].Name() != "ssr" {
			t.Fatalf("languageServerConfig(%v): expected ssr addon, got %#v", args, config.Addons)
		}
	}
}

func TestLanguageServerConfigFallsBackWithoutConfigFile(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root, func() {
		config, err := languageServerConfig(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(config.Addons) != 0 {
			t.Fatalf("expected zero config without gowdk.config.go, got %#v", config.Addons)
		}

		config, err = languageServerConfig([]string{"--ssr"})
		if err != nil {
			t.Fatal(err)
		}
		if len(config.Addons) != 1 || config.Addons[0].Name() != "ssr" {
			t.Fatalf("expected --ssr addon without gowdk.config.go, got %#v", config.Addons)
		}
	})
}

func TestLanguageServerConfigRejectsUnknownArguments(t *testing.T) {
	for _, args := range [][]string{
		{"extra.gwdk"},
		{"--unknown"},
		{"--config"},
	} {
		if _, err := languageServerConfig(args); err == nil || !strings.Contains(err.Error(), "usage: gowdk lsp") {
			t.Fatalf("languageServerConfig(%v): expected usage error, got %v", args, err)
		}
	}
}

func TestDoctorHonorsJSONFlagRegardlessOfOrderOnArgumentErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--bad-flag", "--json"},
		{"--json", "--bad-flag"},
	} {
		report, jsonOutput := runDoctor(args)
		if !jsonOutput {
			t.Fatalf("runDoctor(%v): expected JSON output to be requested", args)
		}
		if report.Status != "error" || !doctorReportHasCheck(report.Checks, "arguments", "error") {
			t.Fatalf("runDoctor(%v): unexpected report %#v", args, report)
		}
	}
}

func TestLiveReloadBrokerNotifyIsNoOpOnNilBroker(t *testing.T) {
	var broker *liveReloadBroker
	broker.notify("reload")
	broker.notifyData("build-error", "boom")
}
