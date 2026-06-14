package auditspec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

// endpointKindForSelector maps selector shorthands to manifest endpoint kinds.
var endpointKindForSelector = map[string]string{
	"act":      "action",
	"action":   "action",
	"api":      "api",
	"fragment": "fragment",
	"command":  "command",
	"query":    "query",
}

// Evaluate matches policies against the posture manifest and returns findings.
// It first reports policy-resolution problems (cycles, unknown extends), then
// the per-target rule violations. Findings are returned in a stable order.
func Evaluate(manifest securitymanifest.SecurityManifest, policies []Policy) []Finding {
	resolved, resolutionFindings := resolve(policies)
	findings := append([]Finding(nil), resolutionFindings...)

	matchedAnything := map[string]bool{}

	for _, endpoint := range manifest.Endpoints {
		for _, policy := range resolved {
			if !policy.matchesEndpoint(endpoint) {
				continue
			}
			matchedAnything[policy.Name] = true
			findings = append(findings, evalEndpoint(endpoint, policy)...)
		}
	}

	for _, route := range manifest.Routes {
		for _, policy := range resolved {
			if !policy.matchesRoute(route) {
				continue
			}
			matchedAnything[policy.Name] = true
			findings = append(findings, evalRoute(route, policy)...)
		}
	}

	for _, policy := range resolved {
		if !policy.hasFrontendSelector() {
			continue
		}
		matchedAnything[policy.Name] = true
		findings = append(findings, evalFrontend(manifest.Frontend, policy)...)
	}

	findings = append(findings, unmatchedSelectorFindings(resolved, matchedAnything)...)
	return dedupeFindings(findings)
}

