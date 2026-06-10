package compiler

import (
	"strings"
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
