package lsp

import "github.com/cssbruno/gowdk/internal/lang"

func semanticTokenType(kind lang.TokenKind) (string, bool) {
	switch kind {
	case lang.TokenMetadata:
		return "decorator", true
	case lang.TokenIdentifier, lang.TokenText:
		return "variable", true
	case lang.TokenString:
		return "string", true
	case lang.TokenLBrace, lang.TokenRBrace, lang.TokenComma, lang.TokenColon, lang.TokenQuestion, lang.TokenArrow:
		return "operator", true
	default:
		return "", false
	}
}
