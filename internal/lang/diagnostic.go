package lang

import (
	"fmt"
	"strings"
)

// Position is a 1-based source location.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Diagnostic describes a language-tool finding.
type Diagnostic struct {
	File     string   `json:"file"`
	Pos      Position `json:"pos"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
}

func (diagnostic Diagnostic) String() string {
	var location string
	if diagnostic.File != "" {
		location = diagnostic.File
	}
	if diagnostic.Pos.Line > 0 {
		if location != "" {
			location += ":"
		}
		location += fmt.Sprintf("%d:%d", diagnostic.Pos.Line, diagnostic.Pos.Column)
	}
	if location != "" {
		return fmt.Sprintf("%s: %s: %s", location, diagnostic.Severity, diagnostic.Message)
	}
	return fmt.Sprintf("%s: %s", diagnostic.Severity, diagnostic.Message)
}

// Diagnostics is a collection that implements error.
type Diagnostics []Diagnostic

func (diagnostics Diagnostics) Error() string {
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, diagnostic.String())
	}
	return strings.Join(lines, "\n")
}

// HasErrors reports whether any diagnostic is an error.
func (diagnostics Diagnostics) HasErrors() bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}
