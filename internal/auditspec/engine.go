package auditspec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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
	frontendRawHTMLAllowlist := rawHTMLAllowlist(resolved)

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

	for _, contract := range manifest.Contracts {
		for _, policy := range resolved {
			if !policy.matchesContract(contract) {
				continue
			}
			matchedAnything[policy.Name] = true
			findings = append(findings, evalContract(contract, policy)...)
		}
	}

	for _, entry := range manifest.Observability {
		for _, policy := range resolved {
			if !policy.matchesObservability(entry) {
				continue
			}
			matchedAnything[policy.Name] = true
			findings = append(findings, evalObservability(entry, policy)...)
		}
	}

	for _, policy := range resolved {
		if !policy.hasFrontendSelector() {
			continue
		}
		matchedAnything[policy.Name] = true
		findings = append(findings, evalFrontend(manifest.Frontend, policy, frontendRawHTMLAllowlist, manifest.BuildMode)...)
	}

	findings = append(findings, unmatchedSelectorFindings(resolved, matchedAnything)...)
	return EnrichFindings(dedupeFindings(findings))
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

func (policy Policy) matchesContract(contract securitymanifest.ContractEntry) bool {
	for _, selector := range policy.Selectors {
		if selector.Kind != SelectorContract {
			continue
		}
		glob := contractSelectorGlob(selector.Raw)
		if matchGlob(glob, contract.Name) || matchGlob(glob, contract.Kind) {
			return true
		}
	}
	return false
}

func (policy Policy) matchesObservability(entry securitymanifest.ObservabilityEntry) bool {
	for _, selector := range policy.Selectors {
		if selector.Kind != SelectorObservability {
			continue
		}
		glob := strings.TrimPrefix(selector.Raw, "observability")
		glob = strings.TrimPrefix(glob, ":")
		if glob == "" {
			glob = "*"
		}
		if matchGlob(glob, entry.ID) || matchGlob(glob, entry.Kind) {
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
			csrfRule := rule
			if csrfRule.Code == "" {
				csrfRule.Code = csrfCodeForKind(endpoint.Kind)
			}
			if policy.Builtin && endpoint.Kind == "api" && csrfRule.Code == "audit_api_missing_csrf" && !gwdkir.HTTPMethodRequiresCSRF(endpoint.Method) {
				continue
			}
			if !endpoint.CSRF {
				findings = append(findings, finding(csrfRule, policy, endpointTarget(endpoint), endpoint.Source,
					fmt.Sprintf("%s endpoint %s does not enforce CSRF", endpoint.Kind, endpoint.ID),
					"Remove Build.CSRF.Disabled, or override the matching baseline policy in a *.audit.gwdk file."))
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
		case RuleRequireRequestLimits:
			findings = append(findings, evalRequestLimits(rule, policy, endpoint)...)
		case RuleRequireVerifiedGuards:
			findings = append(findings, evalGuardEvidence(rule, policy, endpointTarget(endpoint), endpoint.Source, endpoint.GuardEvidence)...)
		}
	}
	return findings
}

// evalRequestLimits evaluates the effective request-limit posture: a positive
// raw body cap, the cap installed before the body is parsed (so it precedes CSRF
// token parsing and handler execution), and a multipart cap when multipart
// bodies are accepted. Phase and multipart are only judged against an explicitly
// recorded RequestLimits posture so an under-specified manifest is not
// false-flagged for an ordering it never claimed.
func evalRequestLimits(rule Rule, policy Policy, endpoint securitymanifest.EndpointEntry) []Finding {
	limits := endpoint.RequestLimits
	rawBytes := limits.RawBodyBytes
	if rawBytes == 0 {
		rawBytes = endpoint.BodyLimitBytes
	}
	if rawBytes <= 0 {
		return []Finding{finding(ruleWithCode(rule, "audit_request_limit_missing"), policy, endpointTarget(endpoint), endpoint.Source,
			fmt.Sprintf("%s endpoint %s does not declare a positive raw request body limit", endpoint.Kind, endpoint.ID),
			"Set a positive Build.BodyLimits cap for this endpoint kind so the request body is bounded before parsing.")}
	}
	if limits.RawBodyBytes <= 0 {
		return nil
	}
	var findings []Finding
	if !limits.InstalledBeforeParse {
		findings = append(findings, finding(ruleWithCode(rule, "audit_request_limit_phase_unsafe"), policy, endpointTarget(endpoint), endpoint.Source,
			fmt.Sprintf("%s endpoint %s installs its body limit after the body is parsed", endpoint.Kind, endpoint.ID),
			"Install the request body limit before CSRF token parsing and handler execution."))
	}
	if limits.MultipartEnabled && limits.MultipartMaxBytes <= 0 {
		findings = append(findings, finding(ruleWithCode(rule, "audit_request_limit_unbounded_multipart"), policy, endpointTarget(endpoint), endpoint.Source,
			fmt.Sprintf("%s endpoint %s accepts multipart bodies without a multipart byte limit", endpoint.Kind, endpoint.ID),
			"Declare a multipart/upload byte limit for endpoints that accept multipart bodies."))
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
		case RuleRequireVerifiedGuards:
			findings = append(findings, evalGuardEvidence(rule, policy, routeTarget(route), route.Source, route.GuardEvidence)...)
		}
	}
	return findings
}

