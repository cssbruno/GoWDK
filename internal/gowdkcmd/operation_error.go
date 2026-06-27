package gowdkcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/lang"
)

type OperationError struct {
	Summary     string
	Diagnostics []devOverlayDiagnostic
	Cause       error
	Code        int
}

func (err *OperationError) Error() string {
	if err == nil {
		return ""
	}
	var lines []string
	if strings.TrimSpace(err.Summary) != "" {
		lines = append(lines, err.Summary)
	}
	for _, diagnostic := range err.Diagnostics {
		lines = append(lines, formatOperationDiagnostic(diagnostic))
	}
	if err.Cause != nil {
		cause := err.Cause.Error()
		if cause != "" && (len(lines) == 0 || cause != lines[len(lines)-1]) {
			lines = append(lines, cause)
		}
	}
	return strings.Join(lines, "\n")
}

func (err *OperationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (err *OperationError) ExitCode() int {
	if err == nil {
		return 0
	}
	return err.Code
}

func formatOperationDiagnostic(diagnostic devOverlayDiagnostic) string {
	severity := strings.TrimSpace(diagnostic.Severity)
	if severity == "" {
		severity = string(compiler.SeverityError)
	}
	label := strings.ToUpper(severity)
	code := strings.TrimSpace(diagnostic.Code)
	if code != "" {
		label += " " + code
	}
	location := strings.TrimSpace(diagnostic.File)
	if diagnostic.Range != nil && diagnostic.Range.Start.Line > 0 {
		if location != "" {
			location = fmt.Sprintf("%s:%d:%d", location, diagnostic.Range.Start.Line, diagnostic.Range.Start.Column)
		} else {
			location = fmt.Sprintf("%d:%d", diagnostic.Range.Start.Line, diagnostic.Range.Start.Column)
		}
	}
	if location != "" {
		return fmt.Sprintf("[%s] %s: %s", label, location, diagnostic.Message)
	}
	return fmt.Sprintf("[%s] %s", label, diagnostic.Message)
}

func operationErrorFromLang(summary string, diagnostics lang.Diagnostics) error {
	return &OperationError{
		Summary:     summary,
		Diagnostics: devOverlayDiagnosticsFromLang(diagnostics),
	}
}

func operationErrorFromCompiler(summary string, diagnostics compiler.ValidationErrors, cause error) error {
	return &OperationError{
		Summary:     summary,
		Diagnostics: devOverlayDiagnosticsFromCompiler(diagnostics),
		Cause:       cause,
	}
}

func operationErrorFromCause(summary string, cause error) error {
	if cause == nil {
		return &OperationError{Summary: summary}
	}
	var compilerDiagnostics compiler.ValidationErrors
	if errors.As(cause, &compilerDiagnostics) {
		return operationErrorFromCompiler(summary, compilerDiagnostics, cause)
	}
	var buildErr *buildgen.BuildError
	if errors.As(cause, &buildErr) {
		return &OperationError{
			Summary:     summary,
			Diagnostics: devOverlayDiagnosticsFromBuildgen(buildErr.Diagnostics),
			Cause:       cause,
		}
	}
	return &OperationError{Summary: summary, Cause: cause}
}

func operationErrorFromAuditFindings(summary string, findings []auditspec.Finding, cause error) error {
	out := make([]devOverlayDiagnostic, 0, len(findings))
	for _, finding := range findings {
		if finding.Severity != diagnostics.SeverityError || finding.Suppression != nil {
			continue
		}
		out = append(out, devOverlayDiagnostic{
			Code:     finding.Code,
			Severity: string(finding.Severity),
			Message:  finding.Message,
			File:     finding.Source,
		})
	}
	return &OperationError{Summary: summary, Diagnostics: out, Cause: cause}
}
