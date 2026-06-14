package compiler

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}

// HasErrors reports whether any diagnostic is error severity. Warning-only
// reports return false so the build proceeds.
func (errs ValidationErrors) HasErrors() bool {
	for _, err := range errs {
		if err.Severity != SeverityWarning {
			return true
		}
	}
	return false
}

// Warnings returns only the warning-severity diagnostics.
func (errs ValidationErrors) Warnings() ValidationErrors {
	var out ValidationErrors
	for _, err := range errs {
		if err.Severity == SeverityWarning {
			out = append(out, err)
		}
	}
	return out
}

func normalizeValidationErrors(errs []ValidationError) ValidationErrors {
	normalized := make(ValidationErrors, len(errs))
	copy(normalized, errs)
	for index := range normalized {
		normalized[index].normalizeSeverity()
	}
	return normalized
}

func (err *ValidationError) normalizeSeverity() {
	if err.Severity != "" {
		return
	}
	if err.Code != "" {
		if severity, ok := diagnostics.DefaultSeverity(err.Code); ok {
			err.Severity = severity
			return
		}
	}
	err.Severity = SeverityError
}
