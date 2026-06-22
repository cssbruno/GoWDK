package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if report.Schema != "gowdk.audit.report.v1" || report.Tool.Version != version || report.PolicyDigest == "" || report.PostureDigest == "" || report.BuildMode != "development" {
		t.Fatalf("audit report missing triage metadata: %#v", report)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("expected raw JSON audit output, got %q: %v", stdout, err)
	}
	var target map[string]any
	if err := json.Unmarshal(raw["target"], &target); err != nil {
		t.Fatalf("expected target JSON object, got %q: %v", raw["target"], err)
	}
	if _, ok := target["projectRoot"]; ok || strings.Contains(string(raw["target"]), root) {
		t.Fatalf("audit target metadata should not expose project root %q: %s", root, raw["target"])
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
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	// writeCLIFile injects `guard public`; the explicit CSRF opt-out should
	// trip audit_action_missing_csrf.
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

func TestAuditCommandTreatsActionCSRFAsEnabledByDefault(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
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
	if err != nil {
		t.Fatalf("expected default CSRF action posture to pass audit: %v", err)
	}

	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	for _, finding := range report.Findings {
		if finding.Code == "audit_action_missing_csrf" {
			t.Fatalf("did not expect missing-CSRF finding with default config: %#v", report.Findings)
		}
	}
}

func TestAuditCommandAppliesDeclaredAuditPolicy(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pagePath := filepath.Join(root, "admin.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page admin
route "/admin"

view {
  <main>Admin</main>
}
`)
	auditPath := filepath.Join(root, "security.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

policy admin {
  match "/admin"
  require guard "role:admin"
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, pagePath, auditPath})
	})
	if err == nil {
		t.Fatal("expected declared policy to fail audit")
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "audit_required_guard_missing" && finding.Policy == "admin" && finding.Source == pagePath+":4" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected declared policy guard finding with page source, got %#v", report.Findings)
	}
}

func TestAuditCommandReportsDeclaredPolicyResolutionFindings(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	auditPath := filepath.Join(root, "security.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

policy broken extends missing {
  match "/"
  deny public
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, pagePath, auditPath})
	})
	if err == nil {
		t.Fatal("expected policy resolution failure")
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "policy_unknown_extends" && finding.Policy == "broken" && finding.Source == auditPath+":3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown extends finding with audit source, got %#v", report.Findings)
	}
}

func TestAuditCommandEmitsStandaloneAuditTests(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSecurityHeaders(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-emit")
	writeCLIFile(t, filepath.Join(root, "model.go"), `package app

type Model struct{}
`)
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	testPath := filepath.Join(root, "security_audit_test.go")

	_, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--emit-tests=" + testPath, pagePath})
	})
	if err != nil {
		t.Fatalf("expected audit emit-tests to succeed: %v", err)
	}
	if !strings.Contains(stderr, "wrote audit tests: "+testPath) {
		t.Fatalf("expected emitted test path on stderr, got %q", stderr)
	}
	payload, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		auditGeneratedMarker,
		auditGeneratedMetadataPrefix,
		"posture=sha256:",
		"policy=sha256:",
		"package app_test",
		`gowdktestkit "github.com/cssbruno/gowdk/runtime/testkit"`,
		`Root: fstest.MapFS{`,
		`SecurityHeaders: map[string]string{`,
		`Name:       "route serves /"`,
		`Name:       "security header X-Frame-Options"`,
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("expected emitted test to contain %q:\n%s", expected, payload)
		}
	}
}

func TestAuditCommandEmitTestsRefusesUserOwnedFile(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSecurityHeaders(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-emit-owned")
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	testPath := filepath.Join(root, "security_audit_test.go")
	original := "package app_test\n\nfunc TestUserOwned(t *testing.T) {}\n"
	writeCLIFile(t, testPath, original)

	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--emit-tests=" + testPath, pagePath})
	})
	if err == nil || !strings.Contains(err.Error(), "not a gowdk-generated audit test") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected user-owned overwrite refusal, got %v", err)
	}
	payload, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != original {
		t.Fatalf("user-owned file was overwritten:\n%s", payload)
	}
}

func TestAuditCommandEmitTestsForceOverwritesUserOwnedFile(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSecurityHeaders(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-emit-force")
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	testPath := filepath.Join(root, "security_audit_test.go")
	writeCLIFile(t, testPath, "package app_test\n\nfunc TestUserOwned(t *testing.T) {}\n")

	_, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--emit-tests=" + testPath, "--force", pagePath})
	})
	if err != nil {
		t.Fatalf("expected forced audit emit-tests to succeed: %v\nstderr:\n%s", err, stderr)
	}
	payload, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), auditGeneratedMarker) || !strings.Contains(string(payload), "package app_test") {
		t.Fatalf("expected forced generated audit test payload, got:\n%s", payload)
	}
	if strings.Contains(string(payload), "TestUserOwned") {
		t.Fatalf("expected forced write to replace previous file:\n%s", payload)
	}
}

func TestAuditCommandRunsGeneratedAuditTests(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSecurityHeaders(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-run")
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)

	_, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--run", pagePath})
	})
	if err != nil {
		t.Fatalf("expected generated audit tests to pass: %v", err)
	}
	if !strings.Contains(stderr, "audit generated app tests passed:") ||
		!strings.Contains(stderr, filepath.Join("gowdkapp", "gowdk_audit_test.go")) {
		t.Fatalf("expected generated app audit test pass message, got %q", stderr)
	}
}

func TestAuditCommandRunSupportsActorExpectationsAgainstGeneratedApp(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSSR(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-run-actor")
	pagePath := filepath.Join(root, "admin.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page admin
route "/admin"
guard role:admin

go server {
}

view {
  <main>Admin</main>
}
`)
	auditPath := filepath.Join(root, "security.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

test admin {
  expect GET "/admin" as "role:admin" status 200
  expect GET "/admin" as "anonymous" status 403
}
`)

	_, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--run", pagePath, auditPath})
	})
	if err != nil {
		t.Fatalf("expected generated app audit actor tests to pass: %v\nstderr:\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "audit generated app tests passed:") {
		t.Fatalf("expected generated app audit test pass message, got %q", stderr)
	}
}

func TestAuditCommandRunReportsMissingCustomGuardFixtures(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSSR(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-run-custom-guard")
	pagePath := filepath.Join(root, "admin.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page admin
route "/admin"
guard auth.required

go server {
}

view {
  <main>Admin</main>
}
`)

	stdout, stderr, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--run", pagePath})
	})
	if err != nil {
		t.Fatalf("expected missing custom guard fixtures to report as a finding, got error: %v\nstderr:\n%s", err, stderr)
	}
	output := stdout + "\n" + stderr
	if strings.Contains(output, "audit generated app tests passed:") {
		t.Fatalf("custom guard audit run must not claim runtime verification, got %q", output)
	}
	if !strings.Contains(output, "audit_guard_unverified") || !strings.Contains(output, "auth.required") || !strings.Contains(output, "explicit fixtures") {
		t.Fatalf("expected unresolved custom guard finding, got %q", output)
	}
}

func TestAuditCommandReportsRuntimeAuditTestFailure(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-run-fail")
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	auditPath := filepath.Join(root, "security.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

test mismatch {
  expect GET "/" status 403
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, "--run", pagePath, auditPath})
	})
	if err == nil {
		t.Fatal("expected runtime audit test mismatch to fail audit")
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "audit_test_failed" && finding.Target == "runtime" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected audit_test_failed finding, got %#v", report.Findings)
	}
}

