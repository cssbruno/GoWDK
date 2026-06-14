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
	SelectorRoute    SelectorKind = "route"
	SelectorEndpoint SelectorKind = "endpoint"
	SelectorFrontend SelectorKind = "frontend"
	SelectorUnknown  SelectorKind = "unknown"
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
	// RuleRequireHeader requires the app to be configured to emit a response
	// header.
	RuleRequireHeader RuleKind = "require_header"
	// RuleRequireClientRouteGuards reports client-visible routes that rely on
	// default-deny because the source declared no guard.
	RuleRequireClientRouteGuards RuleKind = "require_client_route_guards"
	// RuleNoSecretsInBundle forbids secret-shaped values in embedded output.
	RuleNoSecretsInBundle RuleKind = "no_secrets_in_bundle"
	// RuleDenyRawHTMLSinks reports every raw-HTML sink not allowlisted by a
	// RuleAllowRawHTML rule in any resolved frontend policy.
	RuleDenyRawHTMLSinks RuleKind = "deny_raw_html_sinks"
	// RuleAllowRawHTML allowlists one raw-HTML sink (source:field); every sink
	// not allowlisted is reported.
	RuleAllowRawHTML RuleKind = "allow_raw_html"
)

// Selector targets a set of routes, endpoints, or the frontend surface.
type Selector struct {
	Raw  string
	Kind SelectorKind
}

// Rule is one policy constraint. Code is the diagnostic code emitted when the
// rule is violated; Value carries the rule argument (a guard ID, header name,
// byte size, or allowlist entry) when the rule kind needs one.
type Rule struct {
	Kind  RuleKind
	Value string
	Code  string
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
	Target      string               `json:"target,omitempty"`
	Policy      string               `json:"policy,omitempty"`
	Rule        string               `json:"rule,omitempty"`
	Message     string               `json:"message"`
	Source      string               `json:"source,omitempty"`
	Remediation string               `json:"remediation,omitempty"`
}

// Summary counts findings by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

// Summarize counts findings by their registry severity.
func Summarize(findings []Finding) Summary {
	var summary Summary
	for _, finding := range findings {
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
