package actions

import (
	"strings"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/validation"
)

// ValidateRequired records one validation error for each missing required form
// field. Empty and whitespace-only submitted values are treated as missing.
func ValidateRequired(values form.Values, fields []string) validation.Result {
	var result validation.Result
	for _, field := range fields {
		if !hasSubmittedValue(values.All(field)) {
			result.Add(field, "required")
		}
	}
	return result
}

func hasSubmittedValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
