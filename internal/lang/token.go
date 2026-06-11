package lang

// TokenKind identifies one lexical token.
type TokenKind string

const (
	TokenIllegal    TokenKind = "illegal"
	TokenEOF        TokenKind = "eof"
	TokenNewline    TokenKind = "newline"
	TokenMetadata   TokenKind = "metadata"
	TokenIdentifier TokenKind = "identifier"
	TokenString     TokenKind = "string"
	TokenLBrace     TokenKind = "lbrace"
	TokenRBrace     TokenKind = "rbrace"
	TokenComma      TokenKind = "comma"
	TokenColon      TokenKind = "colon"
	TokenQuestion   TokenKind = "question"
	TokenArrow      TokenKind = "arrow"
	TokenText       TokenKind = "text"
)

// Token is a lexical token with source location.
type Token struct {
	Kind   TokenKind
	Lexeme string
	Pos    Position
}
