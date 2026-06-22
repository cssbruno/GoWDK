package securitymanifest

import (
	"net/http"
	"strings"

	"github.com/cssbruno/gowdk"
)

// EvidenceState classifies the kind of proof behind a security posture claim or
// audit finding. It is the shared vocabulary used by gowdk-security.json and the
// gowdk audit report so a human or CI can tell a statically proven fact from an
// app-owned obligation gowdk cannot verify.
type EvidenceState string

const (
	// EvidenceVerifiedStatic marks a fact the compiler proves from generated
	// output or the IR: CSRF wiring, body-limit installation, native guard
	// resolution, configured response headers, raw-HTML sink inventory.
	EvidenceVerifiedStatic EvidenceState = "verified-static"
	// EvidenceVerifiedRuntime marks a fact a generated runtime test exercised
	// against a real generated app (gowdk audit --run).
	EvidenceVerifiedRuntime EvidenceState = "verified-runtime"
	// EvidenceDeclared marks a control the project declared but gowdk has not
	// independently verified.
	EvidenceDeclared EvidenceState = "declared"
	// EvidenceUnverifiedAppOwned marks a control whose decision logic is owned by
	// the application: gowdk generates the call site but cannot prove correctness.
	EvidenceUnverifiedAppOwned EvidenceState = "unverified-app-owned"
	// EvidenceNotApplicable marks a posture item that needs no control: an
	// intentionally public target, or a control the surface does not require.
	EvidenceNotApplicable EvidenceState = "not-applicable"
	// EvidenceWaived marks a finding suppressed by an explicit, justified waiver.
	EvidenceWaived EvidenceState = "waived"
)

// Valid reports whether state is a known evidence classification.
func (state EvidenceState) Valid() bool {
	switch state {
	case EvidenceVerifiedStatic, EvidenceVerifiedRuntime, EvidenceDeclared,
		EvidenceUnverifiedAppOwned, EvidenceNotApplicable, EvidenceWaived:
		return true
	default:
		return false
	}
}

const (
	ownerGOWDKNative = "gowdk-native"
	ownerAppOwned    = "app-owned"
)

// ObligationEntry records one security obligation and how strongly gowdk can
// vouch for it. Statically generated controls (CSRF wiring, body limits,
// security headers, native guards) are verified-static; app-owned controls
// (authentication, session management, tenant/resource authorization, domain
// authorization) are unverified-app-owned because gowdk generates the call sites
// but does not own the decision logic. This list keeps gowdk-security.json honest
// about what the compiler proves versus what the application must prove itself.
type ObligationEntry struct {
	ID       string        `json:"id"`
	Category string        `json:"category"`
	Claim    string        `json:"claim"`
	Owner    string        `json:"owner"`
	Evidence EvidenceState `json:"evidence"`
	Detail   string        `json:"detail,omitempty"`
}

// guardEvidenceState classifies one guard by the binding kind the compiler
// resolved. Native and auth-required guards are statically wired by gowdk;
// app-owned custom guards run but their decision logic is unverified.
func guardEvidenceState(kind string) EvidenceState {
	switch kind {
	case "public":
		return EvidenceNotApplicable
	case "native-rbac", "auth-required":
		return EvidenceVerifiedStatic
	default:
		return EvidenceUnverifiedAppOwned
	}
}

