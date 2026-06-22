package auditspec

import (
	"fmt"
	"strings"
	"time"
)

// Waiver is one declared `waive` rule: an explicit, attributable, expiring
// decision to suppress a specific finding by diagnostic code and target. A
// waiver only suppresses when it carries an owner, justification, and unexpired
// expiry, and when any pinned policy/posture digest still matches the current
// build. Invalid or stale waivers are reported instead of silently doing nothing.
type Waiver struct {
	Code          string
	Target        string
	Owner         string
	Justification string
	Expires       string
	Ticket        string
	PolicyDigest  string
	PostureDigest string
	Policy        string
	Source        string
}

// WaiverContext carries the current policy and posture digests so a waiver that
// pins a digest can be invalidated when the policy set or posture drifts. A zero
// Now falls back to the package clock.
type WaiverContext struct {
	Now           time.Time
	PolicyDigest  string
	PostureDigest string
}

func policyHasWaiver(policy Policy) bool {
	for _, rule := range policy.Rules {
		if rule.Kind == RuleWaive {
			return true
		}
	}
	return false
}

// collectWaivers gathers the declared waivers from every resolved policy,
// de-duplicating a waiver that reaches several policies through extends.
func collectWaivers(policies []Policy) []Waiver {
	var waivers []Waiver
	seen := map[string]bool{}
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			if rule.Kind != RuleWaive {
				continue
			}
			waiver := Waiver{
				Code:          strings.TrimSpace(rule.Value),
				Target:        strings.TrimSpace(rule.Attrs["target"]),
				Owner:         strings.TrimSpace(rule.Attrs["owner"]),
				Justification: strings.TrimSpace(rule.Attrs["justification"]),
				Expires:       strings.TrimSpace(rule.Attrs["expires"]),
				Ticket:        strings.TrimSpace(rule.Attrs["ticket"]),
				PolicyDigest:  strings.TrimSpace(rule.Attrs["policy_digest"]),
				PostureDigest: strings.TrimSpace(rule.Attrs["posture_digest"]),
				Policy:        policy.Name,
				Source:        rule.Source,
			}
			key := waiver.Code + "\x00" + waiver.Target + "\x00" + waiver.Source + "\x00" + waiver.Expires
			if seen[key] {
				continue
			}
			seen[key] = true
			waivers = append(waivers, waiver)
		}
	}
	return waivers
}

// applyWaivers suppresses each finding matched by a valid waiver (recording the
// suppression metadata on the finding rather than dropping it) and returns the
// modified findings plus a finding for every waiver that is malformed, expired,
// digest-mismatched, or matched nothing. Suppressed findings keep their original
// code and severity but are excluded from the error/warning counts by Summarize.
func applyWaivers(findings []Finding, waivers []Waiver, ctx WaiverContext) ([]Finding, []Finding) {
	if len(waivers) == 0 {
		return findings, nil
	}
	now := ctx.Now
	if now.IsZero() {
		now = timeNow()
	}

	used := make([]bool, len(waivers))
	for fi := range findings {
		if findings[fi].Suppression != nil || isPolicyResolutionCode(findings[fi].Code) {
			continue
		}
		for wi := range waivers {
			if !waiverMatches(waivers[wi], findings[fi]) {
				continue
			}
			if state, _ := classifyWaiver(waivers[wi], now, ctx); state != "valid" {
				continue
			}
			findings[fi].Suppression = &Suppression{
				Owner:         waivers[wi].Owner,
				Justification: waivers[wi].Justification,
				Expires:       waivers[wi].Expires,
				Ticket:        waivers[wi].Ticket,
				DigestScope:   waiverDigestScope(waivers[wi]),
			}
			used[wi] = true
			break
		}
	}

	var extra []Finding
	for wi := range waivers {
		state, reason := classifyWaiver(waivers[wi], now, ctx)
		switch state {
		case "malformed":
			extra = append(extra, waiverFinding(waivers[wi], "audit_waiver_malformed", reason,
				"Provide code, target, owner, justification, and expires (YYYY-MM-DD); ticket and policy/posture digest pins are optional."))
		case "expired":
			extra = append(extra, waiverFinding(waivers[wi], "audit_waiver_expired", reason,
				"Re-validate the finding and renew the waiver with a future expiry, or fix the underlying issue and remove the waiver."))
		case "digest-mismatch":
			extra = append(extra, waiverFinding(waivers[wi], "audit_waiver_digest_mismatch", reason,
				"Re-validate the finding against the current policy/posture and update the pinned digest, or remove the stale pin."))
		case "valid":
			if !used[wi] {
				extra = append(extra, waiverFinding(waivers[wi], "audit_waiver_unmatched",
					fmt.Sprintf("waiver for %s on target %q matches no current finding", waivers[wi].Code, waivers[wi].Target),
					"Update the waiver target/code to a current finding (gowdk audit prints them), or remove the stale waiver."))
			}
		}
	}
	return findings, extra
}

