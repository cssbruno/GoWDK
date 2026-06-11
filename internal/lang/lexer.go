package lang

import "unicode"

// Lex tokenizes .gwdk source for editor and CLI tooling.
func Lex(source string) ([]Token, Diagnostics) {
	lexer := scanner{
		source: []rune(source),
		line:   1,
		column: 1,
	}
	return lexer.scan()
}

type scanner struct {
	source []rune
	index  int
	line   int
	column int
}

func (scanner *scanner) scan() ([]Token, Diagnostics) {
	var tokens []Token
	var diagnostics Diagnostics

	for !scanner.done() {
		ch := scanner.peek()
		pos := scanner.position()

		switch {
		case ch == '\r':
			scanner.advance()
		case ch == '\n':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenNewline, Lexeme: "\n", Pos: pos})
		case unicode.IsSpace(ch):
			scanner.advance()
		case ch == '/' && scanner.peekNext() == '/':
			scanner.skipLineComment()
		case isIdentStart(ch):
			tokens = append(tokens, scanner.identifier())
		case ch == '"':
			token, diagnostic := scanner.quotedString()
			tokens = append(tokens, token)
			if diagnostic.Message != "" {
				diagnostics = append(diagnostics, diagnostic)
			}
		case ch == '{':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenLBrace, Lexeme: "{", Pos: pos})
		case ch == '}':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenRBrace, Lexeme: "}", Pos: pos})
		case ch == ',':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenComma, Lexeme: ",", Pos: pos})
		case ch == ':':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenColon, Lexeme: ":", Pos: pos})
		case ch == '?':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenQuestion, Lexeme: "?", Pos: pos})
		case ch == '=' && scanner.peekNext() == '>':
			scanner.advance()
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenArrow, Lexeme: "=>", Pos: pos})
		default:
			tokens = append(tokens, scanner.text())
		}
	}

	tokens = append(tokens, Token{Kind: TokenEOF, Pos: scanner.position()})
	return tokens, diagnostics
}

func (scanner *scanner) identifier() Token {
	pos := scanner.position()
	start := scanner.index
	for !scanner.done() && (isIdentPart(scanner.peek()) || scanner.peek() == '.' || scanner.peek() == '-') {
		scanner.advance()
	}
	lexeme := string(scanner.source[start:scanner.index])
	if scanner.isLineLeading(start) && isMetadataLexeme(lexeme) {
		return Token{Kind: TokenMetadata, Lexeme: lexeme, Pos: pos}
	}
	return Token{Kind: TokenIdentifier, Lexeme: lexeme, Pos: pos}
}

func (scanner *scanner) quotedString() (Token, Diagnostic) {
	pos := scanner.position()
	start := scanner.index
	scanner.advance()
	for !scanner.done() {
		ch := scanner.peek()
		if ch == '\\' {
			scanner.advance()
			if !scanner.done() {
				scanner.advance()
			}
			continue
		}
		if ch == '"' {
			scanner.advance()
			return Token{Kind: TokenString, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos}, Diagnostic{}
		}
		if ch == '\n' {
			break
		}
		scanner.advance()
	}
	return Token{Kind: TokenIllegal, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos}, Diagnostic{
		Pos:      pos,
		Range:    sourceRange(pos, scanner.position()),
		Code:     "unterminated_string",
		Severity: "error",
		Message:  "unterminated string literal",
	}
}

func sourceRange(start, end Position) *Range {
	if start.Line <= 0 || start.Column <= 0 {
		return nil
	}
	if end.Line <= 0 || end.Column <= 0 || (end.Line == start.Line && end.Column <= start.Column) {
		end = Position{Line: start.Line, Column: start.Column + 1}
	}
	return &Range{Start: start, End: end}
}

func (scanner *scanner) text() Token {
	pos := scanner.position()
	start := scanner.index
	for !scanner.done() {
		ch := scanner.peek()
		if unicode.IsSpace(ch) || ch == '"' || ch == '{' || ch == '}' || ch == ',' || ch == ':' || ch == '?' || (ch == '=' && scanner.peekNext() == '>') {
			break
		}
		if ch == '/' && scanner.peekNext() == '/' {
			break
		}
		scanner.advance()
	}
	return Token{Kind: TokenText, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos}
}

func (scanner *scanner) skipLineComment() {
	for !scanner.done() && scanner.peek() != '\n' {
		scanner.advance()
	}
}

func (scanner *scanner) done() bool {
	return scanner.index >= len(scanner.source)
}

func (scanner *scanner) peek() rune {
	if scanner.done() {
		return 0
	}
	return scanner.source[scanner.index]
}

func (scanner *scanner) peekNext() rune {
	if scanner.index+1 >= len(scanner.source) {
		return 0
	}
	return scanner.source[scanner.index+1]
}

func (scanner *scanner) advance() rune {
	ch := scanner.source[scanner.index]
	scanner.index++
	if ch == '\n' {
		scanner.line++
		scanner.column = 1
	} else {
		scanner.column++
	}
	return ch
}

func (scanner *scanner) position() Position {
	return Position{Line: scanner.line, Column: scanner.column}
}

func isIdentStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isIdentPart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

func (scanner *scanner) isLineLeading(start int) bool {
	for index := start - 1; index >= 0; index-- {
		switch scanner.source[index] {
		case '\n', '\r':
			return true
		case ' ', '\t':
			continue
		default:
			return false
		}
	}
	return true
}

func isMetadataLexeme(value string) bool {
	return IsMetadataKeyword(value)
}
