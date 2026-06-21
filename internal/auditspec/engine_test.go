package auditspec

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

func codes(findings []Finding) map[string]int {
	counts := map[string]int{}
	for _, finding := range findings {
		counts[finding.Code]++
	}
	return counts
}

func TestBaselineFlagsMissingCSRFAndPublicAPI(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"public"}, CSRF: false, Public: true},
			{ID: "Health", Kind: "api", Method: "GET", Path: "/api/health", CSRF: false, DefaultDeny: true},
			{ID: "Update", Kind: "api", Method: "POST", Path: "/api/status", Guards: []string{"public"}, CSRF: false, Public: true},
			{ID: "Refresh", Kind: "fragment", Method: "GET", Path: "/frag", DefaultDeny: true},
			{ID: "patients.CreatePatient", Kind: "command", Method: "POST", Path: "/patients", CSRF: false, DefaultDeny: true},
			{ID: "patients.GetPatientPage", Kind: "query", Method: "GET", Path: "/patients", CSRF: false, DefaultDeny: true},
		},
		Frontend: securitymanifest.FrontendSurface{},
	}

	got := codes(Evaluate(manifest, Baseline()))
	if got["audit_action_missing_csrf"] != 1 {
		t.Fatalf("expected one missing-CSRF finding, got %d", got["audit_action_missing_csrf"])
	}
	if got["audit_api_public_by_omission"] != 1 {
		t.Fatalf("expected one public-by-omission API finding, got %d", got["audit_api_public_by_omission"])
	}
	if got["audit_api_missing_csrf"] != 1 {
		t.Fatalf("expected one API missing-CSRF finding, got %d", got["audit_api_missing_csrf"])
	}
	if got["audit_command_missing_csrf"] != 1 {
		t.Fatalf("expected one missing-CSRF command finding, got %d", got["audit_command_missing_csrf"])
	}
	if got["audit_guardless_endpoint_page"] != 3 {
		t.Fatalf("expected three guardless fragment/contract findings, got %d", got["audit_guardless_endpoint_page"])
	}
}

func soundLimits(kind string) securitymanifest.RequestLimitPosture {
	return securitymanifest.RequestLimitPosture{
		EndpointKind:           kind,
		RawBodyBytes:           1 << 20,
		CompressedBodyHandling: "raw-bytes-bounded",
		InstalledBeforeParse:   true,
		Phase:                  "before-body-parse-and-csrf",
		Origin:                 "default:gowdk.DefaultRequestBodyLimitBytes",
	}
}

func TestBaselinePassesWhenPostureIsSound(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"auth.required"}, CSRF: true, RequestLimits: soundLimits("action")},
			{ID: "List", Kind: "api", Method: "GET", Path: "/api/list", Guards: []string{"permission:list.read"}, RequestLimits: soundLimits("api")},
			{ID: "Update", Kind: "api", Method: "POST", Path: "/api/status", Guards: []string{"permission:status.write"}, CSRF: true, RequestLimits: soundLimits("api")},
			{ID: "patients.CreatePatient", Kind: "command", Method: "POST", Path: "/patients", Guards: []string{"auth.required"}, CSRF: true, RequestLimits: soundLimits("command")},
			{ID: "patients.GetPatientPage", Kind: "query", Method: "GET", Path: "/patients", Guards: []string{"auth.required"}, RequestLimits: soundLimits("query")},
		},
	}
	findings := Evaluate(manifest, Baseline())
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a sound posture, got %#v", findings)
	}
}

func TestBaselineFlagsMissingAndUnsafeRequestLimits(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			// No raw cap recorded at all -> missing.
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"auth.required"}, CSRF: true},
			// Cap recorded but installed after parse -> phase unsafe.
			{ID: "Ingest", Kind: "api", Method: "POST", Path: "/api/ingest", Guards: []string{"permission:x"}, CSRF: true,
				RequestLimits: securitymanifest.RequestLimitPosture{EndpointKind: "api", RawBodyBytes: 1 << 20, InstalledBeforeParse: false}},
			// Multipart accepted without a multipart cap -> unbounded multipart.
			{ID: "Upload", Kind: "action", Method: "POST", Path: "/upload", Guards: []string{"auth.required"}, CSRF: true,
				RequestLimits: securitymanifest.RequestLimitPosture{EndpointKind: "action", RawBodyBytes: 1 << 20, InstalledBeforeParse: true, MultipartEnabled: true}},
		},
	}
	got := codes(Evaluate(manifest, Baseline()))
	if got["audit_request_limit_missing"] != 1 {
		t.Fatalf("expected one missing-limit finding, got %#v", got)
	}
	if got["audit_request_limit_phase_unsafe"] != 1 {
		t.Fatalf("expected one phase-unsafe finding, got %#v", got)
	}
	if got["audit_request_limit_unbounded_multipart"] != 1 {
		t.Fatalf("expected one unbounded-multipart finding, got %#v", got)
	}
}

