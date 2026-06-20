package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestDevOverlayErrorEventDataIncludesContext(t *testing.T) {
	lastSuccess := time.Date(2026, 6, 14, 10, 20, 30, 0, time.UTC)
	err := newDevDiagnosticError("build failed", []devOverlayDiagnostic{{
		Code:     "invalid_view",
		Severity: "error",
		Message:  "view block is invalid",
		File:     "src/home.page.gwdk",
		Range: &devOverlayRange{
			Start: devOverlayPosition{Line: 4, Column: 2},
			End:   devOverlayPosition{Line: 4, Column: 8},
		},
	}})

	data := devOverlayErrorEventData(err, inputChange{Changed: []string{"src/home.page.gwdk"}}, lastSuccess)

	var payload devOverlayPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("expected JSON payload, got %q: %v", data, err)
	}
	if payload.Message != "build failed" {
		t.Fatalf("unexpected message: %#v", payload)
	}
	if payload.LastSuccessfulBuild != lastSuccess.Format(time.RFC3339) {
		t.Fatalf("unexpected last successful build: %#v", payload)
	}
	if len(payload.ChangedFiles) != 1 || payload.ChangedFiles[0] != "changed: src/home.page.gwdk" {
		t.Fatalf("unexpected changed files: %#v", payload.ChangedFiles)
	}
	if len(payload.Diagnostics) != 1 || payload.Diagnostics[0].Code != "invalid_view" || payload.Diagnostics[0].Range.Start.Line != 4 {
		t.Fatalf("unexpected diagnostics: %#v", payload.Diagnostics)
	}
}

func TestDevOverlayDiagnosticsFromLanguageDiagnostics(t *testing.T) {
	diagnostics := devOverlayDiagnosticsFromLang(lang.Diagnostics{{
		File:     "src/about.page.gwdk",
		Code:     "unexpected_token",
		Severity: "error",
		Message:  "unexpected token",
		Range: &lang.Range{
			Start: lang.Position{Line: 7, Column: 3},
			End:   lang.Position{Line: 7, Column: 9},
		},
	}})

	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	got := diagnostics[0]
	if got.Code != "unexpected_token" || got.File != "src/about.page.gwdk" || got.Range.Start.Line != 7 || got.Range.End.Column != 9 {
		t.Fatalf("unexpected diagnostic: %#v", got)
	}
}

func TestDevOverlayBuildgenErrorIncludesDiagnosticsAndAttribution(t *testing.T) {
	err := &buildgen.BuildError{
		Err: errors.New("write output"),
		Diagnostics: []buildgen.BuildDiagnostic{{
			Code:    "asset_missing",
			Source:  "src/dashboard.page.gwdk",
			Message: "asset does not exist",
			Span: source.SourceSpan{
				Start: source.SourcePosition{Line: 12, Column: 5},
				End:   source.SourcePosition{Line: 12, Column: 18},
			},
		}},
		Report: buildgen.BuildReport{Events: []buildgen.BuildEvent{{
			Level: buildgen.BuildEventError,
			Route: "/dashboard",
			Path:  "GET /dashboard",
		}}},
	}

	data := devOverlayErrorEventData(err, inputChange{}, time.Time{})

	var payload devOverlayPayload
	if decodeErr := json.Unmarshal([]byte(data), &payload); decodeErr != nil {
		t.Fatalf("expected JSON payload, got %q: %v", data, decodeErr)
	}
	if payload.Route != "/dashboard" || payload.Endpoint != "GET /dashboard" {
		t.Fatalf("unexpected attribution: %#v", payload)
	}
	if len(payload.Diagnostics) != 1 || payload.Diagnostics[0].Code != "asset_missing" || payload.Diagnostics[0].Range.Start.Column != 5 {
		t.Fatalf("unexpected diagnostics: %#v", payload.Diagnostics)
	}
}

func TestDevRuntimeErrorEventDataIsGeneric(t *testing.T) {
	data := devRuntimeErrorEventData(500)

	var payload devOverlayPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("expected JSON payload, got %q: %v", data, err)
	}
	if payload.Title != "GOWDK runtime request failed" || payload.Status != 500 {
		t.Fatalf("unexpected runtime payload: %#v", payload)
	}
	for _, forbidden := range []string{"cookie", "password", "token", "request body", "panic:"} {
		if strings.Contains(strings.ToLower(data), forbidden) {
			t.Fatalf("runtime payload should stay generic and omit %q: %s", forbidden, data)
		}
	}
}
