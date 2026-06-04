package html

import (
	stdhtml "html"
	"strings"
)

// Escape escapes text for safe HTML output.
func Escape(value string) string {
	return stdhtml.EscapeString(value)
}

// Attr renders an escaped HTML attribute when value is non-empty.
func Attr(name, value string) string {
	if value == "" {
		return ""
	}
	return " " + name + `="` + stdhtml.EscapeString(value) + `"`
}

// Classes joins generated class tokens.
func Classes(values ...string) string {
	var classes []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			classes = append(classes, strings.TrimSpace(value))
		}
	}
	return strings.Join(classes, " ")
}