func TestRequestLimitsPerEndpointAndKindSplit(t *testing.T) {
	// The same selector family can carry different effective caps per endpoint,
	// and command/query/action/API caps are evaluated independently.
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Small", Kind: "api", Method: "POST", Path: "/api/a", Guards: []string{"permission:x"}, CSRF: true, BodyLimitBytes: 256 << 10, RequestLimits: securitymanifest.RequestLimitPosture{EndpointKind: "api", RawBodyBytes: 256 << 10, InstalledBeforeParse: true}},
			{ID: "Big", Kind: "api", Method: "POST", Path: "/api/b", Guards: []string{"permission:x"}, CSRF: true, BodyLimitBytes: 4 << 20, RequestLimits: securitymanifest.RequestLimitPosture{EndpointKind: "api", RawBodyBytes: 4 << 20, InstalledBeforeParse: true}},
		},
	}
	// A declared policy caps only API endpoints at 1mb; the per-endpoint posture
	// means only the oversized endpoint is flagged.
	policies := ComposeBaseline([]Policy{{
		Name:      "api_tight",
		Source:    "x.audit.gwdk:1",
		Selectors: []Selector{{Raw: "api:*", Kind: SelectorEndpoint}},
		Rules:     []Rule{{Kind: RuleMaxBody, Value: "1mb", Code: "audit_max_body_exceeds_policy"}},
	}})
	if got := codes(Evaluate(manifest, policies)); got["audit_max_body_exceeds_policy"] != 1 {
		t.Fatalf("expected only the oversized API endpoint to be flagged, got %#v", got)
	}
}

func TestBaselineFlagsRolelessContract(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Contracts: []securitymanifest.ContractEntry{
			{Name: "patients.CreatePatient", Kind: "command", DeclarationSource: "contracts/patients.go:12", ExposureSource: "patients.page.gwdk:8"},
			{Name: "patients.GetPatientPage", Kind: "query", Roles: []string{"web"}},
		},
	}
	findings := Evaluate(manifest, Baseline())
	got := codes(findings)
	if got["audit_contract_roleless"] != 1 {
		t.Fatalf("expected exactly one roleless-contract finding, got %d (%#v)", got["audit_contract_roleless"], got)
	}
	if findings[0].Source != "contracts/patients.go:12" {
		t.Fatalf("expected contract finding to use declaration source, got %#v", findings[0])
	}
}

func TestBaselineFlagsUnverifiedGuardEvidence(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Routes: []securitymanifest.RouteEntry{{
			PageID: "admin",
			Route:  "/admin",
			Kind:   "ssr",
			Guards: []string{"auth.required"},
			GuardEvidence: []securitymanifest.GuardEvidence{{
				ID:                 "auth.required",
				BindingStatus:      "unverified-app-owned",
				RuntimeTestFixture: "unverified-app-owned",
			}},
			Source: "admin.page.gwdk:4",
		}},
	}
	findings := Evaluate(manifest, Baseline())
	got := codes(findings)
	if got["audit_guard_unverified"] != 1 {
		t.Fatalf("expected unverified guard finding, got %#v", got)
	}
	if findings[0].Target != "route:/admin#guard:auth.required" || findings[0].Evidence != "inferred-static" || findings[0].Fingerprint == "" {
		t.Fatalf("expected targeted guard metadata, got %#v", findings[0])
	}
}

