package lsp

import (
	"strings"
	"unicode/utf16"

	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func diagnosticFromLang(item lang.Diagnostic, source string) diagnostic {
	severity := diagnosticSeverityError
	if item.Severity == "warning" {
		severity = diagnosticSeverityWarning
	}
	return diagnostic{
		Range:    rangeFromLangDiagnostic(item, source),
		Severity: severity,
		Code:     item.Code,
		Source:   "gowdk",
		Message:  item.Message,
	}
}

func rangeFromLangDiagnostic(item lang.Diagnostic, source string) lspRange {
	if item.Range != nil {
		return rangeFromLangRange(*item.Range, source)
	}
	return rangeFromPosition(item.Pos, source)
}

func rangeFromLangRange(item lang.Range, source string) lspRange {
	start := positionFromLangPosition(item.Start, source)
	end := positionFromLangPosition(item.End, source)
	if end.Line < start.Line || (end.Line == start.Line && end.Character <= start.Character) {
		end = position{Line: start.Line, Character: start.Character + 1}
	}
	return lspRange{Start: start, End: end}
}

func lspRangeFromSourceSpan(span manifest.SourceSpan, source string) lspRange {
	return rangeFromLangRange(lang.Range{
		Start: lang.Position{Line: span.Start.Line, Column: span.Start.Column},
		End:   lang.Position{Line: span.End.Line, Column: span.End.Column},
	}, source)
}

func rangeFromPosition(pos lang.Position, source string) lspRange {
	if pos.Line <= 0 {
		return lspRange{
			Start: position{Line: 0, Character: 0},
			End:   position{Line: 0, Character: 1},
		}
	}

	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	end := character + 1
	if end > lineLength {
		end = character
	}
	return lspRange{
		Start: position{Line: lineIndex, Character: character},
		End:   position{Line: lineIndex, Character: end},
	}
}

func positionFromLangPosition(pos lang.Position, source string) position {
	if pos.Line <= 0 {
		return position{Line: 0, Character: 0}
	}
	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	return position{Line: lineIndex, Character: character}
}

func fullRange(text string) lspRange {
	lines := strings.Split(text, "\n")
	lastLine := len(lines) - 1
	return lspRange{
		Start: position{Line: 0, Character: 0},
		End: position{
			Line:      lastLine,
			Character: utf16Length(lines[lastLine]),
		},
	}
}

func utf16Column(line string, oneBasedColumn int) int {
	if oneBasedColumn <= 0 {
		return 0
	}
	runes := []rune(line)
	if oneBasedColumn > len(runes) {
		oneBasedColumn = len(runes)
	}
	return len(utf16.Encode(runes[:oneBasedColumn]))
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func clamp(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
