package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestDoctorCommandReportsLoadedEnvFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	secretName := "GOWDK_TEST_DOCTOR_ENV_FILE_SECRET"
	_ = os.Unsetenv(secretName)
	t.Cleanup(func() { _ = os.Unsetenv(secretName) })
	config := writeEnvContractCLIConfig(t, root, secretName, 32)
	envPath := filepath.Join(root, ".env")
	writeCLIFile(t, envPath, secretName+"=doctor-secret-value-32-bytes-long\n")
	writeCLIFile(t, source, `package app

page home
route "/"

view {
  <main>Doctor env</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"doctor", "--json", "--config", config, source})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var decoded struct {
		Environment struct {
			EnvFilePath string `json:"envFilePath"`
			EnvFile     string `json:"envFile"`
		} `json:"environment"`
		Checks []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("expected JSON doctor output, got %q: %v", stdout, err)
	}
	if decoded.Environment.EnvFilePath != envPath || decoded.Environment.EnvFile != "loaded" {
		t.Fatalf("expected loaded env file in environment: %#v", decoded.Environment)
	}
	if !doctorReportHasCheck(decoded.Checks, "env_file", "ok") {
		t.Fatalf("expected env_file ok check: %#v", decoded.Checks)
	}
	if strings.Contains(stdout, "doctor-secret-value") {
		t.Fatalf("doctor output must not include secret values:\n%s", stdout)
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

server {
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
