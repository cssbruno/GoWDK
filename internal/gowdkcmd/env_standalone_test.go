package gowdkcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDoesNotRequireRuntimeSecretsAndEnvCheckDoes(t *testing.T) {
	root := t.TempDir()
	secretName := "GOWDK_TEST_DEPLOYMENT_SECRET_SPLIT"
	if err := os.Unsetenv(secretName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(secretName) })
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Env: gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "`+secretName+`", Required: true, MinBytes: 16},
		},
	},
}
`)
	source := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>Environment split</main>
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"check", source})
		})
		if err != nil {
			t.Fatalf("static check must not require runtime secrets: err=%v stderr=%s", err, stderr)
		}
		if strings.TrimSpace(stdout) != "ok" {
			t.Fatalf("unexpected check output: stdout=%q stderr=%q", stdout, stderr)
		}

		_, _, err = captureCLIOutput(t, func() error {
			return run([]string{"env", "check"})
		})
		if err == nil || !strings.Contains(err.Error(), secretName) {
			t.Fatalf("expected env check to report missing %s, got %v", secretName, err)
		}

		if err := os.Setenv(secretName, "deployment-secret-value"); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, err = captureCLIOutput(t, func() error {
			return run([]string{"env", "check"})
		})
		if err != nil {
			t.Fatalf("env check with secret: err=%v stderr=%s", err, stderr)
		}
		if !strings.Contains(stdout, "environment ok (0 variables, 1 secrets)") {
			t.Fatalf("unexpected env check output: %q", stdout)
		}
	})
}

func TestEnvCheckJSONDoesNotExposeSecretValues(t *testing.T) {
	root := t.TempDir()
	secretName := "GOWDK_TEST_DEPLOYMENT_SECRET_JSON"
	secretValue := "do-not-print-this-secret"
	t.Setenv(secretName, secretValue)
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Env: gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "`+secretName+`", Required: true, MinBytes: 16},
		},
	},
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"env", "check", "--json"})
		})
		if err != nil || stderr != "" {
			t.Fatalf("env check --json: err=%v stderr=%q", err, stderr)
		}
		if strings.Contains(stdout, secretValue) {
			t.Fatalf("JSON report exposed a secret value: %s", stdout)
		}
		var report envCheckReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Status != "ok" || report.Secrets != 1 {
			t.Fatalf("unexpected report: %#v", report)
		}
	})
}

func TestCheckAutomaticallyUsesStandaloneModeWithoutProjectConfig(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>Detached</main>
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"check", source})
		})
		if err != nil || stderr != "" {
			t.Fatalf("standalone check: err=%v stderr=%q", err, stderr)
		}
		if strings.TrimSpace(stdout) != "ok (standalone)" {
			t.Fatalf("unexpected standalone output: %q", stdout)
		}

		stdout, stderr, err = captureCLIOutput(t, func() error {
			return run([]string{"check", "--json", source})
		})
		if err != nil || stderr != "" {
			t.Fatalf("standalone JSON check: err=%v stderr=%q", err, stderr)
		}
		var report struct {
			Mode string `json:"mode"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Mode != "standalone" {
			t.Fatalf("unexpected standalone JSON mode: %q", report.Mode)
		}
	})
}

func TestStandaloneCheckDoesNotFailSSRAddonDiagnostic(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "dashboard.page.gwdk")
	writeCLIFile(t, source, `package app

page dashboard
route "/dashboard"
guard public

server {
}

view {
  <main>Dashboard</main>
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"check", source})
		})
		if err != nil {
			t.Fatalf("standalone SSR check: err=%v stdout=%q stderr=%q", err, stdout, stderr)
		}
		if stdout != "" || !strings.Contains(stderr, "project context required") || !strings.Contains(stderr, "server or Go-block configuration") {
			t.Fatalf("unexpected standalone SSR output: stdout=%q stderr=%q", stdout, stderr)
		}

		stdout, stderr, err = captureCLIOutput(t, func() error {
			return run([]string{"check", "--json", source})
		})
		if err != nil || stderr != "" {
			t.Fatalf("standalone SSR JSON check: err=%v stderr=%q", err, stderr)
		}
		var report struct {
			Mode        string `json:"mode"`
			Diagnostics []struct {
				Code     string `json:"code"`
				Severity string `json:"severity"`
			} `json:"diagnostics"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Mode != "standalone" {
			t.Fatalf("unexpected standalone JSON mode: %q", report.Mode)
		}
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Code == "missing_ssr_addon" {
				t.Fatalf("standalone JSON still reported missing_ssr_addon: %s", stdout)
			}
			if diagnostic.Severity == "error" {
				t.Fatalf("standalone JSON reported an error diagnostic: %s", stdout)
			}
		}
	})
}

func TestExplicitStandaloneCheckDoesNotExecuteProjectConfig(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, source, `package app

page home
route "/"
guard public

view {
  <main>No config execution</main>
}
`)
	writeCLIFile(t, filepath.Join(root, "gowdk.config.go"), `package app

import "github.com/cssbruno/gowdk"

var Config = func() gowdk.Config {
	panic("standalone check executed project config")
}()
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"check", "--standalone", source})
		})
		if err != nil || stderr != "" {
			t.Fatalf("explicit standalone check: err=%v stderr=%q", err, stderr)
		}
		if strings.TrimSpace(stdout) != "ok (standalone)" {
			t.Fatalf("unexpected standalone output: %q", stdout)
		}
	})
}

func TestStandaloneCheckReportsProjectDependentCoverage(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "settings.page.gwdk")
	writeCLIFile(t, source, `package app

page settings
route "/settings"
guard public

act Save POST "/settings"

view {
  <main><form g:post={Save}><button>Save</button></form></main>
}
`)

	withWorkingDir(t, root, func() {
		stdout, stderr, err := captureCLIOutput(t, func() error {
			return run([]string{"check", "--standalone", source})
		})
		if err != nil {
			t.Fatalf("standalone project-dependent check: err=%v stdout=%q stderr=%q", err, stdout, stderr)
		}
		if stdout != "" || !strings.Contains(stderr, "project context required") || !strings.Contains(stderr, "backend handler binding") {
			t.Fatalf("unexpected standalone coverage output: stdout=%q stderr=%q", stdout, stderr)
		}
	})
}
