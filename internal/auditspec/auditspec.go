// Package auditspec is the policy model and evaluation engine for gowdk audit.
//
// A Policy is a named, composable set of Rules applied to targets (routes,
// endpoints, contracts, or the frontend surface) selected by Selectors. The
// built-in Baseline encodes the production-readiness gates from
// docs/engineering/security.md; declared *.audit.gwdk policies extend or
// override it. Evaluate matches the policies against a securitymanifest posture
// and returns registry-coded Findings; it never decides severity — that comes
// only from internal/diagnostics.
package auditspec

import "github.com/cssbruno/gowdk/internal/diagnostics"

// SelectorKind classifies a policy target selector.
type SelectorKind string

const (
	SelectorRoute         SelectorKind = "route"
	SelectorEndpoint      SelectorKind = "endpoint"
	SelectorContract      SelectorKind = "contract"
	SelectorObservability SelectorKind = "observability"
	SelectorFrontend      SelectorKind = "frontend"
	SelectorUnknown       SelectorKind = "unknown"
)

// RuleKind classifies one policy rule.
type RuleKind string

const (
	// RuleRequireCSRF requires a matched endpoint to enforce CSRF.
	RuleRequireCSRF RuleKind = "require_csrf"
	// RuleRequireAnyGuard requires a matched target to state access (any guard,
	// including guard public) rather than be denied by omission.
	RuleRequireAnyGuard RuleKind = "require_any_guard"
	// RuleRequireGuard requires a specific guard ID (for example role:admin).
	RuleRequireGuard RuleKind = "require_guard"
	// RuleDenyPublic forbids guard public on a matched target.
	RuleDenyPublic RuleKind = "deny_public"
	// RuleMaxBody caps a matched endpoint's request body limit.
	RuleMaxBody RuleKind = "max_body"
	// RuleRequireRequestLimits requires a matched endpoint to declare an effective
	// request-limit posture: a positive raw body cap installed before the body is
	// parsed, and a multipart cap when multipart bodies are accepted.
	RuleRequireRequestLimits RuleKind = "require_request_limits"
	// RuleRequireHeader requires the app to be configured to emit a response
	// header.
	RuleRequireHeader RuleKind = "require_header"
	// RuleCheckSecurityHeaders audits the semantic strength of configured
	// security response headers (CSP, nosniff, Referrer-Policy, HSTS, framing).
	RuleCheckSecurityHeaders RuleKind = "check_security_headers"
	// RuleCheckCORS audits the generated cross-origin policy for risky
	// combinations such as a wildcard origin (optionally with credentials).
	RuleCheckCORS RuleKind = "check_cors"
	// RuleRequireClientRouteGuards reports client-visible routes that rely on
	// default-deny because the source declared no guard.
	RuleRequireClientRouteGuards RuleKind = "require_client_route_guards"
	// RuleNoSecretsInBundle forbids secret-shaped values in embedded output.
	RuleNoSecretsInBundle RuleKind = "no_secrets_in_bundle"
	// RuleDenyRawHTMLSinks reports every raw-HTML sink not allowlisted by a
	// RuleAllowRawHTML rule in any resolved frontend policy.
	RuleDenyRawHTMLSinks RuleKind = "deny_raw_html_sinks"
	// RuleAllowRawHTML allowlists one raw-HTML sink (source:field); every sink
	// not allowlisted is reported. This is the legacy coarse allowlist; prefer
	// RuleExceptRawHTML for an exact, justified, expiring exception.
	RuleAllowRawHTML RuleKind = "allow_raw_html"
	// RuleExceptRawHTML suppresses exactly one raw-HTML sink by its fingerprint,
	// and only when the exception carries an owner, justification, unexpired
	// expiry, and sanitizer/trusted-type contract.
	RuleExceptRawHTML RuleKind = "except_raw_html"
	// RuleDenyRolelessContract reports a web-exposed command or query contract
	// that declares no roles, so the data-layer authorization gate has no role to
	// admit. The contract must declare at least one role (or RoleAny to be
	// intentionally public).
	RuleDenyRolelessContract RuleKind = "deny_roleless_contract"
	// RuleRequireVerifiedGuards reports guards whose implementation is app-owned
	// and not backed by audit fixture evidence.
	RuleRequireVerifiedGuards RuleKind = "require_verified_guards"
	// RuleCheckObservability reports unsafe generated trace endpoint posture.
	RuleCheckObservability RuleKind = "check_observability"
	// RuleWaive suppresses one finding (by diagnostic code and target) when the
	// waiver carries an owner, justification, and unexpired expiry, and any pinned
	// policy/posture digest still matches. A waived finding is recorded with its
	// suppression metadata instead of blocking, so a suppression is always an
	// explicit, attributable, expiring decision rather than a silent override.
	RuleWaive RuleKind = "waive"
)

