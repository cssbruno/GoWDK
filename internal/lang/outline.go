package lang

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

// OutlineKind classifies a top-level .gwdk declaration for a document outline.
type OutlineKind string

const (
	OutlineKindPackage   OutlineKind = "package"
	OutlineKindMetadata  OutlineKind = "metadata"
	OutlineKindImport    OutlineKind = "import"
	OutlineKindUse       OutlineKind = "use"
	OutlineKindBlock     OutlineKind = "block"
	OutlineKindEndpoint  OutlineKind = "endpoint"
	OutlineKindComponent OutlineKind = "component"
	OutlineKindPage      OutlineKind = "page"
)

// OutlineSymbol is one entry in a document outline.
type OutlineSymbol struct {
	Kind   OutlineKind
	Name   string
	Detail string
	Span   source.SourceSpan
}

// Outline parses the top-level declaration structure of .gwdk source into a flat
// document outline. It is a recursive-descent pass over the shared tokenizer —
// the first consumer of the ADR 0010 parser direction — and recovers from
// unrecognized lines by skipping to the next line, so a malformed line never
// hides the rest of the outline. Block ranges span to the matching close brace,
// counted over tokens (string literals are single tokens, so braces inside
// strings never miscount).
func Outline(src string) []OutlineSymbol {
	tokens, _ := Lex(src)
	var symbols []OutlineSymbol

	index := 0
	for index < len(tokens) {
		token := tokens[index]
		if token.Kind == TokenEOF {
			break
		}
		if token.Kind == TokenNewline {
			index++
			continue
		}

		lineEnd, hasBrace := lineExtent(tokens, index)
		line := tokens[index:lineEnd]

		if hasBrace {
			closeIndex := matchBrace(tokens, index)
			symbols = append(symbols, OutlineSymbol{
				Kind: OutlineKindBlock,
				Name: blockName(line),
				Span: spanOf(tokens[index], tokens[closeIndex]),
			})
			index = closeIndex + 1
			continue
		}

		if symbol, ok := classifyLine(line); ok {
			symbols = append(symbols, symbol)
		}
		index = lineEnd
	}

	return symbols
}

func blockName(line []Token) string {
	var parts []string
	for _, token := range line {
		if token.Kind == TokenLBrace {
			break
		}
		if token.Kind == TokenIdentifier || token.Kind == TokenMetadata {
			parts = append(parts, token.Lexeme)
		}
	}
	return strings.Join(parts, " ")
}

func classifyLine(line []Token) (OutlineSymbol, bool) {
	first := line[0]
	span := spanOf(first, line[len(line)-1])

	switch {
	case first.Kind == TokenIdentifier && first.Lexeme == "package":
		return OutlineSymbol{Kind: OutlineKindPackage, Name: "package " + nextLexeme(line), Span: span}, true
	case first.Kind == TokenIdentifier && first.Lexeme == "import":
		return OutlineSymbol{Kind: OutlineKindImport, Name: "import", Detail: lineValue(line, 1), Span: span}, true
	case first.Kind == TokenIdentifier && first.Lexeme == "use":
		return OutlineSymbol{Kind: OutlineKindUse, Name: "use " + nextLexeme(line), Detail: lineValue(line, 2), Span: span}, true
	case first.Kind == TokenIdentifier && (first.Lexeme == "act" || first.Lexeme == "api"):
		return OutlineSymbol{Kind: OutlineKindEndpoint, Name: first.Lexeme + " " + nextLexeme(line), Detail: lineValue(line, 2), Span: span}, true
	case first.Kind == TokenMetadata:
		return classifyMetadata(first, line, span), true
	default:
		return OutlineSymbol{}, false
	}
}

func classifyMetadata(first Token, line []Token, span source.SourceSpan) OutlineSymbol {
	name := nextLexeme(line)
	switch first.Lexeme {
	case "component":
		if name != "" {
			return OutlineSymbol{Kind: OutlineKindComponent, Name: "component " + name, Span: span}
		}
	case "page":
		if name != "" {
			return OutlineSymbol{Kind: OutlineKindPage, Name: "page " + name, Span: span}
		}
	}
	return OutlineSymbol{Kind: OutlineKindMetadata, Name: first.Lexeme, Detail: lineValue(line, 1), Span: span}
}

// nextLexeme returns the lexeme of the first identifier or string after the first
// token in the line, unquoted.
func nextLexeme(line []Token) string {
	for index := 1; index < len(line); index++ {
		switch line[index].Kind {
		case TokenIdentifier, TokenText:
			return line[index].Lexeme
		case TokenString:
			return unquote(line[index].Lexeme)
		}
	}
	return ""
}

// lineValue joins the lexemes from position at to the end of the line into a
// short detail string.
func lineValue(line []Token, at int) string {
	var parts []string
	for index := at; index < len(line); index++ {
		lexeme := line[index].Lexeme
		if line[index].Kind == TokenString {
			lexeme = unquote(lexeme)
		}
		if strings.TrimSpace(lexeme) != "" {
			parts = append(parts, lexeme)
		}
	}
	return strings.Join(parts, " ")
}
