package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// emptyPackageSource returns a page source path inside a fresh empty directory.
// inspectFeaturePackage reports the directory as having no Go files (no
// LoadError, no functions), so binding falls through to the inline go {} block.
func emptyPackageSource(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func fragmentPolicyProgram(t *testing.T, goBlockBody string) gwdkir.Program {
	t.Helper()
	return gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:      "patients",
			Package: "pages",
			Source:  emptyPackageSource(t, "patients.page.gwdk"),
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
	// The same-package directory has no Go files, so the only candidate is the
	// inline default go {} block, which declares an unexported near-miss.
	page := gwdkir.Page{
		ID:      "signup",
		Package: "pages",
		Source:  emptyPackageSource(t, "signup.page.gwdk"),
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
		Source:  emptyPackageSource(t, "patients.page.gwdk"),
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

const inlineSubmitGoBlock = `import (
	"context"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Submit(context.Context) (response.Response, error) {
	return response.Response{}, nil
}`

func TestComputeBackendBindingsFlagsAmbiguousHandler(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	writeCompilerTestFile(t, filepath.Join(root, "handlers.go"), `package pages

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Submit(context.Context) (response.Response, error) {
	return response.Response{}, nil
}
`)
	page := gwdkir.Page{
		ID:      "signup",
		Package: "pages",
		Source:  filepath.Join(root, "signup.page.gwdk"),
		Route:   "/signup",
		Blocks: gwdkir.Blocks{
			Actions:  []gwdkir.Action{{Name: "Submit", Method: "POST", Route: "/signup"}},
			GoBlocks: []gwdkir.GoBlock{{Body: inlineSubmitGoBlock}},
		},
	}
	bindings := computeBackendBindings(gwdkir.Program{Pages: []gwdkir.Page{page}})
	binding, ok := findBindingByName(bindings, "action", "Submit")
	if !ok || !binding.Ambiguous {
		t.Fatalf("expected an ambiguous Submit binding, got %#v", binding)
	}
	if !strings.Contains(binding.Message, "declared in both") {
		t.Fatalf("expected ambiguity message, got %q", binding.Message)
	}
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 || diagnostics[0].Code != "ambiguous_backend_handler" {
		t.Fatalf("expected one ambiguous_backend_handler diagnostic, got %#v", diagnostics)
	}
}

func TestComputeBackendBindingsDoesNotMaskSamePackageCompileError(t *testing.T) {
	root := t.TempDir()
	writeCompilerTestModule(t, root)
	// A sibling Go file that fails to type-check, so the same-package package
	// cannot be inspected.
	writeCompilerTestFile(t, filepath.Join(root, "broken.go"), `package pages

func Broken() int { return "not an int" }
`)
	page := gwdkir.Page{
		ID:      "signup",
		Package: "pages",
		Source:  filepath.Join(root, "signup.page.gwdk"),
		Route:   "/signup",
		Blocks: gwdkir.Blocks{
			Actions:  []gwdkir.Action{{Name: "Submit", Method: "POST", Route: "/signup"}},
			GoBlocks: []gwdkir.GoBlock{{Body: inlineSubmitGoBlock}},
		},
	}
	bindings := computeBackendBindings(gwdkir.Program{Pages: []gwdkir.Page{page}})
	binding, ok := findBindingByName(bindings, "action", "Submit")
	if !ok {
		t.Fatalf("expected a Submit binding, got %#v", bindings)
	}
	// Even though the inline go {} block declares a valid Submit, a broken
	// same-package package must not be masked by reporting the inline handler as
	// bound. The compile error itself is reported separately by go_package_error.
	if binding.Status == source.BackendBindingBound {
		t.Fatalf("expected the broken same-package package not to report Submit as bound via inline fallback, got %#v", binding)
	}
	if !strings.Contains(binding.Message, "could not be inspected") {
		t.Fatalf("expected a could-not-inspect message surfacing the compile error, got %q", binding.Message)
	}
}

func TestComputeBackendBindingsStaysSilentForFragmentWithoutCandidate(t *testing.T) {
	page := gwdkir.Page{
		ID:      "patients",
		Package: "pages",
		Source:  emptyPackageSource(t, "patients.page.gwdk"),
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
