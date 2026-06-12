package syntax

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

// LineExtent returns the index that ends the logical line starting at from (the
// next newline or EOF) and whether the line contains a block-opening brace.
func LineExtent(tokens []Token, from int) (int, bool) {
	hasBrace := false
	index := from
	for index < len(tokens) && tokens[index].Kind != TokenNewline && tokens[index].Kind != TokenEOF {
		if tokens[index].Kind == TokenLBrace {
			hasBrace = true
		}
		index++
	}
	return index, hasBrace
}

// MatchBrace returns the index of the close brace that balances the first open
// brace at or after from. An unbalanced block recovers to the last token before
// EOF so the scan still terminates.
func MatchBrace(tokens []Token, from int) int {
	depth := 0
	for index := from; index < len(tokens); index++ {
		switch tokens[index].Kind {
		case TokenLBrace:
			depth++
		case TokenRBrace:
			depth--
			if depth == 0 {
				return index
			}
		case TokenEOF:
			if index > from {
				return index - 1
			}
			return index
		}
	}
	return len(tokens) - 1
}

// Unquote trims surrounding double quotes from a string-literal lexeme.
func Unquote(lexeme string) string {
	return strings.Trim(lexeme, "\"")
}

// SourceRange builds a 1-based, end-exclusive Range from start to end, clamping
// a degenerate or empty range to a single column so diagnostics always cover at
// least one character. It returns nil for an unpositioned start.
func SourceRange(start, end Position) *Range {
	if start.Line <= 0 || start.Column <= 0 {
		return nil
	}
	if end.Line <= 0 || end.Column <= 0 || (end.Line == start.Line && end.Column <= start.Column) {
		end = Position{Line: start.Line, Column: start.Column + 1}
	}
	return &Range{Start: start, End: end}
}

// SpanOf builds the source span covering first through last, inclusive.
func SpanOf(first, last Token) source.SourceSpan {
	return source.SourceSpan{
		Start: source.SourcePosition{Line: first.Pos.Line, Column: first.Pos.Column, Offset: first.Offset},
		End: source.SourcePosition{
			Line:   last.Pos.Line,
			Column: last.Pos.Column + len([]rune(last.Lexeme)),
			Offset: last.Offset + len(last.Lexeme),
		},
	}
}
