package main

import (
	"encoding/json"
	"fmt"
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

	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestBuildCommandWritesIndexHTML(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	if err := os.WriteFile(source, []byte(`package app

@page home
@route "/"

view {
  <main>
    <h1>GOWDK & friends</h1>
  </main>
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

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

func TestBuildCommandDebugPrintsBuildgenReport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, source, `package app

@page home
@route "/"

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
	if !strings.Contains(stderr, "gowdk build report (build):") {
		t.Fatalf("expected debug report header on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "validate/manifest_valid") || !strings.Contains(stderr, "complete/build_complete") {
		t.Fatalf("expected validation and completion events on stderr, got:\n%s", stderr)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected build report artifact: %v", err)
	}
}

func TestBuildCommandReportsBoundContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

@page patients
@route "/patients"

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
		if event.Data["line"] != "8" || event.Data["column"] != strconv.Itoa(wantColumn) {
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

@page patients
@route "/patients"

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
		if event.Data["line"] != "8" || event.Data["column"] != strconv.Itoa(wantColumn) {
			t.Fatalf("unexpected query source location: %#v", event.Data)
		}
		return
	}
	t.Fatalf("missing contract_reference event in report: %s", payload)
}

func TestCheckJSONReportsMissingContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

@page patients
@route "/patients"

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
		!strings.Contains(output, `"line": 8`) {
		t.Fatalf("expected missing contract diagnostic with source span, got:\n%s", output)
	}
}

func TestCheckJSONReportsInvalidGoContractRegistration(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	page := filepath.Join(root, "pages", "patients.page.gwdk")
	writeCLIFile(t, page, `package pages

@page patients
@route "/patients"

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

func TestBuildFailsForMissingContractReference(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pageSource := `package pages

@page patients
@route "/patients"

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

@page patients
@route "/patients"

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

func TestDevRejectsInvalidInterval(t *testing.T) {
	err := run([]string{"dev", "--interval", "0s", "--out", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "dev interval must be positive") {
		t.Fatalf("expected invalid interval error, got %v", err)
	}
}

func TestBuildCommandRequiresProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

@page home
@route "/"

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

@page home
@route "/"

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

@page home
@route "/"

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

@page home
@route "/"

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

@page home
@route "/"

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

@page home
@route "/"

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

func TestBuildIncrementalSPAUsesChangedPageSources(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, home, `package app

@page home
@route "/"

view {
  <main>Before</main>
}
`)
	writeCLIFile(t, about, `package app

@page about
@route "/about"

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

@page home
@route "/"

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

func TestBuildIncrementalSPAFallsBackForComponentChanges(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page home
@route "/"

view {
  <main><Hero title="GOWDK" /></main>
}
`)
	writeCLIFile(t, component, `package app

@component Hero

props {
  title string
}

view {
  <h1>{title}</h1>
}
`)

	used, err := buildIncrementalSPA([]string{"--config", config, "--out", outputDir, page, component}, inputChange{Changed: []string{component}})
	if err != nil {
		t.Fatal(err)
	}
	if used {
		t.Fatal("expected component change to fall back to full build")
	}
}

func TestBuildCommandWritesComponentExpandedHTML(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	component := filepath.Join(root, "hero.cmp.gwdk")
	outputDir := filepath.Join(root, "dist")
	config := writeMinimalCLIConfig(t, root)
	if err := os.WriteFile(page, []byte(`package app

@page home
@route "/"

view {
  <main>
    <Hero title="GOWDK" />
  </main>
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(component, []byte(`package app

@component Hero

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

@page home
@route "/"

view {
  <main>
    <Hero title="Discovered" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `package app

@component Hero

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

@page stale
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
	writeCLIFile(t, filepath.Join(root, "src", "home.page.gwdk"), `package app

@page home
@route "/"

view {
  <main>
    <Hero title="Configured" />
  </main>
}
`)
	writeCLIFile(t, filepath.Join(root, "src", "hero.cmp.gwdk"), `package app

@component Hero

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

@page ignored
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
		`<label class="gowdk-field"><span>Email</span><input name="email" type="email"></input></label>`,
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

@page home
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

@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "ui2", "second.page.gwdk"), `package app

@page second
@route "/second"

view {
  <main>Frontend Two</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

@page admin
@route "/admin"

view {
  <main>Backend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "ignored.page.gwdk"), `package app

@page ignored
@route "/ignored"

view {
  <main>Ignored</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "other", "stray.page.gwdk"), `package app

@page stray
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

@page admin
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

@page home
@route "/"

view {
  <main>Public module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

@page dashboard
@route "/admin"

view {
  <main>Admin module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "api", "status.page.gwdk"), `package app

@page status
@route "/api/status"

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

@page home
@route "/"

view {
  <main>Public module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

@page dashboard
@route "/admin"

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

@page home
@route "/"

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

@page home
@route "/"

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

@page home
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
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

@page home
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

@page bad
@route "/bad"
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

func TestManifestCommandHandlesMultipleFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	about := filepath.Join(root, "about.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, home, `package app

@page home
@route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, about, `package app

@page about
@route "/about"

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

@page home
@route "/"

view {
  <main>Configured check</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `package app

@page ignored
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
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

@page home
@route "/"

view {
  <main>Manifest discovery</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "pages", "ignored.page.gwdk"), `package app

@page ignored
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

func TestManifestCommandDefaultDiscoverySkipsTestdata(t *testing.T) {
	root := t.TempDir()
	writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "pages", "home.page.gwdk"), `package app

@page home
@route "/"

view {
  <main>Home</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "internal", "lang", "testdata", "home.page.gwdk"), `package app

@page home
@route "/fixture"

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

@page dashboard
@route "/dashboard"

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

@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

@page admin
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

func TestRoutesCommandPrintsRoutesAndEndpoints(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "newsletter.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page newsletter
@route "/newsletter"

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
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 6 {
		t.Fatalf("expected endpoint source span for action declaration, got %#v", report.Endpoints[0].SourceSpan)
	}
	assertRouteInfo(t, report.Info, "ssr_disabled", "newsletter")
}

func TestRoutesCommandPrintsSSRRouteKind(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "dashboard.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page dashboard
@route "/dashboard"

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

@page dashboard
@route "/dashboard"

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

@page home
@route "/"

view {
  <main>Frontend</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "backend", "admin.page.gwdk"), `package app

@page admin
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

@page status
@route "/status"

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
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 6 {
		t.Fatalf("expected endpoint source span for API declaration, got %#v", report.Endpoints[0].SourceSpan)
	}
}

func TestRoutesCommandPrintsContractEndpoints(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "patients.page.gwdk")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package pages

@page patients
@route "/patients"

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
	if report.Endpoints[0].SourceSpan == nil || report.Endpoints[0].SourceSpan.Start.Line != 8 {
		t.Fatalf("expected endpoint source span for contract reference, got %#v", report.Endpoints[0].SourceSpan)
	}
}

func TestBuildCommandWritesGeneratedEmbeddedApp(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page home
@route "/"

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

@page patients
@route "/patients"

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

func TestBuildCommandBuildsRunnableEmbeddedBinary(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "app")
	binaryPath := filepath.Join(root, "site")
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page home
@route "/"

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

@page home
@route "/"

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

@page home
@route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

@page dashboard
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
	writeCLIFile(t, filepath.Join(root, "frontend", "home.page.gwdk"), `package app

@page home
@route "/"

view {
  <main>Frontend module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "admin", "dashboard.page.gwdk"), `package app

@page dashboard
@route "/admin"

view {
  <main>Admin module</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "docs", "guide.page.gwdk"), `package app

@page guide
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
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page dashboard
@route "/dashboard"

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

@page blog.post
@route "/blog/{slug}"

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
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, page, `package app

@page newsletter
@route "/newsletter"

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

@page newsletter
@route "/newsletter"

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

@page newsletter
@route "/newsletter"

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

var Config = gowdk.Config{}
`)
	writeCLIFile(t, filepath.Join(root, "features", "auth", "auth.page.gwdk"), `package auth

@page auth
@route "/login"

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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
