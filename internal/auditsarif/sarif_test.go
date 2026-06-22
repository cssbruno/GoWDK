package auditsarif

import (
	"encoding/json"
	"testing"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
)

func sampleFindings() []auditspec.Finding {
	return []auditspec.Finding{
		{
			Code:        "audit_action_missing_csrf",
			Severity:    diagnostics.SeverityError,
			Target:      "action:Submit",
			Source:      "pages/signup.page.gwdk:12",
			Message:     "action endpoint does not enforce CSRF",
			Remediation: "Enable CSRF.",
			Fingerprint: "abc123",
			CWE:         []string{"CWE-352"},
			OWASP:       []string{"A01:2021-Broken Access Control"},
			Confidence:  "high",
			Evidence:    "verified-static",
			Policy:      "baseline",
		},
		{
			Code:        "audit_raw_html_sink",
			Severity:    diagnostics.SeverityError,
			Target:      "component:Card",
			Source:      "components/card.gwdk:4",
			Message:     "raw HTML sink",
			Fingerprint: "def456",
		},
		{
			Code:        "audit_action_missing_csrf",
			Severity:    diagnostics.SeverityError,
			Target:      "action:Waived",
			Source:      "runtime",
			Message:     "waived finding",
			Fingerprint: "waived789",
			Suppression: &auditspec.Suppression{Owner: "sec", Justification: "tracked", Expires: "2099-01-01"},
		},
	}
}

func TestFromFindingsProducesValidSARIFShape(t *testing.T) {
	doc := FromFindings(sampleFindings(), Options{ToolVersion: "9.9.9", InformationURI: "https://gowdk.dev"})

	if doc.Version != "2.1.0" || doc.Schema == "" {
		t.Fatalf("unexpected SARIF envelope: version=%q schema=%q", doc.Version, doc.Schema)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("want exactly one run, got %d", len(doc.Runs))
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Name != "gowdk audit" || run.Tool.Driver.Version != "9.9.9" {
		t.Fatalf("unexpected driver: %#v", run.Tool.Driver)
	}
	// Two distinct codes -> two rules, sorted and deduplicated.
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("want 2 deduped rules, got %d: %#v", len(run.Tool.Driver.Rules), run.Tool.Driver.Rules)
	}
	if run.Tool.Driver.Rules[0].ID != "audit_action_missing_csrf" {
		t.Fatalf("rules not sorted by id: %#v", run.Tool.Driver.Rules)
	}
	if run.Tool.Driver.Rules[0].Name != "AuditActionMissingCsrf" {
		t.Fatalf("unexpected rule name: %q", run.Tool.Driver.Rules[0].Name)
	}
	if len(run.Results) != 3 {
		t.Fatalf("want 3 results, got %d", len(run.Results))
	}
}

func TestFromFindingsMapsLocationsLevelsFingerprintsAndSuppressions(t *testing.T) {
	doc := FromFindings(sampleFindings(), Options{})
	results := doc.Runs[0].Results

	first := results[0]
	if first.Level != "error" {
		t.Fatalf("want error level, got %q", first.Level)
	}
	if got := first.PartialFingerprints[FingerprintKey]; got != "abc123" {
		t.Fatalf("partial fingerprint = %q, want abc123", got)
	}
	if len(first.Locations) != 1 {
		t.Fatalf("want one location, got %d", len(first.Locations))
	}
	loc := first.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "pages/signup.page.gwdk" {
		t.Fatalf("artifact uri = %q, want pages/signup.page.gwdk", loc.ArtifactLocation.URI)
	}
	if loc.Region == nil || loc.Region.StartLine != 12 {
		t.Fatalf("region = %#v, want startLine 12", loc.Region)
	}

	// The waived finding becomes a suppressed result whose location falls back to
	// the target because "runtime" is not a file path.
	waived := results[2]
	if len(waived.Suppressions) != 1 || waived.Suppressions[0].Kind != "external" {
		t.Fatalf("waived finding must carry an external suppression: %#v", waived.Suppressions)
	}
	if waived.Suppressions[0].Justification == "" {
		t.Fatal("suppression must carry a justification")
	}
	if waived.Locations[0].PhysicalLocation.ArtifactLocation.URI != "action:Waived" {
		t.Fatalf("non-file source should fall back to target, got %q", waived.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
	if waived.Locations[0].PhysicalLocation.Region != nil {
		t.Fatal("a non-file source must not carry a line region")
	}
}

func TestFromFindingsMarshalsToStableJSON(t *testing.T) {
	doc := FromFindings(sampleFindings(), Options{ToolVersion: "1.0.0"})
	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal SARIF: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(payload, &round); err != nil {
		t.Fatalf("SARIF is not valid JSON: %v", err)
	}
	if round["$schema"] == nil || round["runs"] == nil {
		t.Fatalf("SARIF missing required top-level keys: %v", round)
	}
}

func TestSarifLevelMapsInfoToNote(t *testing.T) {
	if got := sarifLevel(diagnostics.SeverityInfo); got != "note" {
		t.Fatalf("info severity should map to note, got %q", got)
	}
}
