package clientlang

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenString
	tokenNumber
	tokenBool
	tokenNil
	tokenOp
	tokenLParen
	tokenRParen
	tokenDot
	tokenLBracket
	tokenRBracket
	tokenLBrace
	tokenRBrace
	tokenComma
)

type exprToken struct {
	kind  tokenKind
	value string
	start int
	end   int
}

type exprLexer struct {
	source []rune
	index  int
}

func newExprLexer(source string) *exprLexer {
	return &exprLexer{source: []rune(source)}
}

func (lexer *exprLexer) next() (exprToken, error) {
	lexer.skipSpace()
	if lexer.index >= len(lexer.source) {
		return exprToken{kind: tokenEOF, start: lexer.index, end: lexer.index}, nil
	}
	char := lexer.source[lexer.index]
	switch {
	case isExprIdentStart(char):
		return lexer.ident(), nil
	case unicode.IsDigit(char):
		return lexer.number(), nil
	case char == '"':
		return lexer.string()
	case char == '(':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenLParen, value: "(", start: start, end: lexer.index}, nil
	case char == ')':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenRParen, value: ")", start: start, end: lexer.index}, nil
	case char == '.':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenDot, value: ".", start: start, end: lexer.index}, nil
	case char == '[':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenLBracket, value: "[", start: start, end: lexer.index}, nil
	case char == ']':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenRBracket, value: "]", start: start, end: lexer.index}, nil
	case char == '{':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenLBrace, value: "{", start: start, end: lexer.index}, nil
	case char == '}':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenRBrace, value: "}", start: start, end: lexer.index}, nil
	case char == ',':
		start := lexer.index
		lexer.index++
		return exprToken{kind: tokenComma, value: ",", start: start, end: lexer.index}, nil
	default:
		return lexer.operator()
	}
}

func (lexer *exprLexer) skipSpace() {
	for lexer.index < len(lexer.source) && unicode.IsSpace(lexer.source[lexer.index]) {
		lexer.index++
	}
}

func (lexer *exprLexer) ident() exprToken {
	start := lexer.index
	for lexer.index < len(lexer.source) && isExprIdentPart(lexer.source[lexer.index]) {
		lexer.index++
	}
	value := string(lexer.source[start:lexer.index])
	switch value {
	case "true", "false":
		return exprToken{kind: tokenBool, value: value, start: start, end: lexer.index}
	case "nil":
		return exprToken{kind: tokenNil, value: value, start: start, end: lexer.index}
	default:
		return exprToken{kind: tokenIdent, value: value, start: start, end: lexer.index}
	}
}

func (lexer *exprLexer) number() exprToken {
	start := lexer.index
	for lexer.index < len(lexer.source) && unicode.IsDigit(lexer.source[lexer.index]) {
		lexer.index++
	}
	if lexer.index < len(lexer.source) && lexer.source[lexer.index] == '.' {
		lexer.index++
		for lexer.index < len(lexer.source) && unicode.IsDigit(lexer.source[lexer.index]) {
			lexer.index++
		}
	}
	return exprToken{kind: tokenNumber, value: string(lexer.source[start:lexer.index]), start: start, end: lexer.index}
}

func (lexer *exprLexer) string() (exprToken, error) {
	start := lexer.index
	lexer.index++
	escaped := false
	for lexer.index < len(lexer.source) {
		char := lexer.source[lexer.index]
		lexer.index++
		if escaped {
			escaped = false
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '"':
			value := string(lexer.source[start:lexer.index])
			if _, err := strconv.Unquote(value); err != nil {
				return exprToken{}, err
			}
			return exprToken{kind: tokenString, value: value, start: start, end: lexer.index}, nil
		}
	}
	return exprToken{}, fmt.Errorf("unterminated string")
}

func (lexer *exprLexer) operator() (exprToken, error) {
	remaining := string(lexer.source[lexer.index:])
	for _, op := range []string{"==", "!=", "<=", ">=", "&&", "||", "+", "-", "*", "/", "%", "!", "<", ">"} {
		if strings.HasPrefix(remaining, op) {
			start := lexer.index
			lexer.index += len([]rune(op))
			return exprToken{kind: tokenOp, value: op, start: start, end: lexer.index}, nil
		}
	}
	return exprToken{}, fmt.Errorf("unexpected character %q", lexer.source[lexer.index])
}

func isExprIdentStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isExprIdentPart(char rune) bool {
	return isExprIdentStart(char) || unicode.IsDigit(char)
}