func TestBaselineFlagsUnsafeObservabilityPosture(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Observability: []securitymanifest.ObservabilityEntry{{
			ID:                         "trace.browser",
			Kind:                       "browser-ingest",
			Path:                       "/_gowdk/traces/browser",
			Mounted:                    true,
			BuildMode:                  "production",
			DevOnly:                    false,
			AccessPolicy:               "public",
			ExportsAbsoluteSourcePaths: true,
			SpanDataLeavesProcess:      true,
		}},
	}
	findings := Evaluate(manifest, Baseline())
	got := codes(findings)
	for _, code := range []string{
		"audit_observability_production_exposed",
		"audit_observability_origin_unchecked",
		"audit_observability_content_type_missing",
		"audit_observability_body_limit_missing",
		"audit_observability_batch_limit_missing",
		"audit_observability_absolute_source",
	} {
		if got[code] != 1 {
			t.Fatalf("expected one %s finding, got %#v", code, got)
		}
	}
	for _, finding := range findings {
		if finding.Fingerprint == "" || finding.Confidence == "" || finding.Evidence != "static-observability-posture" {
			t.Fatalf("observability finding missing triage metadata: %#v", finding)
		}
	}
}

func TestBaselineFlagsFrontendAuditFindings(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Frontend: securitymanifest.FrontendSurface{
			UnguardedRoutes: []securitymanifest.UnguardedRoute{{Route: "/draft", Source: "draft.page.gwdk:4"}},
			BundleSecrets:   []securitymanifest.BundleLeak{{Kind: "unsafe-asset:.env", Source: "card.cmp.gwdk:4"}},
			RawHTMLSinks:    []securitymanifest.RawHTMLSink{{OwnerKind: "page", OwnerID: "home", Field: "{TrustedHTML}", Source: "home.page.gwdk:12"}},
		},
	}
	findings := Evaluate(manifest, Baseline())
	got := codes(findings)
	if got["audit_bundle_secret"] != 1 {
		t.Fatalf("expected one bundle secret finding, got %#v", got)
	}
	if got["audit_client_route_unguarded"] != 1 {
		t.Fatalf("expected one client route finding, got %#v", got)
	}
	if got["audit_raw_html_sink"] != 1 {
		t.Fatalf("expected one raw HTML finding, got %#v", got)
	}
	for _, finding := range findings {
		if finding.Source == "" {
			t.Fatalf("frontend finding should include a source: %#v", finding)
		}
	}
}

func TestPolicyRequireHeaderUsesConfiguredHeaders(t *testing.T) {
	policy := Policy{
		Name:      "headers",
		Source:    "security.audit.gwdk:3",
		Selectors: []Selector{{Raw: "frontend", Kind: SelectorFrontend}},
		Rules:     []Rule{{Kind: RuleRequireHeader, Value: "Content-Security-Policy", Code: "audit_headers_missing"}},
	}
	missing := securitymanifest.SecurityManifest{Frontend: securitymanifest.FrontendSurface{
		ConfiguredHeaders: []securitymanifest.ConfiguredHeader{{Name: "X-Content-Type-Options"}},
	}}
	if got := codes(Evaluate(missing, []Policy{policy})); got["audit_headers_missing"] != 1 {
		t.Fatalf("expected missing header finding, got %#v", got)
	}
	present := securitymanifest.SecurityManifest{Frontend: securitymanifest.FrontendSurface{
		ConfiguredHeaders: []securitymanifest.ConfiguredHeader{{Name: "content-security-policy"}},
	}}
	if findings := Evaluate(present, []Policy{policy}); len(findings) != 0 {
		t.Fatalf("expected configured header to satisfy policy, got %#v", findings)
	}
}

func headerManifest(buildMode string, headers ...securitymanifest.ConfiguredHeader) securitymanifest.SecurityManifest {
	return securitymanifest.SecurityManifest{
		BuildMode: buildMode,
		Frontend:  securitymanifest.FrontendSurface{ConfiguredHeaders: headers},
	}
}

func TestBaselineFlagsWeakSecurityHeaders(t *testing.T) {
	manifest := headerManifest("production",
		securitymanifest.ConfiguredHeader{Name: "Content-Security-Policy", Value: "default-src 'self'; script-src 'unsafe-inline'"},
		securitymanifest.ConfiguredHeader{Name: "Referrer-Policy", Value: "unsafe-url"},
		securitymanifest.ConfiguredHeader{Name: "Strict-Transport-Security", Value: "max-age=600"},
		securitymanifest.ConfiguredHeader{Name: "X-Frame-Options", Value: "DENY"},
		securitymanifest.ConfiguredHeader{Name: "Content-Security-Policy-2", Value: ""}, // ignored, distinct name
	)
	got := codes(Evaluate(manifest, Baseline()))
	for _, code := range []string{
		"audit_header_csp_weak",
		"audit_header_nosniff_missing",
		"audit_header_referrer_weak",
		"audit_header_hsts_weak",
	} {
		if got[code] != 1 {
			t.Fatalf("expected one %s finding, got %#v", code, got)
		}
	}
	for _, finding := range Evaluate(manifest, Baseline()) {
		if strings.HasPrefix(finding.Code, "audit_header_") {
			if finding.Source != "config:Build.SecurityHeaders" || finding.Remediation == "" {
				t.Fatalf("header finding missing source/remediation: %#v", finding)
			}
		}
	}
}