// dedupeFindings drops findings that are identical except for the policy that
// raised them, so a policy that extends a baseline policy does not re-report the
// same sink, route, or endpoint twice. The first occurrence wins, which keeps
// the baseline attribution because ComposeBaseline lists baseline policies first.
func dedupeFindings(findings []Finding) []Finding {
	seen := make(map[string]bool, len(findings))
	out := findings[:0]
	for _, finding := range findings {
		key := finding.Code + "\x00" + finding.Target + "\x00" + finding.Source + "\x00" + finding.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

// resolve expands extends so each policy carries its full rule set, and reports
// cycles, unknown parents, duplicate names, and unknown selectors.
func resolve(policies []Policy) ([]Policy, []Finding) {
	var findings []Finding
	byName := map[string]Policy{}
	for _, policy := range policies {
		if _, exists := byName[policy.Name]; exists {
			findings = append(findings, Finding{
				Code:     "policy_duplicate_name",
				Severity: severityFor("policy_duplicate_name"),
				Policy:   policy.Name,
				Source:   policy.Source,
				Message:  fmt.Sprintf("policy %q is declared more than once", policy.Name),
			})
			continue
		}
		byName[policy.Name] = policy
	}

	for _, policy := range policies {
		for _, selector := range policy.Selectors {
			if selector.Kind == SelectorUnknown {
				findings = append(findings, Finding{
					Code:     "policy_unknown_selector",
					Severity: severityFor("policy_unknown_selector"),
					Policy:   policy.Name,
					Source:   policy.Source,
					Message:  fmt.Sprintf("policy %q uses an unrecognized selector %q", policy.Name, selector.Raw),
				})
			}
		}
	}

	resolved := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		rules, ok := flattenRules(policy.Name, byName, map[string]bool{}, &findings)
		if !ok {
			continue
		}
		policy.Rules = rules
		resolved = append(resolved, policy)
	}
	return resolved, findings
}

// flattenRules returns the rules of name plus all transitively extended rules.
// Parent rules come first so a child can override them by appearing later.
func flattenRules(name string, byName map[string]Policy, visiting map[string]bool, findings *[]Finding) ([]Rule, bool) {
	policy, ok := byName[name]
	if !ok {
		return nil, false
	}
	if visiting[name] {
		*findings = append(*findings, Finding{
			Code:     "policy_extends_cycle",
			Severity: severityFor("policy_extends_cycle"),
			Policy:   name,
			Source:   policy.Source,
			Message:  fmt.Sprintf("policy %q forms an extends cycle", name),
		})
		return nil, false
	}
	visiting[name] = true
	defer delete(visiting, name)

	var rules []Rule
	for _, parent := range policy.Extends {
		parentRules, ok := flattenRules(parent, byName, visiting, findings)
		if !ok {
			if _, exists := byName[parent]; !exists {
				*findings = append(*findings, Finding{
					Code:     "policy_unknown_extends",
					Severity: severityFor("policy_unknown_extends"),
					Policy:   policy.Name,
					Source:   policy.Source,
					Message:  fmt.Sprintf("policy %q extends undefined policy %q", policy.Name, parent),
				})
			}
			return nil, false
		}
		rules = append(rules, parentRules...)
	}
	rules = append(rules, policy.Rules...)
	return rules, true
}

func unmatchedSelectorFindings(policies []Policy, matched map[string]bool) []Finding {
	var findings []Finding
	seen := map[string]bool{}
	for _, policy := range policies {
		if policy.Builtin || matched[policy.Name] || seen[policy.Name] {
			continue
		}
		if len(policy.Selectors) == 0 {
			continue
		}
		seen[policy.Name] = true
		findings = append(findings, Finding{
			Code:     "policy_selector_matched_nothing",
			Severity: severityFor("policy_selector_matched_nothing"),
			Policy:   policy.Name,
			Source:   policy.Source,
			Message:  fmt.Sprintf("policy %q matched no routes or endpoints", policy.Name),
		})
	}
	return findings
}

func (policy Policy) matchesEndpoint(endpoint securitymanifest.EndpointEntry) bool {
	for _, selector := range policy.Selectors {
		switch selector.Kind {
		case SelectorEndpoint:
			kind, glob, ok := splitEndpointSelector(selector.Raw)
			if !ok || kind != endpoint.Kind {
				continue
			}
			if matchGlob(glob, endpoint.ID) || matchGlob(glob, endpoint.Path) {
				return true
			}
		case SelectorRoute:
			if matchRouteGlob(selector.Raw, endpoint.Path) {
				return true
			}
		}
	}
	return false
}

func (policy Policy) matchesRoute(route securitymanifest.RouteEntry) bool {
	for _, selector := range policy.Selectors {
		if selector.Kind == SelectorRoute && matchRouteGlob(selector.Raw, route.Route) {
			return true
		}
	}
	return false
}

func (policy Policy) hasFrontendSelector() bool {
	for _, selector := range policy.Selectors {
		if selector.Kind == SelectorFrontend {
			return true
		}
	}
	return false
}

func evalEndpoint(endpoint securitymanifest.EndpointEntry, policy Policy) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		switch rule.Kind {
		case RuleRequireCSRF:
			if !endpoint.CSRF {
				csrfRule := rule
				if csrfRule.Code == "" {
					csrfRule.Code = csrfCodeForKind(endpoint.Kind)
				}
				findings = append(findings, finding(csrfRule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s does not enforce CSRF", endpoint.Kind, endpoint.ID),
					"Enable Build.CSRF.Enabled, or override the matching baseline policy in a *.audit.gwdk file."))
			}
		case RuleRequireAnyGuard:
			if endpoint.DefaultDeny {
				findings = append(findings, finding(rule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s declares no guard and is denied by omission", endpoint.Kind, endpoint.ID),
					"Add a guard to the page that declares this endpoint."))
			}
		case RuleRequireGuard:
			if !containsGuard(endpoint.Guards, rule.Value) {
				findings = append(findings, finding(rule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s does not declare required guard %q", endpoint.Kind, endpoint.ID, rule.Value),
					fmt.Sprintf("Add guard %s to the page that declares this endpoint.", rule.Value)))
			}
		case RuleDenyPublic:
			if endpoint.Public {
				findings = append(findings, finding(rule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s is public but policy denies public access", endpoint.Kind, endpoint.ID),
					"Replace guard public with a protective guard, or narrow the policy selector."))
			}
		case RuleMaxBody:
			limit, ok := parseSize(rule.Value)
			if ok && endpoint.BodyLimitBytes > limit {
				findings = append(findings, finding(rule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s body limit %d exceeds policy maximum %d", endpoint.Kind, endpoint.ID, endpoint.BodyLimitBytes, limit),
					"Lower Build.BodyLimits, or raise the policy max_body if intentional."))
			}
		}
	}
	return findings
}

