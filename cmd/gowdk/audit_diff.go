package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
)

// auditDiff classifies the current findings against a previous report by stable
// fingerprint. It is the surface behind `gowdk audit --diff <previous-report>`
// so CI can gate only newly introduced findings instead of every pre-existing
// one. Matching is by fingerprint, which is independent of line movement, so
// reformatting or relocating code does not surface an old finding as new.
type auditDiff struct {
	Baseline           string                `json:"baseline"`
	Introduced         []auditspec.Finding   `json:"introduced"`
	Resolved           []auditDiffFindingRef `json:"resolved"`
	Unchanged          int                   `json:"unchanged"`
	IntroducedErrors   int                   `json:"introducedErrors"`
	IntroducedWarnings int                   `json:"introducedWarnings"`
}

// auditDiffFindingRef is the compact identity of a finding that is no longer
// present, recorded so a resolved finding stays traceable without duplicating
// the full finding payload.
type auditDiffFindingRef struct {
	Code        string               `json:"code"`
	Severity    diagnostics.Severity `json:"severity,omitempty"`
	Fingerprint string               `json:"fingerprint"`
	Target      string               `json:"target,omitempty"`
	Source      string               `json:"source,omitempty"`
}

// previousAuditReport is the subset of a prior `gowdk audit --json` report that
// the diff reads. Decoding is lenient so a report from a compatible version
// still diffs even as the report grows fields.
type previousAuditReport struct {
	PostureDigest string              `json:"postureDigest"`
	Findings      []auditspec.Finding `json:"findings"`
}

// loadPreviousAuditReport reads and re-enriches a prior report's findings so
// fingerprints are present and computed identically to the current run, even if
// the stored report predates a given enrichment field.
func loadPreviousAuditReport(path string) (previousAuditReport, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return previousAuditReport{}, fmt.Errorf("read previous audit report %q: %w", path, err)
	}
	var previous previousAuditReport
	if err := json.Unmarshal(payload, &previous); err != nil {
		return previousAuditReport{}, fmt.Errorf("parse previous audit report %q: %w", path, err)
	}
	previous.Findings = auditspec.EnrichFindings(previous.Findings)
	return previous, nil
}

// computeAuditDiff compares current enriched findings against a previous set.
// Waived (suppressed) findings are excluded from both the gateable introduced
// and resolved sets so a justified suppression never reads as a regression or a
// fix.
func computeAuditDiff(baseline string, previous, current []auditspec.Finding) auditDiff {
	previousSet := fingerprintSet(previous)
	currentSet := fingerprintSet(current)

	diff := auditDiff{
		Baseline:   baseline,
		Introduced: []auditspec.Finding{},
		Resolved:   []auditDiffFindingRef{},
	}
	for _, finding := range current {
		if finding.Suppression != nil || finding.Fingerprint == "" {
			continue
		}
		if previousSet[finding.Fingerprint] {
			diff.Unchanged++
			continue
		}
		diff.Introduced = append(diff.Introduced, finding)
		switch finding.Severity {
		case diagnostics.SeverityError:
			diff.IntroducedErrors++
		case diagnostics.SeverityWarning:
			diff.IntroducedWarnings++
		}
	}
	for _, finding := range previous {
		if finding.Suppression != nil || finding.Fingerprint == "" {
			continue
		}
		if !currentSet[finding.Fingerprint] {
			diff.Resolved = append(diff.Resolved, auditDiffFindingRef{
				Code:        finding.Code,
				Severity:    finding.Severity,
				Fingerprint: finding.Fingerprint,
				Target:      finding.Target,
				Source:      finding.Source,
			})
		}
	}
	return diff
}

func fingerprintSet(findings []auditspec.Finding) map[string]bool {
	set := make(map[string]bool, len(findings))
	for _, finding := range findings {
		if finding.Suppression != nil || finding.Fingerprint == "" {
			continue
		}
		set[finding.Fingerprint] = true
	}
	return set
}

// previousReportBaseline names the prior report for the diff: its posture digest
// when present, otherwise the file path it was read from.
func previousReportBaseline(previous previousAuditReport, path string) string {
	if previous.PostureDigest != "" {
		return previous.PostureDigest
	}
	return path
}
