package auditspec

import (
	"fmt"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

// timeNow is the clock used to evaluate raw-HTML exception expiry. It is a
// package variable so tests can pin a deterministic moment.
var timeNow = time.Now

// rawHTMLException is one declared fingerprinted exception suppressing a single
// raw-HTML sink.
type rawHTMLException struct {
	fingerprint   string
	owner         string
	justification string
	expires       string
	sanitizer     string
	policy        string
	source        string
}

// rawHTMLSinkFingerprints indexes the fingerprints of the recorded sinks.
func rawHTMLSinkFingerprints(sinks []securitymanifest.RawHTMLSink) map[string]bool {
	if len(sinks) == 0 {
		return nil
	}
	out := make(map[string]bool, len(sinks))
	for _, sink := range sinks {
		if sink.Fingerprint != "" {
			out[sink.Fingerprint] = true
		}
	}
	return out
}

// collectRawHTMLExceptions gathers fingerprinted exceptions from every resolved
// frontend policy, de-duplicating an exception that reaches several policies
// through extends.
func collectRawHTMLExceptions(policies []Policy) []rawHTMLException {
	var exceptions []rawHTMLException
	seen := map[string]bool{}
	for _, policy := range policies {
		if !policy.hasFrontendSelector() {
			continue
		}
		for _, rule := range policy.Rules {
			if rule.Kind != RuleExceptRawHTML {
				continue
			}
			exception := rawHTMLException{
				fingerprint:   strings.TrimSpace(rule.Value),
				owner:         strings.TrimSpace(rule.Attrs["owner"]),
				justification: strings.TrimSpace(rule.Attrs["justification"]),
				expires:       strings.TrimSpace(rule.Attrs["expires"]),
				sanitizer:     strings.TrimSpace(rule.Attrs["sanitizer"]),
				policy:        policy.Name,
				source:        rule.Source,
			}
			key := exception.fingerprint + "\x00" + exception.source + "\x00" + exception.expires
			if seen[key] {
				continue
			}
			seen[key] = true
			exceptions = append(exceptions, exception)
		}
	}
	return exceptions
}

// classifyRawHTMLExceptions resolves each exception to active (suppresses its
// sink), expired, unmatched, or malformed, and returns the active fingerprint
// set plus a finding for every exception that does not actively suppress a sink.
func classifyRawHTMLExceptions(exceptions []rawHTMLException, sinkFingerprints map[string]bool) (map[string]bool, []Finding) {
	if len(exceptions) == 0 {
		return nil, nil
	}
	active := map[string]bool{}
	var findings []Finding
	now := timeNow()
	for _, exception := range exceptions {
		state, reason := classifyRawHTMLException(exception, sinkFingerprints, now)
		switch state {
		case "active":
			active[exception.fingerprint] = true
		case "malformed":
			findings = append(findings, rawHTMLExceptionFinding(exception, "audit_raw_html_exception_malformed", reason,
				"Provide owner, justification, expires (YYYY-MM-DD), and sanitizer/trusted-type, and pin the exact sink fingerprint."))
		case "expired":
			findings = append(findings, rawHTMLExceptionFinding(exception, "audit_raw_html_exception_expired", reason,
				"Re-validate the sink and renew the exception with a future expiry, or remove it and escape the output."))
		case "unmatched":
			findings = append(findings, rawHTMLExceptionFinding(exception, "audit_raw_html_exception_unmatched", reason,
				"Update the fingerprint to the current sink (gowdk audit prints it), or remove the stale exception."))
		}
	}
	return active, findings
}

func classifyRawHTMLException(exception rawHTMLException, sinkFingerprints map[string]bool, now time.Time) (state, reason string) {
	var missing []string
	if exception.fingerprint == "" {
		missing = append(missing, "fingerprint")
	}
	if exception.owner == "" {
		missing = append(missing, "owner")
	}
	if exception.justification == "" {
		missing = append(missing, "justification")
	}
	if exception.expires == "" {
		missing = append(missing, "expires")
	}
	if exception.sanitizer == "" {
		missing = append(missing, "sanitizer")
	}
	if len(missing) > 0 {
		return "malformed", fmt.Sprintf("raw-HTML exception is missing required field(s): %s", strings.Join(missing, ", "))
	}
	expiry, ok := parseExceptionExpiry(exception.expires)
	if !ok {
		return "malformed", fmt.Sprintf("raw-HTML exception expiry %q is not a valid date (use YYYY-MM-DD)", exception.expires)
	}
	if !expiry.After(now) {
		return "expired", fmt.Sprintf("raw-HTML exception for fingerprint %s expired on %s", exception.fingerprint, exception.expires)
	}
	if !sinkFingerprints[exception.fingerprint] {
		return "unmatched", fmt.Sprintf("raw-HTML exception fingerprint %s matches no current sink", exception.fingerprint)
	}
	return "active", ""
}

func parseExceptionExpiry(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func rawHTMLExceptionFinding(exception rawHTMLException, code, reason, remediation string) Finding {
	return Finding{
		Code:        code,
		Severity:    severityFor(code),
		CodeSource:  "policy-default",
		Target:      "raw-html-exception:" + exception.fingerprint,
		Policy:      exception.policy,
		Rule:        string(RuleExceptRawHTML),
		Message:     reason,
		Source:      exception.source,
		Remediation: remediation,
	}
}