func evalRoute(route securitymanifest.RouteEntry, policy Policy) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		switch rule.Kind {
		case RuleRequireGuard:
			if !containsGuard(route.Guards, rule.Value) {
				findings = append(findings, finding(rule, policy, routeTarget(route), route.Source,
					fmt.Sprintf("route %s does not declare required guard %q", route.Route, rule.Value),
					fmt.Sprintf("Add guard %s to %s.", rule.Value, route.PageID)))
			}
		case RuleRequireAnyGuard:
			if route.DefaultDeny {
				findings = append(findings, finding(rule, policy, routeTarget(route), route.Source,
					fmt.Sprintf("route %s declares no guard and is denied by omission", route.Route),
					fmt.Sprintf("State access on %s with guard public or a protective guard.", route.PageID)))
			}
		case RuleDenyPublic:
			if route.Public {
				findings = append(findings, finding(rule, policy, routeTarget(route), route.Source,
					fmt.Sprintf("route %s is public but policy denies public access", route.Route),
					"Replace guard public with a protective guard, or narrow the policy selector."))
			}
		}
	}
	return findings
}

func evalFrontend(surface securitymanifest.FrontendSurface, policy Policy) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		switch rule.Kind {
		case RuleNoSecretsInBundle:
			for _, leak := range surface.BundleSecrets {
				findings = append(findings, finding(rule, policy, "frontend", leak.Source,
					fmt.Sprintf("embedded output carries a secret-shaped value (%s)", leak.Kind),
					"Move the secret to a runtime environment variable, or exclude the file from embedded output."))
			}
		case RuleRequireHeader:
			if !containsHeader(surface.ConfiguredHeaders, rule.Value) {
				findings = append(findings, finding(rule, policy, "frontend", policy.Source,
					fmt.Sprintf("generated app does not declare required response header %q", rule.Value),
					"Enable Build.SecurityHeaders and configure the header."))
			}
		case RuleRequireClientRouteGuards:
			for _, route := range surface.UnguardedRoutes {
				findings = append(findings, finding(rule, policy, "route:"+route.Route, route.Source,
					fmt.Sprintf("client route %s is guardless and relies on generated default-deny handling", route.Route),
					"Declare guard public for an intentionally public page, or add a protective guard."))
			}
		case RuleDenyRawHTMLSinks:
			findings = append(findings, evalRawHTMLSinks(surface, policy, rule)...)
		case RuleAllowRawHTML:
			// Handled by evalRawHTMLSinks against the full allowlist below.
		}
	}
	return findings
}

// evalRawHTMLSinks reports raw-HTML sinks that are not allowlisted by any
// RuleAllowRawHTML rule on the matched frontend policy.
func evalRawHTMLSinks(surface securitymanifest.FrontendSurface, policy Policy, rule Rule) []Finding {
	if len(surface.RawHTMLSinks) == 0 {
		return nil
	}
	allow := map[string]bool{}
	for _, rule := range policy.Rules {
		if rule.Kind == RuleAllowRawHTML {
			allow[rule.Value] = true
		}
	}
	var findings []Finding
	for _, sink := range surface.RawHTMLSinks {
		key := sink.Source
		if allow[key] || allow[sink.OwnerID+":"+sink.Field] {
			continue
		}
		findings = append(findings, finding(rule, policy, "frontend", sink.Source,
			fmt.Sprintf("raw HTML sink %s:%s is not allowlisted", sink.OwnerID, sink.Field),
			"Render escaped output, or add the sink to the policy raw-HTML allowlist."))
	}
	return findings
}