func TestBaselineAcceptsStrongSecurityHeaders(t *testing.T) {
	manifest := headerManifest("production",
		securitymanifest.ConfiguredHeader{Name: "Content-Security-Policy", Value: "default-src 'self'; frame-ancestors 'none'; object-src 'none'"},
		securitymanifest.ConfiguredHeader{Name: "X-Content-Type-Options", Value: "nosniff"},
		securitymanifest.ConfiguredHeader{Name: "Referrer-Policy", Value: "strict-origin-when-cross-origin"},
		securitymanifest.ConfiguredHeader{Name: "Strict-Transport-Security", Value: "max-age=31536000; includeSubDomains"},
		securitymanifest.ConfiguredHeader{Name: "X-Frame-Options", Value: "DENY"},
	)
	for _, finding := range Evaluate(manifest, Baseline()) {
		if strings.HasPrefix(finding.Code, "audit_header_") {
			t.Fatalf("strong headers should not produce a header finding: %#v", finding)
		}
	}
}

func TestBaselineFlagsMissingNosniff(t *testing.T) {
	manifest := headerManifest("development",
		securitymanifest.ConfiguredHeader{Name: "Content-Security-Policy", Value: "default-src 'self'"},
	)
	if got := codes(Evaluate(manifest, Baseline())); got["audit_header_nosniff_missing"] != 1 {
		t.Fatalf("expected missing nosniff finding, got %#v", got)
	}
}

func TestBaselineFlagsConflictingFramePolicy(t *testing.T) {
	manifest := headerManifest("development",
		securitymanifest.ConfiguredHeader{Name: "X-Content-Type-Options", Value: "nosniff"},
		securitymanifest.ConfiguredHeader{Name: "X-Frame-Options", Value: "DENY"},
		securitymanifest.ConfiguredHeader{Name: "Content-Security-Policy", Value: "default-src 'self'; frame-ancestors https://embed.example.com"},
	)
	if got := codes(Evaluate(manifest, Baseline())); got["audit_header_frame_conflict"] != 1 {
		t.Fatalf("expected one frame-conflict finding, got %#v", got)
	}
}

func TestBaselineHSTSAllowedInDevelopmentWithShortMaxAge(t *testing.T) {
	manifest := headerManifest("development",
		securitymanifest.ConfiguredHeader{Name: "X-Content-Type-Options", Value: "nosniff"},
		securitymanifest.ConfiguredHeader{Name: "Strict-Transport-Security", Value: "max-age=600"},
	)
	if got := codes(Evaluate(manifest, Baseline())); got["audit_header_hsts_weak"] != 0 {
		t.Fatalf("short max-age should be tolerated outside production, got %#v", got)
	}
}

func TestDeclaredRequireCSRFResolvesCodeByEndpointKind(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"auth.required"}, CSRF: false},
			{ID: "Update", Kind: "api", Method: "GET", Path: "/api/status", Guards: []string{"auth.required"}, CSRF: false},
			{ID: "patients.CreatePatient", Kind: "command", Method: "POST", Path: "/patients", Guards: []string{"auth.required"}, CSRF: false},
		},
	}
	// A declared require csrf rule leaves Code empty (as the parser now does), so
	// the engine resolves the kind-appropriate code for each matched endpoint.
	policies := []Policy{{
		Name:      "csrf_everywhere",
		Source:    "security.audit.gwdk:1",
		Selectors: []Selector{{Raw: "act:*", Kind: SelectorEndpoint}, {Raw: "api:*", Kind: SelectorEndpoint}, {Raw: "command:*", Kind: SelectorEndpoint}},
		Rules:     []Rule{{Kind: RuleRequireCSRF}},
	}}
	got := codes(Evaluate(manifest, policies))
	if got["audit_action_missing_csrf"] != 1 {
		t.Fatalf("expected one action CSRF finding, got %#v", got)
	}
	if got["audit_command_missing_csrf"] != 1 {
		t.Fatalf("expected one command CSRF finding, got %#v", got)
	}
	if got["audit_api_missing_csrf"] != 1 {
		t.Fatalf("expected one API CSRF finding, got %#v", got)
	}
}

