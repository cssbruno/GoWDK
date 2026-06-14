package auditspec

// Baseline returns the built-in policy set that gowdk audit applies with zero
// configuration. It encodes the production-readiness gates from
// docs/engineering/security.md and docs/engineering/security-threat-model.md so
// security is enforced by default, not by opt-in. Declared *.audit.gwdk policies
// extend or override these via matching selectors and rules.
//
// Severity is never set here; each rule references a registry code and the
// engine resolves severity from internal/diagnostics.
func Baseline() []Policy {
	return []Policy{
		{
			Name:    "baseline.actions",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "act:*", Kind: SelectorEndpoint},
			},
			Rules: []Rule{
				{Kind: RuleRequireCSRF, Code: "audit_action_missing_csrf"},
				{Kind: RuleRequireAnyGuard, Code: "audit_guardless_endpoint_page"},
			},
		},
		{
			Name:    "baseline.fragments",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "fragment:*", Kind: SelectorEndpoint},
			},
			Rules: []Rule{
				{Kind: RuleRequireAnyGuard, Code: "audit_guardless_endpoint_page"},
			},
		},
		{
			Name:    "baseline.api",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "api:*", Kind: SelectorEndpoint},
			},
			Rules: []Rule{
				{Kind: RuleRequireAnyGuard, Code: "audit_api_public_by_omission"},
			},
		},
		{
			Name:    "baseline.contract_commands",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "command:*", Kind: SelectorEndpoint},
			},
			Rules: []Rule{
				{Kind: RuleRequireCSRF, Code: "audit_command_missing_csrf"},
				{Kind: RuleRequireAnyGuard, Code: "audit_guardless_endpoint_page"},
			},
		},
		{
			Name:    "baseline.contract_queries",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "query:*", Kind: SelectorEndpoint},
			},
			Rules: []Rule{
				{Kind: RuleRequireAnyGuard, Code: "audit_guardless_endpoint_page"},
			},
		},
		{
			Name:    "baseline.frontend",
			Builtin: true,
			Selectors: []Selector{
				{Raw: "frontend", Kind: SelectorFrontend},
			},
			Rules: []Rule{
				{Kind: RuleNoSecretsInBundle, Code: "audit_bundle_secret"},
				{Kind: RuleRequireClientRouteGuards, Code: "audit_client_route_unguarded"},
				{Kind: RuleDenyRawHTMLSinks, Code: "audit_raw_html_sink"},
			},
		},
	}
}
