package parser

import (
	"fmt"
	"strings"
)

func validateFragmentTarget(value string) error {
	if value == "" {
		return fmt.Errorf("fragment target is required")
	}
	if strings.ContainsAny(value, "\r\n\t ") {
		return fmt.Errorf("fragment target %q must not contain whitespace", value)
	}
	if !strings.HasPrefix(value, "#") || strings.TrimPrefix(value, "#") == "" {
		return fmt.Errorf("fragment target %q must be a literal id selector", value)
	}
	if strings.ContainsAny(value, "{}") {
		return fmt.Errorf("fragment target %q must be literal", value)
	}
	return nil
}