func evalGuardEvidence(rule Rule, policy Policy, target string, source string, guards []securitymanifest.GuardEvidence) []Finding {
	var findings []Finding
	for _, guard := range guards {
		if guard.BindingStatus != "unverified-app-owned" && guard.RuntimeTestFixture != "unverified-app-owned" {
			continue
		}
		findings = append(findings, finding(rule, policy, target+"#guard:"+guard.ID, source,
			fmt.Sprintf("guard %s is app-owned and has no runtime fixture evidence", guard.ID),
			"Provide an app-owned audit fixture for this guard or use a GOWDK-native guard where applicable."))
	}
	return findings
}

func evalContract(contract securitymanifest.ContractEntry, policy Policy) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		if rule.Kind != RuleDenyRolelessContract {
			continue
		}
		if len(contract.Roles) == 0 {
			findings = append(findings, finding(rule, policy, contractTarget(contract), contractFindingSource(contract),
				fmt.Sprintf("%s contract %s is web-exposed but declares no roles; the data-layer gate denies every web caller and the endpoint is unreachable", contract.Kind, contract.Name),
				"Declare the roles permitted to execute the contract at registration, or RoleAny to expose it to every role intentionally."))
		}
	}
	return findings
}

func evalObservability(entry securitymanifest.ObservabilityEntry, policy Policy) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		if rule.Kind != RuleCheckObservability {
			continue
		}
		target := "observability:" + entry.ID
		if entry.Mounted && (!entry.DevOnly || strings.EqualFold(entry.BuildMode, "production")) {
			typedRule := ruleWithCode(rule, "audit_observability_production_exposed")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("observability endpoint %s is mounted outside the debug-only lane", entry.Path),
				"Disable observability in production output or mount the trace viewer behind an app-owned access gate."))
		}
		if entry.Mounted && !observabilityAccessPolicyChecksOrigin(entry) {
			typedRule := ruleWithCode(rule, "audit_observability_origin_unchecked")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("observability endpoint %s does not declare an origin or loopback access policy", entry.Path),
				"Keep generated trace endpoints loopback-only or add an explicit origin policy before exposing them."))
		}
		if entry.Kind == "browser-ingest" && entry.ContentTypeRequired == "" {
			typedRule := ruleWithCode(rule, "audit_observability_content_type_missing")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("browser trace ingestion endpoint %s does not require a JSON content type", entry.Path),
				"Require application/json for browser trace ingestion."))
		}
		if entry.Kind == "browser-ingest" && entry.BodyLimitBytes <= 0 {
			typedRule := ruleWithCode(rule, "audit_observability_body_limit_missing")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("browser trace ingestion endpoint %s does not declare a request body limit", entry.Path),
				"Declare and enforce a bounded request body limit for trace ingestion."))
		}
		if entry.Kind == "browser-ingest" && entry.BatchLimit <= 0 {
			typedRule := ruleWithCode(rule, "audit_observability_batch_limit_missing")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("browser trace ingestion endpoint %s does not declare a batch limit", entry.Path),
				"Limit the number of spans accepted in one browser ingestion request."))
		}
		if entry.ExportsAbsoluteSourcePaths {
			typedRule := ruleWithCode(rule, "audit_observability_absolute_source")
			findings = append(findings, finding(typedRule, policy, target, "",
				fmt.Sprintf("observability endpoint %s can export absolute source paths", entry.Path),
				"Normalize trace source references before exporting span data."))
		}
	}
	return findings
}

func ruleWithCode(rule Rule, code string) Rule {
	rule.Code = code
	return rule
}

func observabilityAccessPolicyChecksOrigin(entry securitymanifest.ObservabilityEntry) bool {
	if strings.EqualFold(entry.AccessPolicy, "loopback-only") {
		return true
	}
	for _, origin := range entry.AllowedOrigins {
		if strings.TrimSpace(origin) != "" {
			return true
		}
	}
	return false
}

