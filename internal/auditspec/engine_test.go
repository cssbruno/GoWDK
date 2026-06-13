package auditspec

import (
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
			{ID: "Refresh", Kind: "fragment", Method: "GET", Path: "/frag", DefaultDeny: true},
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
	if got["audit_guardless_endpoint_page"] != 1 {
		t.Fatalf("expected one guardless fragment finding, got %d", got["audit_guardless_endpoint_page"])
	}
}

func TestBaselinePassesWhenPostureIsSound(t *testing.T) {
	manifest := securitymanifest.SecurityManifest{
		Endpoints: []securitymanifest.EndpointEntry{
			{ID: "Submit", Kind: "action", Method: "POST", Path: "/signup", Guards: []string{"auth.required"}, CSRF: true},
			{ID: "List", Kind: "api", Method: "GET", Path: "/api/list", Guards: []string{"permission:list.read"}},
		},
	}
	findings := Evaluate(manifest, Baseline())
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a sound posture, got %#v", findings)
	}
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
		{"frontend", SelectorFrontend},
		{"nonsense", SelectorUnknown},
	}
	for _, tc := range cases {
		if got := ParseSelector(tc.raw).Kind; got != tc.want {
			t.Errorf("ParseSelector(%q).Kind = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
