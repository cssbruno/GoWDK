package gowdkcmd

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
// Introduced/resolved/unchanged classify by *presence* (a fingerprint exists at
// all, whether active or waived), so toggling a waiver on a finding that is still
// present reads as unchanged rather than as a fix or a regression. Only active
// (non-suppressed) findings feed the gateable introduced counts, so a waived
// finding never trips the introduced-error gate.
func computeAuditDiff(baseline string, previous, current []auditspec.Finding) auditDiff {
	previousPresence := fingerprintPresenceSet(previous)
	currentPresence := fingerprintPresenceSet(current)

	diff := auditDiff{
		Baseline:   baseline,
		Introduced: []auditspec.Finding{},
		Resolved:   []auditDiffFindingRef{},
	}
	for _, finding := range current {
		if finding.Fingerprint == "" {
			continue
		}
		present := previousPresence[finding.Fingerprint]
		if finding.Suppression != nil {
			// Waived now: it cannot gate. If the same finding existed before, it
			// persists (unchanged); a brand-new pre-waived finding is just tracked
			// as a waiver, not an introduced regression.
			if present {
				diff.Unchanged++
			}
			continue
		}
		if present {
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
		// A finding is resolved only if it was actively reported before and is no
		// longer present at all now. A finding that merely became waived is still
		// present (in currentPresence), so it is not counted as resolved.
		if finding.Suppression != nil || finding.Fingerprint == "" {
			continue
		}
		if !currentPresence[finding.Fingerprint] {
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

// fingerprintPresenceSet records every finding fingerprint, including waived
// ones, so presence is independent of suppression state.
func fingerprintPresenceSet(findings []auditspec.Finding) map[string]bool {
	set := make(map[string]bool, len(findings))
	for _, finding := range findings {
		if finding.Fingerprint == "" {
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
