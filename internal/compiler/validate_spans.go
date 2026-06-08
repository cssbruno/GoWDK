package compiler

import (
	"github.com/cssbruno/gowdk/internal/manifest"
	"strings"
)

func firstSpan(spans ...manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if hasSpan(span) {
			return span
		}
	}
	return manifest.SourceSpan{}
}

func viewBodyNeedleSpan(component manifest.Component, needle string) manifest.SourceSpan {
	needle = strings.TrimSpace(needle)
	if needle == "" || component.Blocks.ViewBody == "" || !hasSpan(component.Blocks.Spans.View) {
		return firstSpan(component.Blocks.Spans.View, component.Span)
	}
	offset := strings.Index(component.Blocks.ViewBody, needle)
	if offset < 0 {
		return firstSpan(component.Blocks.Spans.View, component.Span)
	}
	before := component.Blocks.ViewBody[:offset]
	lineOffset := strings.Count(before, "\n")
	lastNewline := strings.LastIndex(before, "\n")
	lineStart := 0
	if lastNewline >= 0 {
		lineStart = lastNewline + 1
	}
	startColumn := len([]rune(before[lineStart:])) + 1
	endColumn := startColumn + len([]rune(needle))
	return manifest.SourceSpan{
		Start: manifest.SourcePosition{
			Line:   component.Blocks.Spans.View.Start.Line + 1 + lineOffset,
			Column: startColumn,
		},
		End: manifest.SourcePosition{
			Line:   component.Blocks.Spans.View.Start.Line + 1 + lineOffset,
			Column: endColumn,
		},
	}
}

func firstNamedSpan(spans []manifest.NamedSpan, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForName(spans []manifest.NamedSpan, name string, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if item.Name == name && hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForNameOccurrence(spans []manifest.NamedSpan, name string, occurrence int, fallback manifest.SourceSpan) manifest.SourceSpan {
	if occurrence <= 1 {
		return spanForName(spans, name, fallback)
	}
	seen := 0
	for _, item := range spans {
		if item.Name != name {
			continue
		}
		seen++
		if seen == occurrence {
			if hasSpan(item.Span) {
				return item.Span
			}
			return fallback
		}
	}
	return spanForName(spans, name, fallback)
}

func hasSpan(span manifest.SourceSpan) bool {
	return span.Start.Line > 0 && span.Start.Column > 0 && span.End.Line > 0 && span.End.Column > 0
}
