package gowdkcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/source"
)

type devOverlayPayload struct {
	Title               string                 `json:"title,omitempty"`
	Message             string                 `json:"message"`
	Status              int                    `json:"status,omitempty"`
	Diagnostics         []devOverlayDiagnostic `json:"diagnostics,omitempty"`
	LastSuccessfulBuild string                 `json:"lastSuccessfulBuild,omitempty"`
	ChangedFiles        []string               `json:"changedFiles,omitempty"`
	Route               string                 `json:"route,omitempty"`
	Endpoint            string                 `json:"endpoint,omitempty"`
}

type devOverlayDiagnostic struct {
	Code      string           `json:"code,omitempty"`
	Severity  string           `json:"severity,omitempty"`
	Message   string           `json:"message"`
	File      string           `json:"file,omitempty"`
	Range     *devOverlayRange `json:"range,omitempty"`
	Route     string           `json:"route,omitempty"`
	Endpoint  string           `json:"endpoint,omitempty"`
	PageID    string           `json:"pageId,omitempty"`
	Component string           `json:"component,omitempty"`
}

type devOverlayRange struct {
	Start devOverlayPosition `json:"start"`
	End   devOverlayPosition `json:"end,omitempty"`
}

type devOverlayPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type devDiagnosticError struct {
	message     string
	diagnostics []devOverlayDiagnostic
}

func (err *devDiagnosticError) Error() string {
	if err == nil {
		return ""
	}
	return err.message
}

func newDevDiagnosticError(message string, diagnostics []devOverlayDiagnostic) error {
	return &devDiagnosticError{message: message, diagnostics: diagnostics}
}

func devOverlayErrorEventData(err error, change inputChange, lastSuccessfulBuild time.Time) string {
	payload := devOverlayPayload{
		Message:      "Check the terminal for details.",
		Diagnostics:  devOverlayDiagnosticsFromError(err),
		ChangedFiles: change.details(),
	}
	if err != nil && err.Error() != "" {
		payload.Message = err.Error()
	}
	if !lastSuccessfulBuild.IsZero() {
		payload.LastSuccessfulBuild = lastSuccessfulBuild.Format(time.RFC3339)
	}
	payload.Route, payload.Endpoint = devOverlayAttributionFromError(err)

	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return payload.Message
	}
	return string(data)
}

func devRuntimeErrorEventData(status int) string {
	if status <= 0 {
		status = 500
	}
	payload := devOverlayPayload{
		Title:   "GOWDK runtime request failed",
		Message: fmt.Sprintf("Generated app returned HTTP %d through the dev runtime proxy. Check terminal logs for redacted runtime details.", status),
		Status:  status,
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return payload.Message
	}
	return string(data)
}

func devOverlayDiagnosticsFromError(err error) []devOverlayDiagnostic {
	if err == nil {
		return nil
	}
	var out []devOverlayDiagnostic
	var diagnosticErr *devDiagnosticError
	if errors.As(err, &diagnosticErr) {
		out = append(out, diagnosticErr.diagnostics...)
	}
	var operationErr *OperationError
	if errors.As(err, &operationErr) {
		out = append(out, operationErr.Diagnostics...)
	}
	var buildErr *buildgen.BuildError
	if errors.As(err, &buildErr) {
		out = append(out, devOverlayDiagnosticsFromBuildgen(buildErr.Diagnostics)...)
	}
	return out
}

func devOverlayAttributionFromError(err error) (route string, endpoint string) {
	if err == nil {
		return "", ""
	}
	var buildErr *buildgen.BuildError
	if !errors.As(err, &buildErr) {
		return "", ""
	}
	for index := len(buildErr.Report.Events) - 1; index >= 0; index-- {
		event := buildErr.Report.Events[index]
		if event.Level != buildgen.BuildEventError {
			continue
		}
		if event.Route != "" || event.Path != "" {
			return event.Route, event.Path
		}
	}
	return "", ""
}

func devOverlayDiagnosticsFromLang(diagnostics lang.Diagnostics) []devOverlayDiagnostic {
	out := make([]devOverlayDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, devOverlayDiagnostic{
			Code:     diagnostic.Code,
			Severity: diagnostic.Severity,
			Message:  lang.RedactMessage(diagnostic.Message),
			File:     diagnostic.File,
			Range:    devOverlayRangeFromLang(diagnostic.Pos, diagnostic.Range),
		})
	}
	return out
}

func devOverlayDiagnosticsFromCompiler(diagnostics compiler.ValidationErrors) []devOverlayDiagnostic {
	out := make([]devOverlayDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		severity := string(diagnostic.Severity)
		if severity == "" {
			severity = string(compiler.SeverityError)
		}
		out = append(out, devOverlayDiagnostic{
			Code:      diagnostic.Code,
			Severity:  severity,
			Message:   diagnostic.Message,
			File:      diagnostic.Source,
			Range:     devOverlayRangeFromSource(diagnostic.Span),
			PageID:    diagnostic.PageID,
			Component: diagnostic.ComponentName,
		})
	}
	return out
}

func devOverlayDiagnosticsFromBuildgen(diagnostics []buildgen.BuildDiagnostic) []devOverlayDiagnostic {
	out := make([]devOverlayDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, devOverlayDiagnostic{
			Code:      diagnostic.Code,
			Severity:  string(compiler.SeverityError),
			Message:   diagnostic.Message,
			File:      diagnostic.Source,
			Range:     devOverlayRangeFromSource(diagnostic.Span),
			Component: diagnostic.ComponentName,
		})
	}
	return out
}

func devOverlayRangeFromLang(pos lang.Position, rng *lang.Range) *devOverlayRange {
	if rng != nil {
		return &devOverlayRange{
			Start: devOverlayPosition{Line: rng.Start.Line, Column: rng.Start.Column},
			End:   devOverlayPosition{Line: rng.End.Line, Column: rng.End.Column},
		}
	}
	if pos.Line <= 0 || pos.Column <= 0 {
		return nil
	}
	position := devOverlayPosition{Line: pos.Line, Column: pos.Column}
	return &devOverlayRange{Start: position, End: position}
}

func devOverlayRangeFromSource(span source.SourceSpan) *devOverlayRange {
	if span.Start.Line <= 0 || span.Start.Column <= 0 {
		return nil
	}
	return &devOverlayRange{
		Start: devOverlayPosition{Line: span.Start.Line, Column: span.Start.Column},
		End:   devOverlayPosition{Line: span.End.Line, Column: span.End.Column},
	}
}

func devLastSuccessfulBuildTime(outputDir string, fallback time.Time) time.Time {
	info, err := os.Stat(devInputCachePath(outputDir))
	if err == nil {
		return info.ModTime()
	}
	return fallback
}
