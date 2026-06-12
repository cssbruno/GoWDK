package lang

import (
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/syntax"
)

// This file re-exports the leaf internal/syntax package (the shared tokenizer
// and the ADR 0010 recursive-descent parser) under the lang namespace. The
// lexer and parser moved down into a leaf so internal/parser can adopt the
// recursive-descent parser at cutover without the import cycle that would
// otherwise form (parser -> lang -> parser, since lang's tooling imports
// parser). Tooling and tests keep using lang.Lex / lang.Token* / lang.Position
// unchanged; the rich Diagnostic type and message redaction stay in lang.

// Token and position types are aliases, so lang.Token and syntax.Token are the
// same type and values cross the boundary without conversion.
type (
	Token     = syntax.Token
	TokenKind = syntax.TokenKind
	Position  = syntax.Position
	Range     = syntax.Range
	TopLevel  = syntax.TopLevel
)

const (
	TokenIllegal    = syntax.TokenIllegal
	TokenEOF        = syntax.TokenEOF
	TokenNewline    = syntax.TokenNewline
	TokenMetadata   = syntax.TokenMetadata
	TokenIdentifier = syntax.TokenIdentifier
	TokenString     = syntax.TokenString
	TokenLBrace     = syntax.TokenLBrace
	TokenRBrace     = syntax.TokenRBrace
	TokenComma      = syntax.TokenComma
	TokenColon      = syntax.TokenColon
	TokenAssign     = syntax.TokenAssign
	TokenQuestion   = syntax.TokenQuestion
	TokenArrow      = syntax.TokenArrow
	TokenText       = syntax.TokenText
)

// MetadataKeywords and IsMetadataKeyword re-export the single source of truth
// the lexer and formatter share.
var MetadataKeywords = syntax.MetadataKeywords

// IsMetadataKeyword reports whether value is a top-level metadata keyword.
func IsMetadataKeyword(value string) bool {
	return syntax.IsMetadataKeyword(value)
}

// ParseTopLevel runs the recursive-descent declaration parser.
func ParseTopLevel(src string) TopLevel {
	return syntax.ParseTopLevel(src)
}

// Lex tokenizes .gwdk source for tooling, mapping each leaf-level LexError into a
// rich lang.Diagnostic (the error severity, redaction, and JSON marshaling stay
// in lang).
func Lex(source string) ([]Token, Diagnostics) {
	tokens, lexErrors := syntax.Lex(source)
	if len(lexErrors) == 0 {
		return tokens, nil
	}
	diagnostics := make(Diagnostics, 0, len(lexErrors))
	for _, lexError := range lexErrors {
		diagnostics = append(diagnostics, Diagnostic{
			Pos:      lexError.Pos,
			Range:    lexError.Range,
			Code:     lexError.Code,
			Severity: "error",
			Message:  lexError.Message,
		})
	}
	return tokens, diagnostics
}

// The token-stream helpers below are shared by the leaf parser and the outline
// tooling that stays in lang; lang keeps short lowercase wrappers so the outline
// reads the same as before the extraction.

func lineExtent(tokens []Token, from int) (int, bool) {
	return syntax.LineExtent(tokens, from)
}

func matchBrace(tokens []Token, from int) int {
	return syntax.MatchBrace(tokens, from)
}

func unquote(lexeme string) string {
	return syntax.Unquote(lexeme)
}

func spanOf(first, last Token) source.SourceSpan {
	return syntax.SpanOf(first, last)
}

func sourceRange(start, end Position) *Range {
	return syntax.SourceRange(start, end)
}
