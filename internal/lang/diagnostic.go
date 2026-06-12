package lang

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

// Position (lang.Position) and Range (lang.Range) are aliases of the leaf
// syntax package's types — see syntax_shim.go — so the lexer and editor
// diagnostics share one position type.

// RelatedLocation is a secondary source location attached to a diagnostic, such
// as the first declaration that a conflict diagnostic also points at.
type RelatedLocation struct {
	File    string   `json:"file,omitempty"`
	Pos     Position `json:"pos"`
	Range   *Range   `json:"range,omitempty"`
	Message string   `json:"message,omitempty"`
}

// Diagnostic describes a language-tool finding.
type Diagnostic struct {
	File       string            `json:"file"`
	Code       string            `json:"code,omitempty"`
	Pos        Position          `json:"pos"`
	Range      *Range            `json:"range,omitempty"`
	Severity   string            `json:"severity"`
	Fix        *diagnostics.Fix  `json:"fix,omitempty"`
	Message    string            `json:"message"`
	Suggestion string            `json:"suggestion,omitempty"`
	Related    []RelatedLocation `json:"related,omitempty"`
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
	message := RedactMessage(diagnostic.Message)
	if location != "" {
		return fmt.Sprintf("%s: %s: %s", location, diagnostic.Severity, message)
	}
	return fmt.Sprintf("%s: %s", diagnostic.Severity, message)
}

// MarshalJSON redacts secret-bearing source content from the message and
// suggestion before serialization so check --json and other JSON sinks never
// emit a hardcoded secret quoted from .gwdk source.
func (diagnostic Diagnostic) MarshalJSON() ([]byte, error) {
	type alias Diagnostic
	redacted := alias(diagnostic)
	if redacted.Severity == "" && redacted.Code != "" {
		if severity, ok := diagnostics.DefaultSeverity(redacted.Code); ok {
			redacted.Severity = string(severity)
		}
	}
	if redacted.Fix == nil && redacted.Code != "" {
		if fix, ok := diagnostics.FixFor(redacted.Code); ok {
			redacted.Fix = &fix
		}
	}
	redacted.Message = RedactMessage(redacted.Message)
	redacted.Suggestion = RedactMessage(redacted.Suggestion)
	return json.Marshal(redacted)
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
