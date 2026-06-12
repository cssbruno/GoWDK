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
	TokenAssign     TokenKind = "assign"
	TokenQuestion   TokenKind = "question"
	TokenArrow      TokenKind = "arrow"
	TokenText       TokenKind = "text"
)

// Token is a lexical token with source location. Offset is the 0-based byte
// offset of the token start in the source, the exact substrate the planned
// recursive-descent parser (ADR 0010) uses to build spans without re-deriving
// positions from line/column.
type Token struct {
	Kind   TokenKind
	Lexeme string
	Pos    Position
	Offset int
}
