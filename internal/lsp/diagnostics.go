package lsp

import (
	"strings"
	"unicode/utf16"

	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/source"
)

func diagnosticFromLang(item lang.Diagnostic, uri string, body string) diagnostic {
	severity := diagnosticSeverityError
	if item.Severity == "warning" {
		severity = diagnosticSeverityWarning
	}
	return diagnostic{
		Range:              rangeFromLangDiagnostic(item, body),
		Severity:           severity,
		Code:               item.Code,
		Source:             "gowdk",
		Message:            lang.RedactMessage(item.Message),
		RelatedInformation: relatedInformationFromLang(item.Related, uri, body),
	}
}

// relatedInformationFromLang maps a diagnostic's secondary locations to LSP
// relatedInformation. The current single-document check surfaces same-file
// conflicts, so ranges are computed against body and the document uri is used.
func relatedInformationFromLang(related []lang.RelatedLocation, uri string, body string) []diagnosticRelatedInformation {
	if len(related) == 0 {
		return nil
	}
	information := make([]diagnosticRelatedInformation, 0, len(related))
	for _, item := range related {
		rng := rangeFromPosition(item.Pos, body)
		if item.Range != nil {
			rng = rangeFromLangRange(*item.Range, body)
		}
		information = append(information, diagnosticRelatedInformation{
			Location: location{URI: uri, Range: rng},
			Message:  lang.RedactMessage(item.Message),
		})
	}
	return information
}

func rangeFromLangDiagnostic(item lang.Diagnostic, body string) lspRange {
	if item.Range != nil {
		return rangeFromLangRange(*item.Range, body)
	}
	return rangeFromPosition(item.Pos, body)
}

func rangeFromLangRange(item lang.Range, body string) lspRange {
	start := positionFromLangPosition(item.Start, body)
	end := positionFromLangPosition(item.End, body)
	if end.Line < start.Line || (end.Line == start.Line && end.Character <= start.Character) {
		end = position{Line: start.Line, Character: start.Character + 1}
	}
	return lspRange{Start: start, End: end}
}

func lspRangeFromSourceSpan(span source.SourceSpan, body string) lspRange {
	return rangeFromLangRange(lang.Range{
		Start: lang.Position{Line: span.Start.Line, Column: span.Start.Column},
		End:   lang.Position{Line: span.End.Line, Column: span.End.Column},
	}, body)
}

func rangeFromPosition(pos lang.Position, body string) lspRange {
	if pos.Line <= 0 {
		return lspRange{
			Start: position{Line: 0, Character: 0},
			End:   position{Line: 0, Character: 1},
		}
	}

	lines := strings.Split(body, "\n")
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

func positionFromLangPosition(pos lang.Position, body string) position {
	if pos.Line <= 0 {
		return position{Line: 0, Character: 0}
	}
	lines := strings.Split(body, "\n")
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