func TestRuleCodeOverrideValidation(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Routes: []securitymanifest.RouteEntry{
			{PageID: "admin", Route: "/admin", Kind: "ssr", Guards: []string{"auth.required"}},
		},
	}
	base := func(code string) []Policy {
		return []Policy{{
			Name:      "guards",
			Source:    "security.audit.gwdk:3",
			Selectors: []Selector{{Raw: "/admin/**", Kind: SelectorRoute}},
			Rules:     []Rule{{Kind: RuleRequireGuard, Value: "role:admin", Code: code, Source: "security.audit.gwdk:4"}},
		}}
	}

	// Valid override within the rule family.
	got := codes(Evaluate(manifest, base("audit_required_guard_missing")))
	if got["audit_required_guard_missing"] != 1 || got["policy_rule_code_incompatible"] != 0 {
		t.Fatalf("valid in-family override should be accepted, got %#v", got)
	}

	// Unrelated code from a different family.
	got = codes(Evaluate(manifest, base("audit_bundle_secret")))
	if got["policy_rule_code_incompatible"] != 1 {
		t.Fatalf("unrelated code should be incompatible, got %#v", got)
	}
	// Neutralized back to the default code so the rule still fires correctly.
	if got["audit_required_guard_missing"] != 1 {
		t.Fatalf("neutralized override should still emit the default-coded finding, got %#v", got)
	}

	// Unknown / unregistered code.
	got = codes(Evaluate(manifest, base("audit_made_up_code")))
	if got["policy_rule_code_unknown"] != 1 {
		t.Fatalf("unknown code should be reported, got %#v", got)
	}

	// No override at all.
	noOverride := []Policy{{
		Name:      "guards",
		Source:    "security.audit.gwdk:3",
		Selectors: []Selector{{Raw: "/admin/**", Kind: SelectorRoute}},
		Rules:     []Rule{{Kind: RuleRequireGuard, Value: "role:admin", Code: "audit_required_guard_missing"}},
	}}
	got = codes(Evaluate(manifest, noOverride))
	if got["policy_rule_code_incompatible"]+got["policy_rule_code_unknown"]+got["policy_rule_code_severity_lowered"] != 0 {
		t.Fatalf("no override should not raise a code-validation finding, got %#v", got)
	}
}

func TestRuleCodeOverrideCannotLowerSeverity(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Refresh", Kind: "fragment", Method: "GET", Path: "/frag", DefaultDeny: true, RequestLimits: soundLimits("fragment")},
		},
	}
	// require_any_guard defaults to an error code; audit_client_route_unguarded is
	// in the same family but only a warning, so the override must be rejected.
	policies := []Policy{{
		Name:      "downgrade",
		Source:    "security.audit.gwdk:1",
		Selectors: []Selector{{Raw: "fragment:*", Kind: SelectorEndpoint}},
		Rules:     []Rule{{Kind: RuleRequireAnyGuard, Code: "audit_client_route_unguarded", Source: "security.audit.gwdk:2"}},
	}}
	findings := Evaluate(manifest, policies)
	got := codes(findings)
	if got["policy_rule_code_severity_lowered"] != 1 {
		t.Fatalf("expected a severity-lowered finding, got %#v", got)
	}
	// The downgraded code must not appear; the rule fires at full severity instead.
	if got["audit_client_route_unguarded"] != 0 || got["audit_guardless_endpoint_page"] != 1 {
		t.Fatalf("severity-lowering override should be neutralized to the default code, got %#v", got)
	}
	for _, finding := range findings {
		if finding.Code == "policy_rule_code_severity_lowered" && finding.Source != "security.audit.gwdk:2" {
			t.Fatalf("validation finding should carry the rule source span, got %#v", finding)
		}
	}
}

func TestFindingsRecordCodeSource(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"public"}, CSRF: false, Public: true, RequestLimits: soundLimits("action")},
		},
	}
	for _, finding := range Evaluate(manifest, Baseline()) {
		if finding.CodeSource != "baseline-default" {
			t.Fatalf("baseline finding should record baseline-default code source, got %#v", finding)
		}
	}
}

