package syntax

import "unicode"

// LexError is a lexer-level finding (currently only unterminated string
// literals). It carries the position substrate the tooling layer needs; the
// richer Diagnostic type — severity, fixes, redaction, JSON — lives in
// internal/lang, which maps each LexError into a lang.Diagnostic in its Lex
// wrapper. Keeping the lexer free of that machinery is what keeps this package a
// leaf.
type LexError struct {
	Pos     Position
	Range   *Range
	Code    string
	Message string
}

// Lex tokenizes .gwdk source for the parser, editor, and CLI tooling.
func Lex(source string) ([]Token, []LexError) {
	runes := []rune(source)
	// byteOffsets[i] is the 0-based byte offset of rune i in the original
	// source; the final entry is the total byte length. Offsets are taken from
	// ranging the original string (which reports true byte positions) rather
	// than summing utf8.RuneLen, so malformed UTF-8 — where []rune turns each
	// bad byte into a 3-byte U+FFFD — does not drift token offsets.
	byteOffsets := make([]int, len(runes)+1)
	runeIndex := 0
	for byteIndex := range source {
		byteOffsets[runeIndex] = byteIndex
		runeIndex++
	}
	byteOffsets[len(runes)] = len(source)

	lexer := scanner{
		source:      runes,
		byteOffsets: byteOffsets,
		line:        1,
		column:      1,
	}
	return lexer.scan()
}

type scanner struct {
	source      []rune
	byteOffsets []int
	index       int
	line        int
	column      int
}

// offset returns the 0-based byte offset of the current rune in the original
// source.
func (scanner *scanner) offset() int {
	if scanner.index < len(scanner.byteOffsets) {
		return scanner.byteOffsets[scanner.index]
	}
	return scanner.byteOffsets[len(scanner.byteOffsets)-1]
}

func (scanner *scanner) scan() ([]Token, []LexError) {
	var tokens []Token
	var errors []LexError

	for !scanner.done() {
		ch := scanner.peek()
		pos := scanner.position()
		offset := scanner.offset()

		switch {
		case ch == '\r':
			scanner.advance()
		case ch == '\n':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenNewline, Lexeme: "\n", Pos: pos, Offset: offset})
		case unicode.IsSpace(ch):
			scanner.advance()
		case ch == '/' && scanner.peekNext() == '/':
			scanner.skipLineComment()
		case isIdentStart(ch):
			tokens = append(tokens, scanner.identifier())
		case ch == '"':
			token, lexError := scanner.quotedString()
			tokens = append(tokens, token)
			if lexError.Message != "" {
				errors = append(errors, lexError)
			}
		case ch == '{':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenLBrace, Lexeme: "{", Pos: pos, Offset: offset})
		case ch == '}':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenRBrace, Lexeme: "}", Pos: pos, Offset: offset})
		case ch == ',':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenComma, Lexeme: ",", Pos: pos, Offset: offset})
		case ch == ':':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenColon, Lexeme: ":", Pos: pos, Offset: offset})
		case ch == '?':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenQuestion, Lexeme: "?", Pos: pos, Offset: offset})
		case ch == '=' && scanner.peekNext() == '>':
			scanner.advance()
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenArrow, Lexeme: "=>", Pos: pos, Offset: offset})
		case ch == '=':
			scanner.advance()
			tokens = append(tokens, Token{Kind: TokenAssign, Lexeme: "=", Pos: pos, Offset: offset})
		default:
			tokens = append(tokens, scanner.text())
		}
	}

	tokens = append(tokens, Token{Kind: TokenEOF, Pos: scanner.position(), Offset: scanner.offset()})
	return tokens, errors
}

func (scanner *scanner) identifier() Token {
	pos := scanner.position()
	offset := scanner.offset()
	start := scanner.index
	for !scanner.done() && (isIdentPart(scanner.peek()) || scanner.peek() == '.' || scanner.peek() == '-') {
		scanner.advance()
	}
	lexeme := string(scanner.source[start:scanner.index])
	if scanner.isLineLeading(start) && isMetadataLexeme(lexeme) {
		return Token{Kind: TokenMetadata, Lexeme: lexeme, Pos: pos, Offset: offset}
	}
	return Token{Kind: TokenIdentifier, Lexeme: lexeme, Pos: pos, Offset: offset}
}

func (scanner *scanner) quotedString() (Token, LexError) {
	pos := scanner.position()
	offset := scanner.offset()
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
			return Token{Kind: TokenString, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos, Offset: offset}, LexError{}
		}
		if ch == '\n' {
			break
		}
		scanner.advance()
	}
	return Token{Kind: TokenIllegal, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos, Offset: offset}, LexError{
		Pos:     pos,
		Range:   SourceRange(pos, scanner.position()),
		Code:    "unterminated_string",
		Message: "unterminated string literal",
	}
}

func (scanner *scanner) text() Token {
	pos := scanner.position()
	offset := scanner.offset()
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
	return Token{Kind: TokenText, Lexeme: string(scanner.source[start:scanner.index]), Pos: pos, Offset: offset}
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

func (scanner *scanner) advance() {
	ch := scanner.source[scanner.index]
	scanner.index++
	if ch == '\n' {
		scanner.line++
		scanner.column = 1
	} else {
		scanner.column++
	}
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
