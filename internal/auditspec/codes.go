package auditspec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

// ruleDefaultCode is the diagnostic code a rule family emits when a declared
// policy does not pin one explicitly. require_csrf returns "" because the engine
// resolves an endpoint-kind-appropriate code per match.
func ruleDefaultCode(kind RuleKind) string {
	switch kind {
	case RuleRequireCSRF:
		return ""
	case RuleRequireAnyGuard:
		return "audit_guardless_endpoint_page"
	case RuleRequireGuard:
		return "audit_required_guard_missing"
	case RuleDenyPublic:
		return "audit_public_not_allowed"
	case RuleMaxBody:
		return "audit_max_body_exceeds_policy"
	case RuleRequireHeader:
		return "audit_headers_missing"
	case RuleNoSecretsInBundle:
		return "audit_bundle_secret"
	case RuleDenyRawHTMLSinks:
		return "audit_raw_html_sink"
	case RuleDenyRolelessContract:
		return "audit_contract_roleless"
	case RuleRequireClientRouteGuards:
		return "audit_client_route_unguarded"
	case RuleRequireVerifiedGuards:
		return "audit_guard_unverified"
	default:
		return ""
	}
}

// ruleAllowedCodes is the set of diagnostic codes a rule family may legally pin
// with `as <code>`. It is deliberately narrow: a security rule may only relabel
// itself within its own family, so an explicit code can never quietly turn an
// access-control finding into an unrelated, lower-severity, or misleading one.
// A nil result means the rule takes no code override at all.
func ruleAllowedCodes(kind RuleKind) []string {
	switch kind {
	case RuleRequireCSRF:
		return []string{"audit_action_missing_csrf", "audit_api_missing_csrf", "audit_command_missing_csrf"}
	case RuleRequireAnyGuard:
		// Same access-control family; includes the lower-severity client-route code
		// so a downgrade attempt is caught by the severity floor rather than allowed.
		return []string{"audit_guardless_endpoint_page", "audit_api_public_by_omission", "audit_required_guard_missing", "audit_client_route_unguarded"}
	case RuleRequireGuard:
		return []string{"audit_required_guard_missing", "audit_guardless_endpoint_page", "audit_api_public_by_omission"}
	case RuleDenyPublic:
		return []string{"audit_public_not_allowed"}
	case RuleMaxBody:
		return []string{"audit_max_body_exceeds_policy"}
	case RuleRequireHeader:
		return []string{"audit_headers_missing", "audit_headers_runtime_missing"}
	case RuleNoSecretsInBundle:
		return []string{"audit_bundle_secret"}
	case RuleDenyRawHTMLSinks:
		return []string{"audit_raw_html_sink"}
	case RuleDenyRolelessContract:
		return []string{"audit_contract_roleless"}
	case RuleRequireClientRouteGuards:
		return []string{"audit_client_route_unguarded"}
	case RuleRequireVerifiedGuards:
		return []string{"audit_guard_unverified"}
	default:
		return nil
	}
}

// ruleMinSeverity is the lowest severity a rule's effective code may carry. A
// pinned code may raise severity but never drop below this floor.
func ruleMinSeverity(kind RuleKind) diagnostics.Severity {
	switch kind {
	case RuleMaxBody, RuleRequireHeader, RuleDenyRawHTMLSinks, RuleRequireClientRouteGuards, RuleRequireVerifiedGuards:
		return diagnostics.SeverityWarning
	default:
		return diagnostics.SeverityError
	}
}

// validatePolicyRuleCodes checks explicit `as <code>` overrides on declared
// policies. Each invalid override produces a policy-resolution finding with a
// source span and is neutralized back to the rule's default code so it cannot
// weaken severity, change category, or mislead remediation downstream.
func validatePolicyRuleCodes(policies []Policy) ([]Policy, []Finding) {
	var findings []Finding
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		if policy.Builtin || len(policy.Rules) == 0 {
			out = append(out, policy)
			continue
		}
		rules := make([]Rule, len(policy.Rules))
		for index, rule := range policy.Rules {
			if rule.Code != "" {
				if reason, code, ok := ruleCodeViolation(rule); ok {
					findings = append(findings, ruleCodeFinding(policy, rule, code, reason))
					rule.Code = ruleDefaultCode(rule.Kind)
				}
			}
			rules[index] = rule
		}
		policy.Rules = rules
		out = append(out, policy)
	}
	return out, findings
}

// ruleCodeViolation reports the first compatibility problem with a pinned code.
func ruleCodeViolation(rule Rule) (reason string, code string, violated bool) {
	allowed := ruleAllowedCodes(rule.Kind)
	if len(allowed) == 0 {
		return fmt.Sprintf("rule %s does not support an explicit diagnostic code", rule.Kind),
			"policy_rule_code_incompatible", true
	}
	if _, ok := diagnostics.Lookup(rule.Code); !ok {
		return fmt.Sprintf("rule %s pins unknown diagnostic code %q", rule.Kind, rule.Code),
			"policy_rule_code_unknown", true
	}
	if !containsCode(allowed, rule.Code) {
		return fmt.Sprintf("rule %s cannot use diagnostic code %q; allowed codes are %s", rule.Kind, rule.Code, strings.Join(sortedCopy(allowed), ", ")),
			"policy_rule_code_incompatible", true
	}
	minSeverity := ruleMinSeverity(rule.Kind)
	if severityRank(severityFor(rule.Code)) > severityRank(minSeverity) {
		return fmt.Sprintf("rule %s code %q has severity %q, below the rule minimum %q", rule.Kind, rule.Code, severityFor(rule.Code), minSeverity),
			"policy_rule_code_severity_lowered", true
	}
	return "", "", false
}

func ruleCodeFinding(policy Policy, rule Rule, code, reason string) Finding {
	source := rule.Source
	if source == "" {
		source = policy.Source
	}
	return Finding{
		Code:        code,
		Severity:    severityFor(code),
		CodeSource:  "policy-resolution",
		Target:      "policy:" + policy.Name,
		Policy:      policy.Name,
		Rule:        string(rule.Kind),
		Message:     reason,
		Source:      source,
		Remediation: "Pin a diagnostic code in the rule's own family that does not lower its severity, or omit the `as <code>` override.",
	}
}

// codeSourceFor describes where a finding's effective code and severity came
// from so human and JSON output can make the provenance clear.
func codeSourceFor(policy Policy, rule Rule) string {
	if policy.Builtin {
		return "baseline-default"
	}
	def := ruleDefaultCode(rule.Kind)
	switch {
	case rule.Code == "" || rule.Code == def:
		return "rule-default"
	case rule.Kind == RuleRequireCSRF && containsCode(ruleAllowedCodes(RuleRequireCSRF), rule.Code):
		// require_csrf has no single default; a kind-resolved code is the default.
		return "rule-default"
	default:
		return "policy-override"
	}
}

func containsCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