func TestEvaluateDeduplicatesFindingsAcrossExtendingPolicies(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Frontend: securitymanifest.FrontendSurface{
			RawHTMLSinks: []securitymanifest.RawHTMLSink{
				{OwnerKind: "page", OwnerID: "home", Field: "{TrustedHTML}", Source: "home.page.gwdk:12"},
			},
		},
	}
	// browser_hardening extends baseline.frontend (which already denies raw HTML)
	// and denies raw HTML again, so the same sink is evaluated three times.
	declared := []Policy{{
		Name:      "browser_hardening",
		Source:    "security.audit.gwdk:3",
		Extends:   []string{"baseline.frontend"},
		Selectors: []Selector{{Raw: "frontend", Kind: SelectorFrontend}},
		Rules:     []Rule{{Kind: RuleDenyRawHTMLSinks, Code: "audit_raw_html_sink"}},
	}}
	if got := codes(Evaluate(manifest, ComposeBaseline(declared)))["audit_raw_html_sink"]; got != 1 {
		t.Fatalf("expected one deduped raw HTML finding, got %d", got)
	}
}

func TestFrontendRawHTMLAllowlistSuppressesBaselineFinding(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Frontend: securitymanifest.FrontendSurface{
			RawHTMLSinks: []securitymanifest.RawHTMLSink{
				{OwnerKind: "page", OwnerID: "home", Field: "TrustedHTML", Source: "home.page.gwdk:12"},
			},
		},
	}
	declared := []Policy{{
		Name:      "browser_hardening",
		Source:    "security.audit.gwdk:3",
		Extends:   []string{"baseline.frontend"},
		Selectors: []Selector{{Raw: "frontend", Kind: SelectorFrontend}},
		Rules:     []Rule{{Kind: RuleAllowRawHTML, Value: "home:TrustedHTML"}},
	}}
	if got := codes(Evaluate(manifest, ComposeBaseline(declared)))["audit_raw_html_sink"]; got != 0 {
		t.Fatalf("expected declared allowlist to suppress baseline raw HTML finding, got %d", got)
	}
}

func rawHTMLSinkManifest(field, source string) securitymanifest.SecurityManifest {
	fingerprint := securitymanifest.RawHTMLFingerprint("page", "home", field, source, 0)
	return securitymanifest.SecurityManifest{
		Frontend: securitymanifest.FrontendSurface{
			RawHTMLSinks: []securitymanifest.RawHTMLSink{
				{OwnerKind: "page", OwnerID: "home", Field: field, Source: source, Ordinal: 0, Fingerprint: fingerprint},
			},
		},
	}
}

func exceptionPolicy(fingerprint string, attrs map[string]string) []Policy {
	return ComposeBaseline([]Policy{{
		Name:      "raw_html_waivers",
		Source:    "security.audit.gwdk:3",
		Extends:   []string{"baseline.frontend"},
		Selectors: []Selector{{Raw: "frontend", Kind: SelectorFrontend}},
		Rules:     []Rule{{Kind: RuleExceptRawHTML, Value: fingerprint, Source: "security.audit.gwdk:4", Attrs: attrs}},
	}})
}

func goodExceptionAttrs() map[string]string {
	return map[string]string{
		"owner":         "team-content",
		"justification": "server-sanitized markdown",
		"expires":       "2999-12-31",
		"sanitizer":     "bluemonday",
	}
}

func TestRawHTMLExceptionExactFingerprintSuppresses(t *testing.T) {
	manifest := rawHTMLSinkManifest("{Body}", "home.page.gwdk:12")
	fingerprint := manifest.Frontend.RawHTMLSinks[0].Fingerprint
	got := codes(Evaluate(manifest, exceptionPolicy(fingerprint, goodExceptionAttrs())))
	if got["audit_raw_html_sink"] != 0 {
		t.Fatalf("an exact active exception should suppress the sink, got %#v", got)
	}
	if got["audit_raw_html_exception_unmatched"]+got["audit_raw_html_exception_expired"]+got["audit_raw_html_exception_malformed"] != 0 {
		t.Fatalf("active exception should not produce a state finding, got %#v", got)
	}
}

