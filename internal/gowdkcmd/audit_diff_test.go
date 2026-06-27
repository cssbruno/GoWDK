package gowdkcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
)

func enrichedFinding(code, target, source, message string) auditspec.Finding {
	enriched := auditspec.EnrichFindings([]auditspec.Finding{{
		Code:     code,
		Severity: diagnostics.SeverityError,
		Target:   target,
		Source:   source,
		Message:  message,
	}})
	return enriched[0]
}

func TestComputeAuditDiffClassifiesIntroducedResolvedUnchanged(t *testing.T) {
	previous := []auditspec.Finding{
		enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:10", "missing csrf"),
		enrichedFinding("audit_raw_html_sink", "component:Old", "old.gwdk:3", "raw html"),
	}
	current := []auditspec.Finding{
		// previous[0] moved to a different line: same fingerprint -> unchanged.
		enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:42", "missing csrf"),
		// A brand new finding -> introduced.
		enrichedFinding("audit_contract_roleless", "contract:New", "new.gwdk:7", "roleless"),
	}

	diff := computeAuditDiff("sha256:prev", previous, current)
	if diff.Unchanged != 1 {
		t.Fatalf("want 1 unchanged (line moved), got %d", diff.Unchanged)
	}
	if len(diff.Introduced) != 1 || diff.Introduced[0].Code != "audit_contract_roleless" {
		t.Fatalf("want 1 introduced roleless finding, got %#v", diff.Introduced)
	}
	if diff.IntroducedErrors != 1 {
		t.Fatalf("want 1 introduced error, got %d", diff.IntroducedErrors)
	}
	if len(diff.Resolved) != 1 || diff.Resolved[0].Code != "audit_raw_html_sink" {
		t.Fatalf("want 1 resolved raw-html finding, got %#v", diff.Resolved)
	}
}

func TestFindingFingerprintIsStableAcrossLineMovement(t *testing.T) {
	a := enrichedFinding("audit_action_missing_csrf", "action:A", "pages/a.gwdk:10", "missing csrf")
	b := enrichedFinding("audit_action_missing_csrf", "action:A", "pages/a.gwdk:999", "missing csrf")
	if a.Fingerprint == "" || a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprint must ignore line numbers: %q vs %q", a.Fingerprint, b.Fingerprint)
	}
	c := enrichedFinding("audit_action_missing_csrf", "action:B", "pages/a.gwdk:10", "missing csrf")
	if a.Fingerprint == c.Fingerprint {
		t.Fatal("fingerprint must change when the target changes")
	}
}

func TestComputeAuditDiffIgnoresWaivedFindings(t *testing.T) {
	waived := enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:1", "missing csrf")
	waived.Suppression = &auditspec.Suppression{Owner: "sec"}
	diff := computeAuditDiff("base", nil, []auditspec.Finding{waived})
	if len(diff.Introduced) != 0 || diff.IntroducedErrors != 0 {
		t.Fatalf("waived findings must not count as introduced: %#v", diff)
	}
}

func TestComputeAuditDiffTreatsNowWaivedFindingAsUnchangedNotResolved(t *testing.T) {
	active := enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:10", "missing csrf")
	// The same finding, still present, but waived in the current report.
	waivedNow := enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:10", "missing csrf")
	waivedNow.Suppression = &auditspec.Suppression{Owner: "sec"}

	diff := computeAuditDiff("base", []auditspec.Finding{active}, []auditspec.Finding{waivedNow})
	if len(diff.Resolved) != 0 {
		t.Fatalf("a still-present but now-waived finding must not be reported resolved: %#v", diff.Resolved)
	}
	if diff.Unchanged != 1 {
		t.Fatalf("a still-present finding (now waived) should be unchanged, got %d", diff.Unchanged)
	}
}

func TestComputeAuditDiffTreatsUnwaivedFindingAsUnchangedNotIntroduced(t *testing.T) {
	waivedBefore := enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:10", "missing csrf")
	waivedBefore.Suppression = &auditspec.Suppression{Owner: "sec"}
	activeNow := enrichedFinding("audit_action_missing_csrf", "action:A", "a.gwdk:10", "missing csrf")

	diff := computeAuditDiff("base", []auditspec.Finding{waivedBefore}, []auditspec.Finding{activeNow})
	if len(diff.Introduced) != 0 || diff.IntroducedErrors != 0 {
		t.Fatalf("re-activating a previously waived finding must not read as newly introduced: %#v", diff.Introduced)
	}
	if diff.Unchanged != 1 {
		t.Fatalf("a finding present in both reports should be unchanged, got %d", diff.Unchanged)
	}
}

func TestLoadPreviousAuditReportReEnrichesFingerprints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prev.json")
	// A report written by an older tool that did not store fingerprints.
	payload := `{"postureDigest":"sha256:x","findings":[{"code":"audit_action_missing_csrf","severity":"error","target":"action:A","source":"a.gwdk:3","message":"missing csrf"}]}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := loadPreviousAuditReport(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(previous.Findings) != 1 || previous.Findings[0].Fingerprint == "" {
		t.Fatalf("expected a re-enriched fingerprint, got %#v", previous.Findings)
	}
	if got := previousReportBaseline(previous, path); got != "sha256:x" {
		t.Fatalf("baseline should prefer the posture digest, got %q", got)
	}
}

func TestLoadPreviousAuditReportRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPreviousAuditReport(path); err == nil {
		t.Fatal("expected an error for an unparseable previous report")
	}
}

func TestAuditErrorExitCodePrefersRuntimeFailure(t *testing.T) {
	staticOnly := []auditspec.Finding{
		{Code: "audit_action_missing_csrf", Severity: diagnostics.SeverityError},
	}
	if got := auditErrorExitCode(staticOnly); got != auditExitErrorFindings {
		t.Fatalf("static error findings should exit %d, got %d", auditExitErrorFindings, got)
	}
	withRuntime := append([]auditspec.Finding{}, staticOnly...)
	withRuntime = append(withRuntime, auditspec.Finding{Code: "audit_test_failed", Severity: diagnostics.SeverityError})
	if got := auditErrorExitCode(withRuntime); got != auditExitRuntimeFailure {
		t.Fatalf("a runtime test failure should exit %d, got %d", auditExitRuntimeFailure, got)
	}
}