// Selector targets a set of routes, endpoints, or the frontend surface.
type Selector struct {
	Raw  string
	Kind SelectorKind
}

// Rule is one policy constraint. Code is the diagnostic code emitted when the
// rule is violated; Value carries the rule argument (a guard ID, header name,
// byte size, or allowlist entry) when the rule kind needs one. Source records
// where a declared rule originated so code-override validation can point at it.
type Rule struct {
	Kind   RuleKind
	Value  string
	Code   string
	Source string
	Attrs  map[string]string
}

// Policy is a named, composable set of rules applied to selected targets.
type Policy struct {
	Name      string
	Extends   []string
	Selectors []Selector
	Rules     []Rule
	Source    string
	Builtin   bool
}

// Finding is one policy violation or policy-resolution error.
type Finding struct {
	Code        string               `json:"code"`
	Severity    diagnostics.Severity `json:"severity"`
	CodeSource  string               `json:"codeSource,omitempty"`
	Fingerprint string               `json:"fingerprint,omitempty"`
	Target      string               `json:"target,omitempty"`
	Policy      string               `json:"policy,omitempty"`
	Rule        string               `json:"rule,omitempty"`
	Confidence  string               `json:"confidence,omitempty"`
	Evidence    string               `json:"evidence,omitempty"`
	CWE         []string             `json:"cwe,omitempty"`
	OWASP       []string             `json:"owasp,omitempty"`
	Suppression *Suppression         `json:"suppression,omitempty"`
	Message     string               `json:"message"`
	Source      string               `json:"source,omitempty"`
	Remediation string               `json:"remediation,omitempty"`
}

// Suppression is reserved for explicit waiver records once the audit DSL grows
// waiver syntax. Keeping the shape in JSON now lets downstream tooling preserve
// the field without inventing an incompatible suppression contract later.
type Suppression struct {
	Owner         string `json:"owner,omitempty"`
	Justification string `json:"justification,omitempty"`
	Expires       string `json:"expires,omitempty"`
	Ticket        string `json:"ticket,omitempty"`
	DigestScope   string `json:"digestScope,omitempty"`
}

// Summary counts findings by severity. Waived findings (suppressed by an
// explicit waiver) are counted only under Waived so a justified, unexpired
// suppression does not block, while the suppression stays recorded in the report.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
	Waived   int `json:"waived"`
}

// Summarize counts findings by their registry severity, excluding waived
// findings from the error/warning/info counts.
func Summarize(findings []Finding) Summary {
	var summary Summary
	for _, finding := range findings {
		if finding.Suppression != nil {
			summary.Waived++
			continue
		}
		switch finding.Severity {
		case diagnostics.SeverityError:
			summary.Errors++
		case diagnostics.SeverityWarning:
			summary.Warnings++
		default:
			summary.Info++
		}
	}
	return summary
}

// Status reports "fail" when any error finding exists, "warning" when only
// warnings exist, and "ok" otherwise.
func Status(summary Summary) string {
	switch {
	case summary.Errors > 0:
		return "fail"
	case summary.Warnings > 0:
		return "warning"
	default:
		return "ok"
	}
}

// severityFor resolves a code's severity from the diagnostic registry, defaulting
// to error for unknown codes so a missing registration is loud, not silent.
func severityFor(code string) diagnostics.Severity {
	if severity, ok := diagnostics.DefaultSeverity(code); ok {
		return severity
	}
	return diagnostics.SeverityError
}
