package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/source"
)

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