func waiverMatches(waiver Waiver, finding Finding) bool {
	if waiver.Code != finding.Code {
		return false
	}
	target := strings.TrimSpace(waiver.Target)
	if target == "" {
		return false
	}
	return target == finding.Target || matchGlob(target, finding.Target)
}

// classifyWaiver resolves a waiver to "valid", "malformed", "expired", or
// "digest-mismatch". Required fields are code, target, owner, justification, and
// expires; ticket and the policy/posture digest pins are optional.
func classifyWaiver(waiver Waiver, now time.Time, ctx WaiverContext) (state, reason string) {
	var missing []string
	if waiver.Code == "" {
		missing = append(missing, "code")
	}
	if waiver.Target == "" {
		missing = append(missing, "target")
	}
	if waiver.Owner == "" {
		missing = append(missing, "owner")
	}
	if waiver.Justification == "" {
		missing = append(missing, "justification")
	}
	if waiver.Expires == "" {
		missing = append(missing, "expires")
	}
	if len(missing) > 0 {
		return "malformed", fmt.Sprintf("waiver is missing required field(s): %s", strings.Join(missing, ", "))
	}
	expiry, ok := parseExceptionExpiry(waiver.Expires)
	if !ok {
		return "malformed", fmt.Sprintf("waiver expiry %q is not a valid date (use YYYY-MM-DD)", waiver.Expires)
	}
	if !expiry.After(now) {
		return "expired", fmt.Sprintf("waiver for %s expired on %s", waiver.Code, waiver.Expires)
	}
	if waiver.PolicyDigest != "" && ctx.PolicyDigest != "" && waiver.PolicyDigest != ctx.PolicyDigest {
		return "digest-mismatch", fmt.Sprintf("waiver pins policy digest %s but the current policy digest is %s", waiver.PolicyDigest, ctx.PolicyDigest)
	}
	if waiver.PostureDigest != "" && ctx.PostureDigest != "" && waiver.PostureDigest != ctx.PostureDigest {
		return "digest-mismatch", fmt.Sprintf("waiver pins posture digest %s but the current posture digest is %s", waiver.PostureDigest, ctx.PostureDigest)
	}
	return "valid", ""
}

func waiverDigestScope(waiver Waiver) string {
	var parts []string
	if waiver.PolicyDigest != "" {
		parts = append(parts, "policy="+waiver.PolicyDigest)
	}
	if waiver.PostureDigest != "" {
		parts = append(parts, "posture="+waiver.PostureDigest)
	}
	if len(parts) == 0 {
		return "unpinned"
	}
	return strings.Join(parts, " ")
}

func waiverFinding(waiver Waiver, code, reason, remediation string) Finding {
	target := strings.TrimSpace(waiver.Target)
	if target == "" {
		target = waiver.Code
	}
	return Finding{
		Code:        code,
		Severity:    severityFor(code),
		CodeSource:  "policy-default",
		Target:      "waiver:" + waiver.Code + "#" + target,
		Policy:      waiver.Policy,
		Rule:        string(RuleWaive),
		Message:     reason,
		Source:      waiver.Source,
		Remediation: remediation,
	}
}

func isPolicyResolutionCode(code string) bool {
	return strings.HasPrefix(code, "policy_") || strings.HasPrefix(code, "audit_waiver_")
}