func TestRawHTMLExceptionStaleSourceMovementIsUnmatched(t *testing.T) {
	// Exception was pinned to the sink at its old line; the sink has since moved.
	oldFingerprint := securitymanifest.RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:12", 0)
	manifest := rawHTMLSinkManifest("{Body}", "home.page.gwdk:40")
	got := codes(Evaluate(manifest, exceptionPolicy(oldFingerprint, goodExceptionAttrs())))
	if got["audit_raw_html_exception_unmatched"] != 1 {
		t.Fatalf("moved sink should leave the exception unmatched, got %#v", got)
	}
	if got["audit_raw_html_sink"] != 1 {
		t.Fatalf("moved sink should be reported as unallowlisted, got %#v", got)
	}
}

func TestRawHTMLExceptionChangedExpressionIsUnmatched(t *testing.T) {
	oldFingerprint := securitymanifest.RawHTMLFingerprint("page", "home", "{Body}", "home.page.gwdk:12", 0)
	manifest := rawHTMLSinkManifest("{RenderedBody}", "home.page.gwdk:12")
	got := codes(Evaluate(manifest, exceptionPolicy(oldFingerprint, goodExceptionAttrs())))
	if got["audit_raw_html_exception_unmatched"] != 1 || got["audit_raw_html_sink"] != 1 {
		t.Fatalf("changed expression should unmatch the exception and report the sink, got %#v", got)
	}
}

func TestRawHTMLExceptionExpired(t *testing.T) {
	manifest := rawHTMLSinkManifest("{Body}", "home.page.gwdk:12")
	fingerprint := manifest.Frontend.RawHTMLSinks[0].Fingerprint
	attrs := goodExceptionAttrs()
	attrs["expires"] = "2000-01-01"
	got := codes(Evaluate(manifest, exceptionPolicy(fingerprint, attrs)))
	if got["audit_raw_html_exception_expired"] != 1 {
		t.Fatalf("expired exception should be reported, got %#v", got)
	}
	if got["audit_raw_html_sink"] != 1 {
		t.Fatalf("expired exception should no longer suppress the sink, got %#v", got)
	}
}

func TestRawHTMLExceptionMalformedRequiresEvidence(t *testing.T) {
	manifest := rawHTMLSinkManifest("{Body}", "home.page.gwdk:12")
	fingerprint := manifest.Frontend.RawHTMLSinks[0].Fingerprint
	attrs := goodExceptionAttrs()
	delete(attrs, "sanitizer") // missing sanitizer/trusted-type evidence
	findings := Evaluate(manifest, exceptionPolicy(fingerprint, attrs))
	got := codes(findings)
	if got["audit_raw_html_exception_malformed"] != 1 {
		t.Fatalf("missing sanitizer should make the exception malformed, got %#v", got)
	}
	if got["audit_raw_html_sink"] != 1 {
		t.Fatalf("malformed exception must not suppress the sink, got %#v", got)
	}
	for _, finding := range findings {
		if finding.Code == "audit_raw_html_exception_malformed" && finding.Source != "security.audit.gwdk:4" {
			t.Fatalf("exception finding should carry the rule source, got %#v", finding)
		}
	}
}

func TestComposeBaselineLetsDeclaredPolicyOverrideBuiltin(t *testing.T) {
	policies := ComposeBaseline([]Policy{{
		Name:      "baseline.frontend",
		Selectors: []Selector{{Raw: "frontend", Kind: SelectorFrontend}},
		Rules:     []Rule{{Kind: RuleRequireHeader, Value: "Content-Security-Policy", Code: "audit_headers_missing"}},
	}})
	for _, policy := range policies {
		if policy.Name != "baseline.frontend" {
			continue
		}
		if policy.Builtin {
			t.Fatalf("declared override should replace builtin baseline.frontend: %#v", policy)
		}
		if len(policy.Rules) != 1 || policy.Rules[0].Kind != RuleRequireHeader {
			t.Fatalf("unexpected overridden frontend policy: %#v", policy)
		}
		return
	}
	t.Fatalf("baseline.frontend missing from composed policies: %#v", policies)
}

func TestSeverityComesFromRegistry(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"public"}, CSRF: false, Public: true},
		},
	}
	findings := Evaluate(manifest, Baseline())
	if len(findings) == 0 {
		t.Fatal("expected a finding")
	}
	if findings[0].Severity != diagnostics.SeverityError {
		t.Fatalf("severity must resolve from the registry: got %q", findings[0].Severity)
	}
}

