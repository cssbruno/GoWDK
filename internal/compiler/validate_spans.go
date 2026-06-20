package compiler

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func firstSpan(spans ...source.SourceSpan) source.SourceSpan {
	for _, span := range spans {
		if hasSpan(span) {
			return span
		}
	}
	return source.SourceSpan{}
}

func pageViewBodyOffsetSpan(page gwdkir.Page, start int, end int) source.SourceSpan {
	return viewBodyOffsetSpan(
		page.Blocks.ViewBody,
		page.Blocks.Spans.ViewBodyStart,
		page.Blocks.Spans.View,
		page.Spans.Page,
		start,
		end,
	)
}

func componentViewBodyOffsetSpan(component gwdkir.Component, start int, end int) source.SourceSpan {
	return viewBodyOffsetSpan(
		component.Blocks.ViewBody,
		component.Blocks.Spans.ViewBodyStart,
		component.Blocks.Spans.View,
		component.Span,
		start,
		end,
	)
}

func viewBodyOffsetSpan(body string, bodyStart source.SourcePosition, viewSpan source.SourceSpan, fallback source.SourceSpan, start int, end int) source.SourceSpan {
	if start < 0 || end <= start || start >= len([]rune(body)) {
		return firstSpan(viewSpan, fallback)
	}
	startPos := bodyOffsetPosition(body, bodyStart, viewSpan, start)
	endPos := bodyOffsetPosition(body, bodyStart, viewSpan, end)
	if startPos.Line == 0 || endPos.Line == 0 {
		return firstSpan(viewSpan, fallback)
	}
	return source.SourceSpan{Start: startPos, End: endPos}
}

func bodyOffsetPosition(body string, bodyStart source.SourcePosition, viewSpan source.SourceSpan, offset int) source.SourcePosition {
	line := bodyStart.Line
	column := bodyStart.Column
	if line == 0 || column == 0 {
		line = viewSpan.Start.Line + 1
		column = 1
	}
	if line == 0 || column == 0 {
		return source.SourcePosition{}
	}
	for index, char := range []rune(body) {
		if index == offset {
			return source.SourcePosition{Line: line, Column: column}
		}
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	if offset == len([]rune(body)) {
		return source.SourcePosition{Line: line, Column: column}
	}
	return source.SourcePosition{}
}

func firstNamedSpan(spans []source.NamedSpan, fallback source.SourceSpan) source.SourceSpan {
	for _, item := range spans {
		if hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForName(spans []source.NamedSpan, name string, fallback source.SourceSpan) source.SourceSpan {
	for _, item := range spans {
		if item.Name == name && hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForNameOccurrence(spans []source.NamedSpan, name string, occurrence int, fallback source.SourceSpan) source.SourceSpan {
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

func hasSpan(span source.SourceSpan) bool {
	return span.Start.Line > 0 && span.Start.Column > 0 && span.End.Line > 0 && span.End.Column > 0
}
