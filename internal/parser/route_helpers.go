package parser

import (
	"fmt"
	"strings"

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
		end := start + len([]rune(value))
		spans = append(spans, source.NamedSpan{
			Name: value,
			Span: source.SourceSpan{
				Start: source.SourcePosition{Line: lineNumber, Column: start + 1},
				End:   source.SourcePosition{Line: lineNumber, Column: end + 1},
			},
		})
		searchStart = end
	}
	return spans
}

func parseRouteDeclaration(route string, lineNumber int, rawLine string) (string, []source.RouteParam, []source.NamedSpan, error) {
	matches := routeParamPattern.FindAllStringSubmatchIndex(route, -1)
	if len(matches) == 0 {
		return route, nil, nil, nil
	}
	routeStart := strings.Index(rawLine, route)
	if routeStart < 0 {
		routeStart = 0
	}
	normalizedParts := make([]string, 0, len(matches)*3+1)
	last := 0
	params := make([]source.RouteParam, 0, len(matches))
	spans := make([]source.NamedSpan, 0, len(matches))
	for _, match := range matches {
		name := route[match[2]:match[3]]
		paramType := "string"
		if match[4] >= 0 && match[5] >= 0 {
			paramType = route[match[4]:match[5]]
		}
		if !isSupportedRouteParamType(paramType) {
			return "", nil, nil, fmt.Errorf("unsupported route parameter type %q for %s; supported types: string, int, int64, uint, uint64, bool, float64", paramType, name)
		}
		start := routeStart + match[0]
		end := routeStart + match[1]
		span := source.SourceSpan{
			Start: source.SourcePosition{Line: lineNumber, Column: start + 1},
			End:   source.SourcePosition{Line: lineNumber, Column: end + 1},
		}
		params = append(params, source.RouteParam{Name: name, Type: paramType, Span: span})
		spans = append(spans, source.NamedSpan{
			Name: name,
			Span: span,
		})
		normalizedParts = append(normalizedParts, route[last:match[0]], "{", name, "}")
		last = match[1]
	}
	normalizedParts = append(normalizedParts, route[last:])
	return strings.Join(normalizedParts, ""), params, spans, nil
}

func routeParamSpans(route string, lineNumber int, rawLine string) []source.NamedSpan {
	_, _, spans, _ := parseRouteDeclaration(route, lineNumber, rawLine)
	return spans
}

func isSupportedRouteParamType(value string) bool {
	switch value {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64":
		return true
	default:
		return false
	}
}
