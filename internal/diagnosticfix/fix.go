package diagnosticfix

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

// Position is a 1-based text position.
type Position struct {
	Line   int
	Column int
}

// Range is a 1-based text range. End is exclusive.
type Range struct {
	Start Position
	End   Position
}

// Diagnostic is the small diagnostic shape needed by registry rewriters.
type Diagnostic struct {
	Code    string
	Message string
	Range   Range
}

// TextEdit replaces Range with NewText.
type TextEdit struct {
	Range   Range
	NewText string
}

// Edits returns source edits for a registry fix and diagnostic.
func Edits(fix diagnostics.Fix, source string, diagnostic Diagnostic) ([]TextEdit, error) {
	switch fix.Rewriter {
	case diagnostics.FixEndpointHeaderFromMessage:
		return endpointHeaderEdits(source, diagnostic)
	case diagnostics.FixInsertMissingUse:
		return missingUseEdits(source, diagnostic)
	default:
		return nil, fmt.Errorf("diagnostic fix %q has no rewriter", fix.Title)
	}
}

func endpointHeaderEdits(source string, diagnostic Diagnostic) ([]TextEdit, error) {
	replacement, ok := EndpointHeaderReplacement(diagnostic.Message)
	if !ok {
		return nil, fmt.Errorf("%s diagnostic does not include an endpoint replacement", diagnostic.Code)
	}
	replacement = endpointReplacementRoute(source, replacement)
	if emptyRange(diagnostic.Range) {
		return nil, fmt.Errorf("%s diagnostic has no source range", diagnostic.Code)
	}
	editRange, err := endpointBlockRange(source, diagnostic.Range)
	if err != nil {
		return nil, err
	}
	return []TextEdit{{Range: editRange, NewText: replacement}}, nil
}

func missingUseEdits(source string, diagnostic Diagnostic) ([]TextEdit, error) {
	alias, ok := MissingUseAlias(diagnostic.Message)
	if !ok {
		return nil, fmt.Errorf("%s diagnostic does not include a missing use alias", diagnostic.Code)
	}
	insert := UseInsertionPosition(source, diagnostic.Range.Start)
	return []TextEdit{{
		Range:   Range{Start: insert, End: insert},
		NewText: `use ` + alias + ` "package"` + "\n",
	}}, nil
}

// EndpointHeaderReplacement extracts the current endpoint declaration from an
// old endpoint syntax diagnostic message.
func EndpointHeaderReplacement(message string) (string, bool) {
	start := strings.Index(message, "use `")
	if start < 0 {
		return "", false
	}
	start += len("use `")
	end := strings.IndexByte(message[start:], '`')
	if end < 0 {
		return "", false
	}
	replacement := message[start : start+end]
	if strings.HasPrefix(replacement, "act ") || strings.HasPrefix(replacement, "api ") {
		if strings.Contains(message[start+end+1:], "and move behavior to Go") {
			return replacement, true
		}
	}
	return "", false
}

func endpointReplacementRoute(source string, replacement string) string {
	if !strings.Contains(replacement, `"<path>"`) {
		return replacement
	}
	if strings.HasPrefix(replacement, "act ") {
		if route, ok := pageRoute(source); ok {
			return strings.Replace(replacement, `"<path>"`, quoteRoute(route), 1)
		}
	}
	if strings.HasPrefix(replacement, "api ") {
		fields := strings.Fields(replacement)
		if len(fields) >= 2 {
			return strings.Replace(replacement, `"<path>"`, quoteRoute("/api/"+strings.ToLower(fields[1])), 1)
		}
	}
	return replacement
}

func pageRoute(source string) (string, bool) {
	for _, line := range strings.Split(source, "\n") {
		text := strings.TrimSpace(line)
		routeText, ok := strings.CutPrefix(text, "route ")
		if !ok {
			// Keep safe fixes useful for files that still contain legacy metadata.
			routeText, ok = strings.CutPrefix(text, "@route ")
		}
		if !ok {
			continue
		}
		route := strings.TrimSpace(routeText)
		if len(route) >= 2 && route[0] == '"' && route[len(route)-1] == '"' {
			unquoted, err := strconv.Unquote(route)
			if err != nil {
				return "", false
			}
			return unquoted, true
		}
	}
	return "", false
}

func quoteRoute(route string) string {
	if strings.TrimSpace(route) == "" {
		route = "/"
	}
	return strconv.Quote(route)
}

// MissingUseAlias extracts the missing GOWDK use alias from a diagnostic.
func MissingUseAlias(message string) (string, bool) {
	start := strings.Index(message, "Add `use ")
	if start < 0 {
		return "", false
	}
	start += len("Add `use ")
	end := strings.Index(message[start:], ` "<package>"`)
	if end < 0 {
		return "", false
	}
	alias := message[start : start+end]
	if !isIdentifier(alias) {
		return "", false
	}
	return alias, true
}

// UseInsertionPosition inserts use declarations before view when possible.
func UseInsertionPosition(source string, fallback Position) Position {
	lines := strings.Split(source, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == "view {" {
			return Position{Line: index + 1, Column: 1}
		}
	}
	if fallback.Line <= 0 || fallback.Column <= 0 {
		return Position{Line: 1, Column: 1}
	}
	return fallback
}

func emptyRange(item Range) bool {
	return item.Start.Line <= 0 || item.Start.Column <= 0 || item.End.Line <= 0 || item.End.Column <= 0
}

func endpointBlockRange(source string, header Range) (Range, error) {
	lines := strings.Split(source, "\n")
	lineIndex := header.Start.Line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		return Range{}, fmt.Errorf("endpoint diagnostic range is outside the file")
	}
	line := lines[lineIndex]
	open := strings.IndexByte(line, '{')
	if open < 0 {
		return header, nil
	}
	if strings.TrimSpace(line[open+1:]) == "}" {
		header.End = Position{Line: header.Start.Line, Column: len([]rune(line)) + 1}
		return header, nil
	}
	for index := lineIndex + 1; index < len(lines); index++ {
		text := strings.TrimSpace(lines[index])
		if text == "" {
			continue
		}
		if text != "}" {
			return Range{}, fmt.Errorf("old endpoint block contains behavior; move behavior to Go before applying the fix")
		}
		header.End = Position{Line: index + 1, Column: len([]rune(lines[index])) + 1}
		return header, nil
	}
	return Range{}, fmt.Errorf("old endpoint block has no closing brace")
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '_' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
