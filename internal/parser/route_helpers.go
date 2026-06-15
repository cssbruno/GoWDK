package parser

import (
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/gowdk/internal/source"
)

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSSList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func sourceLineSpan(lineNumber int, rawLine string) source.SourceSpan {
	startColumn := 1
	for _, r := range rawLine {
		if r != ' ' && r != '\t' {
			break
		}
		startColumn++
	}
	endColumn := len([]rune(rawLine)) + 1
	if endColumn <= startColumn {
		endColumn = startColumn + 1
	}
	return source.SourceSpan{
		Start: source.SourcePosition{Line: lineNumber, Column: startColumn},
		End:   source.SourcePosition{Line: lineNumber, Column: endColumn},
	}
}

func sourceBodyStart(lines []string, firstLineNumber int) source.SourcePosition {
	for offset, rawLine := range lines {
		for index, char := range []rune(rawLine) {
			if strings.TrimSpace(string(char)) == "" {
				continue
			}
			return source.SourcePosition{Line: firstLineNumber + offset, Column: index + 1}
		}
	}
	return source.SourcePosition{}
}

func namedValueSpans(values []string, lineNumber int, rawLine string) []source.NamedSpan {
	if len(values) == 0 {
		return nil
	}
	spans := make([]source.NamedSpan, 0, len(values))
	searchStart := 0
	for _, value := range values {
		if value == "" {
			continue
		}
		index := strings.Index(rawLine[searchStart:], value)
		if index < 0 {
			spans = append(spans, source.NamedSpan{Name: value, Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		start := searchStart + index
		end := start + len(value)
		spans = append(spans, source.NamedSpan{
			Name: value,
			Span: source.SourceSpan{
				Start: source.SourcePosition{Line: lineNumber, Column: runeColumn(rawLine, start)},
				End:   source.SourcePosition{Line: lineNumber, Column: runeColumn(rawLine, end)},
			},
		})
		searchStart = end
	}
	return spans
}

// runeColumn converts a byte offset within line to the 1-based rune column the
// lexer reports. Offsets past the end of the line are clamped so callers with a
// fallback offset cannot slice out of range.
func runeColumn(line string, byteOffset int) int {
	if byteOffset > len(line) {
		byteOffset = len(line)
	}
	return utf8.RuneCountInString(line[:byteOffset]) + 1
}

func parseRouteDeclaration(route string, lineNumber int, rawLine string) (string, []source.RouteParam, []source.NamedSpan, error) {
	return source.ParseRouteDeclaration(route, lineNumber, rawLine)
}

func routeParamSpans(route string, lineNumber int, rawLine string) []source.NamedSpan {
	_, _, spans, _ := parseRouteDeclaration(route, lineNumber, rawLine)
	return spans
}
