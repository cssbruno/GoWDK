package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestAuditCommandPassesCleanProject(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	writeCLIFile(t, filepath.Join(root, "home.page.gwdk"), `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, filepath.Join(root, "home.page.gwdk")})
	})
	if err != nil {
		t.Fatalf("expected clean audit to succeed: %v", err)
	}

	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	if report.Version != 1 || report.Status != "ok" {
		t.Fatalf("unexpected audit report: status=%q version=%d", report.Status, report.Version)
	}
	if report.Summary.Errors != 0 || len(report.Findings) != 0 {
		t.Fatalf("expected no findings for a clean project: %#v", report.Findings)
	}
	if report.Summary.Routes != 1 {
		t.Fatalf("expected one route in posture, got %d", report.Summary.Routes)
	}
}

func TestAuditCommandFlagsMissingCSRFAndExitsNonZero(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	// writeCLIFile injects `guard public`, and the minimal config leaves CSRF
	// disabled, so the action endpoint must trip audit_action_missing_csrf.
	writeCLIFile(t, filepath.Join(root, "signup.page.gwdk"), `package app

page signup
route "/signup"

act Submit POST "/submit"

view {
  <main>Sign up</main>
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, filepath.Join(root, "signup.page.gwdk")})
	})
	if err == nil {
		t.Fatal("expected non-zero exit when an error finding exists")
	}
	if _, silent := err.(interface{ SilentCLIError() }); !silent {
		t.Fatalf("audit error should be a silent CLI error, got %T", err)
	}

	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	if report.Status != "fail" || report.Summary.Errors == 0 {
		t.Fatalf("expected a failing audit, got status=%q errors=%d", report.Status, report.Summary.Errors)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "audit_action_missing_csrf" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected audit_action_missing_csrf finding, got %#v", report.Findings)
	}
}
