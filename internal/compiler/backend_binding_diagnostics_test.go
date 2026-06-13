package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func fragmentPolicyProgram(t *testing.T, goBlockBody string) gwdkir.Program {
	t.Helper()
	return gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:      "patients",
			Package: "pages",
			Source:  "no-such-dir/patients.page.gwdk",
			Route:   "/patients",
			Blocks: gwdkir.Blocks{
				Fragments: []gwdkir.FragmentEndpoint{{Name: "Summary", Method: "GET", Route: "/patients/summary", Target: "#summary"}},
				GoBlocks:  []gwdkir.GoBlock{{Body: goBlockBody}},
			},
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:   gwdkir.EndpointFragment,
			PageID: "patients",
			Symbol: "Summary",
			Method: "GET",
			Path:   "/patients/summary",
		}},
	}
}

func TestProductionPolicyAllowsUnexportedFragmentNearMiss(t *testing.T) {
	ir := fragmentPolicyProgram(t, "func summary() {}")
	BindBackendHandlers(&ir)
	err := ValidateBackendBindingPolicyIR(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, ir)
	if err != nil {
		t.Fatalf("expected an unexported fragment near-miss to stay a warning in production, got error: %v", err)
	}
}

func TestProductionPolicyStillRejectsUnsupportedFragmentSignature(t *testing.T) {
	ir := fragmentPolicyProgram(t, "func Summary(x int) {}")
	BindBackendHandlers(&ir)
	err := ValidateBackendBindingPolicyIR(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, ir)
	if err == nil || !strings.Contains(err.Error(), "fragment") {
		t.Fatalf("expected production to still reject an unsupported fragment signature, got: %v", err)
	}
}

func findBindingByName(bindings []source.BackendBinding, kind, name string) (source.BackendBinding, bool) {
	for _, binding := range bindings {
		if binding.Kind == kind && binding.FunctionName == name {
			return binding, true
		}
	}
	return source.BackendBinding{}, false
}

func TestComputeBackendBindingsFlagsUnexportedInlineActionHandler(t *testing.T) {
	// Same-package directory does not exist, so the only candidate is the
	// inline default go {} block, which declares an unexported near-miss.
	page := gwdkir.Page{
		ID:      "signup",
		Package: "pages",
		Source:  "no-such-dir/signup.page.gwdk",
		Route:   "/signup",
		Blocks: gwdkir.Blocks{
			Actions:  []gwdkir.Action{{Name: "Submit", Method: "POST", Route: "/signup"}},
			GoBlocks: []gwdkir.GoBlock{{Body: "func submit() {}"}},
		},
	}
	bindings := computeBackendBindings(gwdkir.Program{Pages: []gwdkir.Page{page}})
	binding, ok := findBindingByName(bindings, "action", "Submit")
	if !ok {
		t.Fatalf("missing Submit action binding in %#v", bindings)
	}
	if binding.Status != source.BackendBindingMissing || !binding.UnexportedCandidate {
		t.Fatalf("expected missing+unexported binding, got %#v", binding)
	}
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 || diagnostics[0].Code != "unexported_backend_handler" {
		t.Fatalf("expected one unexported_backend_handler diagnostic, got %#v", diagnostics)
	}
}

func TestComputeBackendBindingsFlagsUnexportedInlineFragmentHandler(t *testing.T) {
	page := gwdkir.Page{
		ID:      "patients",
		Package: "pages",
		Source:  "no-such-dir/patients.page.gwdk",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{
			Fragments: []gwdkir.FragmentEndpoint{{Name: "Summary", Method: "GET", Route: "/patients/summary", Target: "#summary"}},
			GoBlocks:  []gwdkir.GoBlock{{Body: "func summary() {}"}},
		},
	}
	bindings := computeBackendBindings(gwdkir.Program{Pages: []gwdkir.Page{page}})
	binding, ok := findBindingByName(bindings, "fragment", "Summary")
	if !ok {
		t.Fatalf("expected a fragment binding for the unexported near-miss, got %#v", bindings)
	}
	if binding.Status != source.BackendBindingMissing || !binding.UnexportedCandidate {
		t.Fatalf("expected missing+unexported fragment binding, got %#v", binding)
	}
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 || diagnostics[0].Code != "unexported_backend_handler" {
		t.Fatalf("expected one unexported_backend_handler diagnostic, got %#v", diagnostics)
	}
}

func TestComputeBackendBindingsStaysSilentForFragmentWithoutCandidate(t *testing.T) {
	page := gwdkir.Page{
		ID:      "patients",
		Package: "pages",
		Source:  "no-such-dir/patients.page.gwdk",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{
			Fragments: []gwdkir.FragmentEndpoint{{Name: "Summary", Method: "GET", Route: "/patients/summary", Target: "#summary"}},
		},
	}
	bindings := computeBackendBindings(gwdkir.Program{Pages: []gwdkir.Page{page}})
	if _, ok := findBindingByName(bindings, "fragment", "Summary"); ok {
		t.Fatalf("expected no fragment binding without a handler candidate, got %#v", bindings)
	}
	if diagnostics := BackendBindingDiagnostics(bindings); len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for a handler-less fragment, got %#v", diagnostics)
	}
}

func TestBackendBindingDiagnosticsWarnsOnUnsupportedSignature(t *testing.T) {
	bindings := []source.BackendBinding{{
		Kind:         "action",
		PageID:       "signup",
		Source:       "pages/signup.page.gwdk",
		FunctionName: "Submit",
		Status:       source.BackendBindingUnsupportedSignature,
		Message:      "GOWDK action handler pages.Submit is unsupported: wrong shape",
	}}
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	got := diagnostics[0]
	if got.Code != "unsupported_backend_signature" {
		t.Fatalf("expected unsupported_backend_signature, got %q", got.Code)
	}
	if got.Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %v", got.Severity)
	}
	if got.PageID != "signup" || got.Source != "pages/signup.page.gwdk" {
		t.Fatalf("expected page/source carried through, got %#v", got)
	}
}

func TestBackendBindingDiagnosticsWarnsOnUnexportedCandidate(t *testing.T) {
	bindings := []source.BackendBinding{{
		Kind:                "api",
		PageID:              "session",
		Source:              "pages/session.page.gwdk",
		FunctionName:        "Session",
		Status:              source.BackendBindingMissing,
		UnexportedCandidate: true,
		Message:             "GOWDK API handler pages.Session is not implemented; an unexported function session exists in the same package — export it as Session",
	}}
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "unexported_backend_handler" {
		t.Fatalf("expected unexported_backend_handler, got %q", diagnostics[0].Code)
	}
	if diagnostics[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %v", diagnostics[0].Severity)
	}
}

func TestBackendBindingDiagnosticsStaysSilentForBoundAndPlainMissing(t *testing.T) {
	bindings := []source.BackendBinding{
		{Kind: "action", FunctionName: "Bound", Status: source.BackendBindingBound},
		{Kind: "action", FunctionName: "NotImplemented", Status: source.BackendBindingMissing},
	}
	if diagnostics := BackendBindingDiagnostics(bindings); len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for bound and plain-missing bindings, got %#v", diagnostics)
	}
}

func TestFirstRuneLower(t *testing.T) {
	cases := map[string]string{
		"Submit":  "submit",
		"Session": "session",
		"A":       "a",
		"":        "",
	}
	for input, want := range cases {
		if got := firstRuneLower(input); got != want {
			t.Fatalf("firstRuneLower(%q) = %q, want %q", input, got, want)
		}
	}
}