func writeAuditCLIConfigWithSecurityHeaders(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, path, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		SecurityHeaders: gowdk.SecurityHeadersConfig{
			Enabled: true,
			Headers: map[string]string{
				"X-Frame-Options": "DENY",
			},
		},
	},
}
`)
	return path
}

func TestAuditCommandReportsEvidenceClassification(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
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
	if err != nil {
		t.Fatalf("expected audit to succeed for a CSRF-protected action: %v", err)
	}

	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}

	if report.Posture.Obligations == 0 {
		t.Fatalf("expected posture obligations to be reported, got %#v", report.Posture)
	}
	if report.Posture.UnverifiedAppOwned == 0 {
		t.Fatalf("expected app-owned obligations to be surfaced honestly, got %#v", report.Posture)
	}

	appOwned := map[string]string{}
	for _, obligation := range report.Manifest.Obligations {
		appOwned[obligation.ID] = string(obligation.Evidence)
	}
	for _, id := range []string{"auth.authentication", "session.management", "authz.resource", "authz.domain"} {
		if appOwned[id] != "unverified-app-owned" {
			t.Fatalf("expected %q to be unverified-app-owned, got %q (obligations=%#v)", id, appOwned[id], report.Manifest.Obligations)
		}
	}
	if appOwned["request.body-limit"] != "verified-static" || appOwned["request.csrf"] != "verified-static" {
		t.Fatalf("expected request obligations verified-static, got %#v", report.Manifest.Obligations)
	}

	for _, route := range report.Manifest.Routes {
		for _, guard := range route.GuardEvidence {
			if guard.Evidence == "" {
				t.Fatalf("route guard evidence missing classification: %#v", guard)
			}
		}
	}
}

func TestAuditCommandRecordsExplicitWaiver(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	pagePath := filepath.Join(root, "signup.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page signup
route "/signup"

act Submit POST "/submit"

view {
  <main>Sign up</main>
}
`)
	auditPath := filepath.Join(root, "waivers.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

policy waivers {
  waive audit_action_missing_csrf target "action:Submit" owner "team-x" justification "legacy endpoint, migrating Q3" expires "2099-01-01" ticket "SEC-9"
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, pagePath, auditPath})
	})
	if err != nil {
		t.Fatalf("expected the waived finding not to fail the audit: %v", err)
	}

	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	if report.Summary.Errors != 0 || report.Summary.Waived != 1 {
		t.Fatalf("expected the waiver to suppress the error, got %#v", report.Summary)
	}
	if len(report.Waivers) != 1 || report.Waivers[0].Code != "audit_action_missing_csrf" || report.Waivers[0].Owner != "team-x" || report.Waivers[0].Ticket != "SEC-9" {
		t.Fatalf("expected an applied-waiver record in the report, got %#v", report.Waivers)
	}
	if len(report.Manifest.Waivers) != 1 || report.Manifest.Waivers[0].Ticket != "SEC-9" {
		t.Fatalf("expected gowdk-security.json to record the declared waiver, got %#v", report.Manifest.Waivers)
	}
}