func TestDeclaredPolicyExtendsComposes(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Routes: []securitymanifest.RouteEntry{
			{PageID: "admin", Route: "/admin/patients", Kind: "ssr", Guards: []string{"auth.required"}},
		},
	}
	policies := []Policy{
		{Name: "authed", Source: "a.audit.gwdk", Selectors: []Selector{{Raw: "/admin/**", Kind: SelectorRoute}}, Rules: []Rule{{Kind: RuleDenyPublic, Code: "audit_public_not_allowed"}}},
		{Name: "admin", Source: "a.audit.gwdk", Extends: []string{"authed"}, Selectors: []Selector{{Raw: "/admin/**", Kind: SelectorRoute}}, Rules: []Rule{{Kind: RuleRequireGuard, Value: "role:admin", Code: "audit_required_guard_missing"}}},
	}
	got := codes(Evaluate(manifest, policies))
	// "admin" inherits deny_public from "authed" (route is not public, so no
	// finding there) and adds require role:admin (route lacks it -> finding).
	if got["audit_required_guard_missing"] != 1 {
		t.Fatalf("expected one required-guard finding from composed policy, got %#v", got)
	}
}

func TestExtendsCycleIsReported(t *testing.T) {
	policies := []Policy{
		{Name: "a", Extends: []string{"b"}, Source: "x"},
		{Name: "b", Extends: []string{"a"}, Source: "x"},
	}
	got := codes(Evaluate(securitymanifest.SecurityManifest{}, policies))
	if got["policy_extends_cycle"] == 0 {
		t.Fatalf("expected a cycle finding, got %#v", got)
	}
}

func TestUnknownExtendsIsReported(t *testing.T) {
	policies := []Policy{
		{Name: "a", Extends: []string{"missing"}, Source: "x"},
	}
	got := codes(Evaluate(securitymanifest.SecurityManifest{}, policies))
	if got["policy_unknown_extends"] == 0 {
		t.Fatalf("expected an unknown-extends finding, got %#v", got)
	}
}

func TestDuplicatePolicyNameIsReported(t *testing.T) {
	policies := []Policy{
		{Name: "a", Source: "x"},
		{Name: "a", Source: "y"},
	}
	got := codes(Evaluate(securitymanifest.SecurityManifest{}, policies))
	if got["policy_duplicate_name"] == 0 {
		t.Fatalf("expected a duplicate-name finding, got %#v", got)
	}
}

func TestMaxBodyRuleFlagsOversizedLimit(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Upload", Kind: "action", Method: "POST", Path: "/upload", Guards: []string{"auth.required"}, CSRF: true, BodyLimitBytes: 1 << 20},
		},
	}
	policies := []Policy{
		{Name: "tight", Source: "x", Selectors: []Selector{{Raw: "act:*", Kind: SelectorEndpoint}}, Rules: []Rule{{Kind: RuleMaxBody, Value: "256kb", Code: "audit_max_body_exceeds_policy"}}},
	}
	got := codes(Evaluate(manifest, policies))
	if got["audit_max_body_exceeds_policy"] != 1 {
		t.Fatalf("expected one oversized-body finding, got %#v", got)
	}
}

func TestRouteGlobMatching(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/admin/**", "/admin/patients", true},
		{"/admin/**", "/admin", true},
		{"/admin/**", "/admin/a/b/c", true},
		{"/admin/**", "/public", false},
		{"/blog/*", "/blog/post", true},
		{"/blog/*", "/blog/post/comments", false},
		{"/", "/", true},
		{"/dashboard", "/dashboard", true},
		{"/dashboard", "/dash", false},
	}
	for _, tc := range cases {
		if got := matchRouteGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchRouteGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"256kb", 256 * 1024, true},
		{"1mb", 1 << 20, true},
		{"512", 512, true},
		{"2gb", 2 << 30, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseSize(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseSize(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseSelectorClassifies(t *testing.T) {
	cases := []struct {
		raw  string
		want SelectorKind
	}{
		{"/admin/**", SelectorRoute},
		{"act:*", SelectorEndpoint},
		{"api:Health", SelectorEndpoint},
		{"command:*", SelectorEndpoint},
		{"query:*", SelectorEndpoint},
		{"frontend", SelectorFrontend},
		{"nonsense", SelectorUnknown},
	}
	for _, tc := range cases {
		if got := ParseSelector(tc.raw).Kind; got != tc.want {
			t.Errorf("ParseSelector(%q).Kind = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
