package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

func TestNormalizeValidationErrorsUsesRegistrySeverity(t *testing.T) {
	errs := normalizeValidationErrors([]ValidationError{{Code: "missing_page_guard"}})
	if len(errs) != 1 {
		t.Fatalf("expected one diagnostic, got %d", len(errs))
	}
	if errs[0].Severity != diagnostics.SeverityWarning {
		t.Fatalf("expected registry warning severity, got %q", errs[0].Severity)
	}
	if errs.HasErrors() {
		t.Fatalf("warning-only diagnostics should not fail the build: %#v", errs)
	}
}

func TestNormalizeValidationErrorsFailsClosedForUnknownCodes(t *testing.T) {
	errs := normalizeValidationErrors([]ValidationError{{Code: "not_registered"}})
	if len(errs) != 1 {
		t.Fatalf("expected one diagnostic, got %d", len(errs))
	}
	if errs[0].Severity != SeverityError {
		t.Fatalf("unknown diagnostic should default to error, got %q", errs[0].Severity)
	}
	if !errs.HasErrors() {
		t.Fatalf("unknown diagnostic should fail the build")
	}
}