func TestAuditCommandRejectsBaselineOverride(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	auditPath := filepath.Join(root, "override.audit.gwdk")
	writeCLIFile(t, auditPath, `package app

policy baseline.actions {
  match "act:*"
  require header "X-Whatever"
}
`)

	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, pagePath, auditPath})
	})
	if err == nil {
		t.Fatal("expected a same-name baseline override to fail the audit")
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "policy_baseline_override" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected policy_baseline_override finding, got %#v", report.Findings)
	}
}

func TestAuditCommandCheckTestsDetectsStaleAndFreshState(t *testing.T) {
	root := t.TempDir()
	config := writeAuditCLIConfigWithSecurityHeaders(t, root)
	writeCLITestModule(t, root, "example.com/gowdk-audit-check")
	writeCLIFile(t, filepath.Join(root, "model.go"), `package app

type Model struct{}
`)
	pagePath := filepath.Join(root, "home.page.gwdk")
	writeCLIFile(t, pagePath, `package app

page home
route "/"

view {
  <main>Home</main>
}
`)
	testPath := filepath.Join(root, "security_audit_test.go")

	// Missing checked-in test is reported as stale.
	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, "--check-tests=" + testPath, pagePath})
	})
	if err == nil {
		t.Fatal("expected --check-tests to fail when no generated test is committed")
	}
	if !auditReportHasCode(t, stdout, "audit_test_stale") {
		t.Fatalf("expected audit_test_stale for a missing test, got %s", stdout)
	}

	// Emit and commit the test, then a fresh check passes.
	if _, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--emit-tests=" + testPath, pagePath})
	}); err != nil {
		t.Fatalf("emit-tests: %v", err)
	}
	if _, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, "--check-tests=" + testPath, pagePath})
	}); err != nil {
		t.Fatalf("expected a freshly emitted test to pass --check-tests: %v", err)
	}

	// Changing the posture (a new route) makes the committed test stale.
	aboutPath := filepath.Join(root, "about.page.gwdk")
	writeCLIFile(t, aboutPath, `package app

page about
route "/about"

view {
  <main>About</main>
}
`)
	stdout, _, err = captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, "--check-tests=" + testPath, pagePath, aboutPath})
	})
	if err == nil {
		t.Fatal("expected --check-tests to fail after the posture changed")
	}
	if !auditReportHasCode(t, stdout, "audit_test_stale") {
		t.Fatalf("expected audit_test_stale after posture change, got %s", stdout)
	}
}

func auditReportHasCode(t *testing.T, stdout, code string) bool {
	t.Helper()
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected JSON audit output, got %q: %v", stdout, err)
	}
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func writeAuditCLIConfigWithSSR(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, path, `package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{ssr.Addon()},
}
`)
	return path
}