func evalFrontend(surface securitymanifest.FrontendSurface, policy Policy, rawHTMLAllowlist map[string]bool, buildMode string) []Finding {
	var findings []Finding
	for _, rule := range policy.Rules {
		switch rule.Kind {
		case RuleCheckSecurityHeaders:
			findings = append(findings, evalSecurityHeaders(surface, policy, buildMode)...)
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
			findings = append(findings, evalRawHTMLSinks(surface, policy, rule, rawHTMLAllowlist)...)
		case RuleAllowRawHTML:
			// Handled by evalRawHTMLSinks against the resolved frontend allowlist.
		}
	}
	return findings
}

// evalRawHTMLSinks reports raw-HTML sinks that are not allowlisted by any
// RuleAllowRawHTML rule on any resolved frontend policy.
func evalRawHTMLSinks(surface securitymanifest.FrontendSurface, policy Policy, rule Rule, allow map[string]bool) []Finding {
	if len(surface.RawHTMLSinks) == 0 {
		return nil
	}
	var findings []Finding
	for _, sink := range surface.RawHTMLSinks {
		if rawHTMLSinkAllowed(allow, sink) {
			continue
		}
		findings = append(findings, finding(rule, policy, "frontend", sink.Source,
			fmt.Sprintf("raw HTML sink %s:%s is not allowlisted", sink.OwnerID, sink.Field),
			"Render escaped output, or add the sink to the policy raw-HTML allowlist."))
	}
	return findings
}

func rawHTMLAllowlist(policies []Policy) map[string]bool {
	allow := map[string]bool{}
	for _, policy := range policies {
		if !policy.hasFrontendSelector() {
			continue
		}
		for _, rule := range policy.Rules {
			if rule.Kind != RuleAllowRawHTML {
				continue
			}
			addRawHTMLAllowlistValue(allow, rule.Value)
		}
	}
	if len(allow) == 0 {
		return nil
	}
	return allow
}

func addRawHTMLAllowlistValue(allow map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	allow[value] = true
	if owner, field, ok := strings.Cut(value, ":"); ok {
		owner = strings.TrimSpace(owner)
		field = strings.Trim(strings.TrimSpace(field), "{}")
		if owner != "" && field != "" {
			allow[owner+":"+field] = true
		}
	}
}

func rawHTMLSinkAllowed(allow map[string]bool, sink securitymanifest.RawHTMLSink) bool {
	if len(allow) == 0 {
		return false
	}
	if allow[strings.TrimSpace(sink.Source)] {
		return true
	}
	field := strings.TrimSpace(sink.Field)
	if allow[sink.OwnerID+":"+field] {
		return true
	}
	normalizedField := strings.Trim(field, "{}")
	return normalizedField != field && allow[sink.OwnerID+":"+normalizedField]
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
	switch kind {
	case "api":
		return "audit_api_missing_csrf"
	case "command":
		return "audit_command_missing_csrf"
	default:
		return "audit_action_missing_csrf"
	}
}

func endpointTarget(endpoint securitymanifest.EndpointEntry) string {
	return endpoint.Kind + ":" + endpoint.ID
}

func routeTarget(route securitymanifest.RouteEntry) string {
	return "route:" + route.Route
}

func contractTarget(contract securitymanifest.ContractEntry) string {
	return "contract:" + contract.Name
}

func contractFindingSource(contract securitymanifest.ContractEntry) string {
	if contract.DeclarationSource != "" {
		return contract.DeclarationSource
	}
	return contract.ExposureSource
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
	case raw == "observability" || strings.HasPrefix(raw, "observability:"):
		return Selector{Raw: raw, Kind: SelectorObservability}
	case raw == "contract" || strings.HasPrefix(raw, "contract:"):
		return Selector{Raw: raw, Kind: SelectorContract}
	case strings.HasPrefix(raw, "/"):
		return Selector{Raw: raw, Kind: SelectorRoute}
	default:
		if _, _, ok := splitEndpointSelector(raw); ok {
			return Selector{Raw: raw, Kind: SelectorEndpoint}
		}
		return Selector{Raw: raw, Kind: SelectorUnknown}
	}
}

// contractSelectorGlob extracts the glob from a contract selector. "contract"
// and "contract:" match every contract; "contract:<glob>" matches by contract
// name or kind.
func contractSelectorGlob(raw string) string {
	glob := strings.TrimPrefix(raw, "contract")
	glob = strings.TrimPrefix(glob, ":")
	if glob == "" {
		return "*"
	}
	return glob
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