// obligations derives the security obligations of one built module and the
// evidence gowdk can provide for each. The order is stable so the posture digest
// is deterministic. It returns nil when the module exposes no web surface.
func obligations(config gowdk.Config, manifest SecurityManifest) []ObligationEntry {
	hasEndpoints := len(manifest.Endpoints) > 0
	if !hasEndpoints && len(manifest.Routes) == 0 {
		return nil
	}

	var out []ObligationEntry

	if hasEndpoints {
		out = append(out, ObligationEntry{
			ID:       "request.body-limit",
			Category: "request-limit",
			Claim:    "Generated endpoints install a raw-body byte cap before the body is parsed.",
			Owner:    ownerGOWDKNative,
			Evidence: EvidenceVerifiedStatic,
		})
		out = append(out, ObligationEntry{
			ID:       "request.csrf",
			Category: "csrf",
			Claim:    "State-changing endpoints enforce CSRF; any gap is reported as a finding.",
			Owner:    ownerGOWDKNative,
			Evidence: csrfObligationEvidence(manifest),
		})
	}

	headerEvidence := EvidenceNotApplicable
	headerDetail := "no security response headers are configured (Build.SecurityHeaders)"
	if len(manifest.Frontend.ConfiguredHeaders) > 0 {
		headerEvidence = EvidenceVerifiedStatic
		headerDetail = ""
	}
	out = append(out, ObligationEntry{
		ID:       "response.security-headers",
		Category: "header",
		Claim:    "Configured security response headers are emitted by the generated runtime.",
		Owner:    ownerGOWDKNative,
		Evidence: headerEvidence,
		Detail:   headerDetail,
	})

	if len(manifest.Frontend.RawHTMLSinks) > 0 {
		out = append(out, ObligationEntry{
			ID:       "render.raw-html",
			Category: "raw-html",
			Claim:    "Raw-HTML render sinks are inventoried; each must be allowlisted or carry a justified exception.",
			Owner:    ownerGOWDKNative,
			Evidence: EvidenceVerifiedStatic,
		})
	}

	native, custom := guardOwnershipPresence(manifest)
	if native {
		out = append(out, ObligationEntry{
			ID:       "access.native-guards",
			Category: "guard",
			Claim:    "Native role/permission/auth guards are resolved and wired before body decode.",
			Owner:    ownerGOWDKNative,
			Evidence: EvidenceVerifiedStatic,
		})
	}
	if custom {
		out = append(out, ObligationEntry{
			ID:       "access.app-guards",
			Category: "guard",
			Claim:    "App-owned custom guards run before body decode; their decision logic is not verified by gowdk.",
			Owner:    ownerAppOwned,
			Evidence: EvidenceUnverifiedAppOwned,
		})
	}

	authDetail := "no auth addon is configured; authentication is entirely app-owned"
	if config.HasFeature(gowdk.FeatureAuth) {
		authDetail = "the auth addon wires identity plumbing, but credential and identity verification stay app-owned"
	}
	out = append(out, ObligationEntry{
		ID:       "auth.authentication",
		Category: "authentication",
		Claim:    "User authentication (credential checks, token and identity validation) is correct.",
		Owner:    ownerAppOwned,
		Evidence: EvidenceUnverifiedAppOwned,
		Detail:   authDetail,
	})
	out = append(out, ObligationEntry{
		ID:       "session.management",
		Category: "session",
		Claim:    "Session rotation, fixation defense, and secure session storage are correct.",
		Owner:    ownerAppOwned,
		Evidence: EvidenceUnverifiedAppOwned,
	})
	out = append(out, ObligationEntry{
		ID:       "authz.resource",
		Category: "authorization",
		Claim:    "Per-tenant and per-resource (object-level) authorization is enforced.",
		Owner:    ownerAppOwned,
		Evidence: EvidenceUnverifiedAppOwned,
	})
	out = append(out, ObligationEntry{
		ID:       "authz.domain",
		Category: "authorization",
		Claim:    "Domain and business authorization rules are enforced.",
		Owner:    ownerAppOwned,
		Evidence: EvidenceUnverifiedAppOwned,
	})

	return out
}

func csrfObligationEvidence(manifest SecurityManifest) EvidenceState {
	for _, endpoint := range manifest.Endpoints {
		if endpointIsStateChanging(endpoint) {
			return EvidenceVerifiedStatic
		}
	}
	return EvidenceNotApplicable
}

func endpointIsStateChanging(endpoint EndpointEntry) bool {
	switch strings.ToUpper(strings.TrimSpace(endpoint.Method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	switch endpoint.Kind {
	case "action", "command":
		return true
	}
	return false
}

func guardOwnershipPresence(manifest SecurityManifest) (native, custom bool) {
	scan := func(evidence []GuardEvidence) {
		for _, guard := range evidence {
			switch guard.Evidence {
			case EvidenceVerifiedStatic:
				native = true
			case EvidenceUnverifiedAppOwned:
				custom = true
			}
		}
	}
	for _, route := range manifest.Routes {
		scan(route.GuardEvidence)
	}
	for _, endpoint := range manifest.Endpoints {
		scan(endpoint.GuardEvidence)
	}
	return native, custom
}
