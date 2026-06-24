package securitymanifest

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestGuardEvidenceStateClassification(t *testing.T) {
	cases := map[string]EvidenceState{
		"public":        EvidenceNotApplicable,
		"native-rbac":   EvidenceVerifiedStatic,
		"auth-required": EvidenceVerifiedStatic,
		"custom":        EvidenceUnverifiedAppOwned,
		"app-something": EvidenceUnverifiedAppOwned,
	}
	for kind, want := range cases {
		if got := guardEvidenceState(kind); got != want {
			t.Fatalf("guardEvidenceState(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestObligationsClassifyStaticAndAppOwned(t *testing.T) {
	manifest := SecurityManifest{
		Endpoints: []EndpointEntry{{
			ID:     "Submit",
			Kind:   "action",
			Method: "POST",
			CSRF:   true,
			GuardEvidence: []GuardEvidence{
				{ID: "role:admin", Kind: "native-rbac", Evidence: EvidenceVerifiedStatic},
				{ID: "team.member", Kind: "custom", Evidence: EvidenceUnverifiedAppOwned},
			},
		}},
		Frontend: FrontendSurface{
			ConfiguredHeaders: []ConfiguredHeader{{Name: "Content-Security-Policy", Value: "default-src 'self'"}},
			RawHTMLSinks:      []RawHTMLSink{{Field: "post.body", Fingerprint: "abc"}},
		},
	}

	byID := map[string]ObligationEntry{}
	for _, obligation := range obligations(gowdk.Config{}, manifest) {
		if !obligation.Evidence.Valid() {
			t.Fatalf("obligation %q has invalid evidence state %q", obligation.ID, obligation.Evidence)
		}
		byID[obligation.ID] = obligation
	}

	verifiedStatic := []string{"request.body-limit", "request.csrf", "response.security-headers", "render.raw-html", "access.native-guards"}
	for _, id := range verifiedStatic {
		if byID[id].Evidence != EvidenceVerifiedStatic {
			t.Fatalf("expected %q verified-static, got %#v", id, byID[id])
		}
		if byID[id].Owner != ownerGOWDKNative {
			t.Fatalf("expected %q owned by gowdk-native, got %q", id, byID[id].Owner)
		}
	}

	if byID["access.app-guards"].Evidence != EvidenceUnverifiedAppOwned {
		t.Fatalf("expected custom guard obligation unverified-app-owned, got %#v", byID["access.app-guards"])
	}

	for _, id := range []string{"auth.authentication", "session.management", "authz.resource", "authz.domain"} {
		obligation, ok := byID[id]
		if !ok {
			t.Fatalf("missing app-owned obligation %q", id)
		}
		if obligation.Evidence != EvidenceUnverifiedAppOwned || obligation.Owner != ownerAppOwned {
			t.Fatalf("expected %q unverified app-owned, got %#v", id, obligation)
		}
	}
	if !strings.Contains(sessionManagementDetail(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("auth", gowdk.FeatureAuth)}}), "signed-cookie mode") {
		t.Fatalf("expected auth addon session detail to name signed-cookie mode")
	}
}

func TestObligationsMarkMissingHeadersAndReadOnlyCSRFNotApplicable(t *testing.T) {
	manifest := SecurityManifest{
		Endpoints: []EndpointEntry{{ID: "List", Kind: "query", Method: "GET"}},
	}
	byID := map[string]ObligationEntry{}
	for _, obligation := range obligations(gowdk.Config{}, manifest) {
		byID[obligation.ID] = obligation
	}
	if byID["response.security-headers"].Evidence != EvidenceNotApplicable {
		t.Fatalf("expected not-applicable headers obligation, got %#v", byID["response.security-headers"])
	}
	if byID["request.csrf"].Evidence != EvidenceNotApplicable {
		t.Fatalf("expected not-applicable csrf obligation for read-only endpoints, got %#v", byID["request.csrf"])
	}
}

func TestObligationsEmptyWithoutWebSurface(t *testing.T) {
	if got := obligations(gowdk.Config{}, SecurityManifest{}); got != nil {
		t.Fatalf("expected no obligations without a web surface, got %#v", got)
	}
}

func TestBuildPopulatesGuardEvidenceState(t *testing.T) {
	if state := guardEvidenceFor(gowdk.Config{}, "role:admin").Evidence; state != EvidenceVerifiedStatic {
		t.Fatalf("native rbac guard evidence = %q, want verified-static", state)
	}
	if state := guardEvidenceFor(gowdk.Config{}, "team.lead").Evidence; state != EvidenceUnverifiedAppOwned {
		t.Fatalf("custom guard evidence = %q, want unverified-app-owned", state)
	}
	if state := guardEvidenceFor(gowdk.Config{}, PublicGuardID).Evidence; state != EvidenceNotApplicable {
		t.Fatalf("public guard evidence = %q, want not-applicable", state)
	}
}
