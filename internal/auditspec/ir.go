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
// A declared policy with the same name as a built-in baseline policy replaces
// that built-in policy so projects can intentionally override a baseline slice.
func ComposeBaseline(declared []Policy) []Policy {
	out := Baseline()
	byName := map[string]int{}
	for index, policy := range out {
		byName[policy.Name] = index
	}
	for _, policy := range declared {
		if index, ok := byName[policy.Name]; ok && out[index].Builtin {
			out[index] = policy
			continue
		}
		out = append(out, policy)
	}
	return out
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
