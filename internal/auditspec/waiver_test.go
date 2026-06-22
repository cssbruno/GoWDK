package auditspec

import (
	"testing"
	"time"

	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

func waiverTestManifest() securitymanifest.SecurityManifest {
	return securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{{
			ID:             "Submit",
			Kind:           "action",
			Method:         "POST",
			Path:           "/submit",
			Guards:         []string{"role:admin"},
			GuardEvidence:  []securitymanifest.GuardEvidence{{ID: "role:admin", Kind: "native-rbac", BindingStatus: "resolved-native", Evidence: securitymanifest.EvidenceVerifiedStatic}},
			CSRF:           false,
			BodyLimitBytes: 1 << 20,
			RequestLimits:  securitymanifest.RequestLimitPosture{RawBodyBytes: 1 << 20, InstalledBeforeParse: true},
			Source:         "signup.page.gwdk:4",
		}},
	}
}

func waiverPolicy(attrs map[string]string) Policy {
	return Policy{
		Name:   "project.waivers",
		Source: "waivers.audit.gwdk:1",
		Rules: []Rule{{
			Kind:   RuleWaive,
			Value:  "audit_action_missing_csrf",
			Source: "waivers.audit.gwdk:2",
			Attrs:  attrs,
		}},
	}
}

func findingByCode(findings []Finding, code string) (Finding, bool) {
	for _, finding := range findings {
		if finding.Code == code {
			return finding, true
		}
	}
	return Finding{}, false
}

var waiverNow = time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

func TestValidWaiverSuppressesFindingAndRecordsSuppression(t *testing.T) {
	policies := append(Baseline(), waiverPolicy(map[string]string{
		"target":        "action:Submit",
		"owner":         "team-x",
		"justification": "legacy endpoint, migrating Q3",
		"expires":       "2027-01-01",
		"ticket":        "SEC-9",
	}))
	findings := EvaluateWithWaivers(waiverTestManifest(), policies, WaiverContext{Now: waiverNow})

	finding, ok := findingByCode(findings, "audit_action_missing_csrf")
	if !ok {
		t.Fatalf("expected the csrf finding to remain (waived), got %#v", findings)
	}
	if finding.Suppression == nil {
		t.Fatalf("waived finding should carry suppression metadata, got %#v", finding)
	}
	if finding.Suppression.Owner != "team-x" || finding.Suppression.Ticket != "SEC-9" {
		t.Fatalf("unexpected suppression metadata: %#v", finding.Suppression)
	}
	if finding.Evidence != string(securitymanifest.EvidenceWaived) {
		t.Fatalf("waived finding evidence = %q, want waived", finding.Evidence)
	}
	summary := Summarize(findings)
	if summary.Errors != 0 || summary.Waived != 1 {
		t.Fatalf("a justified waiver must not block: errors=%d waived=%d", summary.Errors, summary.Waived)
	}
}

func TestMalformedWaiverDoesNotSuppress(t *testing.T) {
	policies := append(Baseline(), waiverPolicy(map[string]string{
		"target": "action:Submit",
		"owner":  "team-x",
		// justification and expires intentionally omitted.
	}))
	findings := EvaluateWithWaivers(waiverTestManifest(), policies, WaiverContext{Now: waiverNow})

	finding, _ := findingByCode(findings, "audit_action_missing_csrf")
	if finding.Suppression != nil {
		t.Fatalf("malformed waiver must not suppress, got %#v", finding.Suppression)
	}
	if _, ok := findingByCode(findings, "audit_waiver_malformed"); !ok {
		t.Fatalf("expected audit_waiver_malformed finding, got %#v", findings)
	}
	if Summarize(findings).Errors == 0 {
		t.Fatal("the underlying finding plus the malformed waiver must keep an error")
	}
}

func TestExpiredWaiverDoesNotSuppress(t *testing.T) {
	policies := append(Baseline(), waiverPolicy(map[string]string{
		"target":        "action:Submit",
		"owner":         "team-x",
		"justification": "stale",
		"expires":       "2020-01-01",
	}))
	findings := EvaluateWithWaivers(waiverTestManifest(), policies, WaiverContext{Now: waiverNow})

	if finding, _ := findingByCode(findings, "audit_action_missing_csrf"); finding.Suppression != nil {
		t.Fatalf("expired waiver must not suppress, got %#v", finding.Suppression)
	}
	if _, ok := findingByCode(findings, "audit_waiver_expired"); !ok {
		t.Fatalf("expected audit_waiver_expired finding, got %#v", findings)
	}
}

func TestUnmatchedWaiverIsReported(t *testing.T) {
	policies := append(Baseline(), waiverPolicy(map[string]string{
		"target":        "action:DoesNotExist",
		"owner":         "team-x",
		"justification": "typo",
		"expires":       "2027-01-01",
	}))
	findings := EvaluateWithWaivers(waiverTestManifest(), policies, WaiverContext{Now: waiverNow})

	if finding, _ := findingByCode(findings, "audit_action_missing_csrf"); finding.Suppression != nil {
		t.Fatal("a waiver for a different target must not suppress the real finding")
	}
	if _, ok := findingByCode(findings, "audit_waiver_unmatched"); !ok {
		t.Fatalf("expected audit_waiver_unmatched finding, got %#v", findings)
	}
}

func TestDigestPinnedWaiverInvalidatesOnDrift(t *testing.T) {
	policies := append(Baseline(), waiverPolicy(map[string]string{
		"target":         "action:Submit",
		"owner":          "team-x",
		"justification":  "pinned to a reviewed build",
		"expires":        "2027-01-01",
		"posture_digest": "sha256:reviewed",
	}))
	ctx := WaiverContext{Now: waiverNow, PostureDigest: "sha256:current-drifted"}
	findings := EvaluateWithWaivers(waiverTestManifest(), policies, ctx)

	if finding, _ := findingByCode(findings, "audit_action_missing_csrf"); finding.Suppression != nil {
		t.Fatal("a posture-digest-pinned waiver must not suppress after the posture drifts")
	}
	if _, ok := findingByCode(findings, "audit_waiver_digest_mismatch"); !ok {
		t.Fatalf("expected audit_waiver_digest_mismatch finding, got %#v", findings)
	}
}

func TestWaiverCannotSuppressPolicyResolutionFinding(t *testing.T) {
	// A waiver must not be able to silence a policy-resolution error such as a
	// baseline override; those are structural, not posture findings.
	declared := Policy{
		Name:      "baseline.actions",
		Source:    "weaken.audit.gwdk:1",
		Selectors: []Selector{{Raw: "act:*", Kind: SelectorEndpoint}},
		Rules:     []Rule{{Kind: RuleRequireHeader, Value: "X-Whatever", Code: "audit_headers_missing"}},
	}
	policies := append(Baseline(), declared, waiverPolicy(map[string]string{
		"target":        "*",
		"owner":         "team-x",
		"justification": "trying to silence the override",
		"expires":       "2027-01-01",
	}))
	// Point the waiver at the override code.
	policies[len(policies)-1].Rules[0].Value = "policy_baseline_override"

	findings := EvaluateWithWaivers(waiverTestManifest(), policies, WaiverContext{Now: waiverNow})
	finding, ok := findingByCode(findings, "policy_baseline_override")
	if !ok {
		t.Fatalf("expected policy_baseline_override to remain, got %#v", findings)
	}
	if finding.Suppression != nil {
		t.Fatal("a waiver must not suppress a policy-resolution finding")
	}
}