func finding(rule Rule, policy Policy, target, source, message, remediation string) Finding {
	return Finding{
		Code:        rule.Code,
		Severity:    severityFor(rule.Code),
		Target:      target,
		Policy:      policy.Name,
		Rule:        string(rule.Kind),
		Message:     message,
		Source:      source,
		Remediation: remediation,
	}
}

// csrfCodeForKind resolves the diagnostic code for a CSRF requirement when a
// declared rule did not pin one, so a command endpoint reports the command code
// rather than the action code.
func csrfCodeForKind(kind string) string {
	if kind == "command" {
		return "audit_command_missing_csrf"
	}
	return "audit_action_missing_csrf"
}

func endpointTarget(endpoint securitymanifest.EndpointEntry) string {
	return endpoint.Kind + ":" + endpoint.ID
}

func routeTarget(route securitymanifest.RouteEntry) string {
	return "route:" + route.Route
}

func containsGuard(guards []string, want string) bool {
	for _, guard := range guards {
		if guard == want {
			return true
		}
	}
	return false
}

func containsHeader(headers []securitymanifest.ConfiguredHeader, want string) bool {
	want = strings.TrimSpace(want)
	for _, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header.Name), want) {
			return true
		}
	}
	return false
}

// ParseSelector classifies a raw selector string.
func ParseSelector(raw string) Selector {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "frontend":
		return Selector{Raw: raw, Kind: SelectorFrontend}
	case strings.HasPrefix(raw, "/"):
		return Selector{Raw: raw, Kind: SelectorRoute}
	default:
		if _, _, ok := splitEndpointSelector(raw); ok {
			return Selector{Raw: raw, Kind: SelectorEndpoint}
		}
		return Selector{Raw: raw, Kind: SelectorUnknown}
	}
}

func splitEndpointSelector(raw string) (kind, glob string, ok bool) {
	colon := strings.IndexByte(raw, ':')
	if colon <= 0 {
		return "", "", false
	}
	prefix := raw[:colon]
	mapped, known := endpointKindForSelector[prefix]
	if !known {
		return "", "", false
	}
	glob = raw[colon+1:]
	if glob == "" {
		glob = "*"
	}
	return mapped, glob, true
}

// matchGlob matches a simple glob (supporting a trailing or standalone *)
// against a single token.
func matchGlob(pattern, value string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}

// matchRouteGlob matches a route glob against a path. ** matches zero or more
// trailing segments; * matches exactly one segment; other segments match
// literally.
func matchRouteGlob(pattern, path string) bool {
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")
	return matchSegments(patternSegments, pathSegments)
}

func matchSegments(pattern, path []string) bool {
	for index, segment := range pattern {
		if segment == "**" {
			return true
		}
		if index >= len(path) {
			return false
		}
		if segment == "*" {
			continue
		}
		if segment != path[index] {
			return false
		}
	}
	return len(pattern) == len(path)
}

// parseSize parses a byte size such as "256kb", "1mb", or a bare byte count.
func parseSize(value string) (int64, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0, false
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(value, "kb"):
		multiplier, value = 1<<10, strings.TrimSuffix(value, "kb")
	case strings.HasSuffix(value, "mb"):
		multiplier, value = 1<<20, strings.TrimSuffix(value, "mb")
	case strings.HasSuffix(value, "gb"):
		multiplier, value = 1<<30, strings.TrimSuffix(value, "gb")
	case strings.HasSuffix(value, "b"):
		value = strings.TrimSuffix(value, "b")
	}
	value = strings.TrimSpace(value)
	var number int64
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
		number = number*10 + int64(char-'0')
	}
	return number * multiplier, true
}

// SortFindings orders findings deterministically by severity, code, then target.
func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if severityRank(left.Severity) != severityRank(right.Severity) {
			return severityRank(left.Severity) < severityRank(right.Severity)
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Target < right.Target
	})
}

func severityRank(severity diagnostics.Severity) int {
	switch severity {
	case diagnostics.SeverityError:
		return 0
	case diagnostics.SeverityWarning:
		return 1
	default:
		return 2
	}
}
