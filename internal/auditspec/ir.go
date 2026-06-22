package auditspec

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// PoliciesFromIR converts parsed *.audit.gwdk specs into engine policies.
func PoliciesFromIR(specs []gwdkir.AuditSpec) []Policy {
	var policies []Policy
	for _, spec := range specs {
		for _, policy := range spec.Policies {
			out := Policy{
				Name:    policy.Name,
				Extends: append([]string(nil), policy.Extends...),
				Source:  sourceRef(spec.Source, policy.Span),
			}
			for _, apply := range policy.Applies {
				out.Selectors = append(out.Selectors, ParseSelector(apply.Selector))
			}
			for _, rule := range policy.Rules {
				var attrs map[string]string
				if len(rule.Attrs) > 0 {
					attrs = make(map[string]string, len(rule.Attrs))
					for key, value := range rule.Attrs {
						attrs[key] = value
					}
				}
				out.Rules = append(out.Rules, Rule{
					Kind:   RuleKind(rule.Kind),
					Value:  rule.Value,
					Code:   rule.Code,
					Source: sourceRef(spec.Source, rule.Span),
					Attrs:  attrs,
				})
			}
			policies = append(policies, out)
		}
	}
	return policies
}

// ComposeBaseline returns the built-in baseline with declared policies appended.
//
// Built-in baseline policies are monotonic: a declared policy can extend or
// tighten the baseline (with `extends`) or suppress a specific finding (with an
// explicit `waive`), but it can no longer silently replace a baseline policy by
// reusing its name. A same-name declared policy is kept here and reported by
// resolve as policy_baseline_override; its rules are not applied, so the baseline
// can never be weakened by omission. This keeps the fail-closed production story
// honest: removing a built-in error requires an attributable, expiring waiver.
func ComposeBaseline(declared []Policy) []Policy {
	out := Baseline()
	return append(out, declared...)
}

func sourceRef(file string, span source.SourceSpan) string {
	if file == "" {
		return ""
	}
	if span.Start.Line > 0 {
		return fmt.Sprintf("%s:%d", file, span.Start.Line)
	}
	return file
}
