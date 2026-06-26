package gowdkcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk/internal/auditschema"
)

// writeAuditCSRFFailingProject writes a project whose action endpoint trips
// audit_action_missing_csrf, so the audit produces at least one error finding to
// exercise the CI-native output surfaces.
func writeAuditCSRFFailingProject(t *testing.T, root string) (config, page string) {
	t.Helper()
	config = filepath.Join(root, "gowdk.config.go")
	writeCLIFile(t, config, `package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		CSRF: gowdk.CSRFConfig{Disabled: true},
	},
}
`)
	page = filepath.Join(root, "signup.page.gwdk")
	writeCLIFile(t, page, `package app

page signup
route "/signup"

act Submit POST "/submit"

view {
  <main>Signup</main>
}
`)
	return config, page
}

func TestAuditCommandSchemaFlagPublishesVersionedContracts(t *testing.T) {
	cases := map[string][]string{
		"default":  {"audit", "--schema"},
		"report":   {"audit", "--schema=report"},
		"security": {"audit", "--schema=security"},
	}
	for name, args := range cases {
		stdout, _, err := captureCLIOutput(t, func() error { return run(args) })
		if err != nil {
			t.Fatalf("audit %v: %v", args, err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("schema %q is not valid JSON: %v", name, err)
		}
		if _, ok := doc["$id"].(string); !ok {
			t.Fatalf("schema %q is missing $id", name)
		}
	}

	_, _, err := captureCLIOutput(t, func() error { return run([]string{"audit", "--schema=bogus"}) })
	if err == nil {
		t.Fatal("expected an error for an unknown schema name")
	}
	if code := exitCodeFor(err); code != auditExitInvalidSource {
		t.Fatalf("an unknown schema should exit %d, got %d", auditExitInvalidSource, code)
	}
}

func TestAuditCommandEmitsSARIF(t *testing.T) {
	root := t.TempDir()
	config, page := writeAuditCSRFFailingProject(t, root)

	// Bare --sarif: SARIF is the stdout payload, and the finding gate still trips.
	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--sarif", "--config", config, page})
	})
	if err == nil {
		t.Fatal("expected the finding gate to trip even when emitting SARIF")
	}
	if code := exitCodeFor(err); code != auditExitErrorFindings {
		t.Fatalf("error findings should exit %d, got %d", auditExitErrorFindings, code)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("SARIF stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if doc["version"] != "2.1.0" {
		t.Fatalf("unexpected SARIF version: %v", doc["version"])
	}

	// --sarif=<file>: SARIF goes to the sidecar file; stdout stays the JSON report.
	sarifPath := filepath.Join(root, "audit.sarif")
	stdout, _, err = captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--sarif=" + sarifPath, "--config", config, page})
	})
	if err == nil {
		t.Fatal("expected the finding gate to trip")
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout should remain the JSON report: %v", err)
	}
	sarifBytes, readErr := os.ReadFile(sarifPath)
	if readErr != nil {
		t.Fatalf("expected a SARIF sidecar file: %v", readErr)
	}
	var sarifDoc map[string]any
	if err := json.Unmarshal(sarifBytes, &sarifDoc); err != nil {
		t.Fatalf("SARIF file is not valid JSON: %v", err)
	}
	runs, _ := sarifDoc["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("expected one SARIF run, got %d", len(runs))
	}
}

func TestAuditReportJSONStaysWithinPublishedSchema(t *testing.T) {
	root := t.TempDir()
	config, page := writeAuditCSRFFailingProject(t, root)
	stdout, _, _ := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, page})
	})

	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &top); err != nil {
		t.Fatalf("audit JSON: %v", err)
	}
	assertJSONKeysDescribed(t, []byte(stdout), auditschema.AuditReport)

	var findings []json.RawMessage
	if err := json.Unmarshal(top["findings"], &findings); err != nil {
		t.Fatalf("findings: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding to validate the finding sub-schema")
	}
	for _, finding := range findings {
		assertJSONKeysDescribed(t, finding, auditschema.AuditReport, "$defs", "finding")
	}

	assertJSONKeysDescribed(t, top["manifest"], auditschema.SecurityManifest)
}

func TestAuditCommandDiffGatesOnlyIntroducedErrors(t *testing.T) {
	root := t.TempDir()
	config, page := writeAuditCSRFFailingProject(t, root)

	baseline, _, _ := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--config", config, page})
	})
	baselinePath := filepath.Join(root, "baseline.json")
	if err := os.WriteFile(baselinePath, []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}

	// Diff vs self: the finding is pre-existing -> no introduced errors -> exit 0.
	stdout, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--diff", baselinePath, "--config", config, page})
	})
	if err != nil {
		t.Fatalf("pre-existing findings must not gate a diff run: %v", err)
	}
	var report auditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("diff JSON: %v", err)
	}
	if report.Diff == nil || len(report.Diff.Introduced) != 0 || report.Diff.Unchanged == 0 {
		t.Fatalf("expected a diff with 0 introduced and >0 unchanged: %#v", report.Diff)
	}

	// Diff vs an empty baseline: the finding is newly introduced -> exit 3.
	emptyPath := filepath.Join(root, "empty.json")
	if err := os.WriteFile(emptyPath, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = captureCLIOutput(t, func() error {
		return run([]string{"audit", "--json", "--diff=" + emptyPath, "--config", config, page})
	})
	if err == nil {
		t.Fatal("expected newly introduced errors to gate the diff run")
	}
	if code := exitCodeFor(err); code != auditExitErrorFindings {
		t.Fatalf("introduced error findings should exit %d, got %d", auditExitErrorFindings, code)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("diff JSON: %v", err)
	}
	if report.Diff == nil || report.Diff.IntroducedErrors == 0 {
		t.Fatalf("expected introduced errors against an empty baseline: %#v", report.Diff)
	}
}

func TestAuditCommandInvalidSourceExitsTwo(t *testing.T) {
	root := t.TempDir()
	config := writeMinimalCLIConfig(t, root)
	badPath := filepath.Join(root, "broken.page.gwdk")
	writeCLIFile(t, badPath, "this is not valid gwdk syntax {{{\n")
	_, _, err := captureCLIOutput(t, func() error {
		return run([]string{"audit", "--config", config, badPath})
	})
	if err == nil {
		t.Fatal("expected invalid source to fail the audit")
	}
	if code := exitCodeFor(err); code != auditExitInvalidSource {
		t.Fatalf("invalid source should exit %d, got %d", auditExitInvalidSource, code)
	}
}

// assertJSONKeysDescribed asserts every key of the JSON object in raw appears in
// the named schema at the given path. It is the drift guard: a new emitted field
// that the published schema does not describe fails the build.
func assertJSONKeysDescribed(t *testing.T, raw []byte, name auditschema.Name, path ...string) {
	t.Helper()
	described, err := auditschema.DescribedKeys(name, path...)
	if err != nil {
		t.Fatalf("DescribedKeys(%q, %v): %v", name, path, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	for key := range obj {
		if !described[key] {
			t.Fatalf("schema %q does not describe emitted key %q at %v; update internal/auditschema/schema", name, key, path)
		}
	}
}
