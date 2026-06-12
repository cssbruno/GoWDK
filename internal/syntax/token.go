// Package syntax is the leaf lexer + recursive-descent parser for .gwdk source.
// It owns the shared tokenizer (ADR 0010) and the typed declaration parser that
// produces real gwdkast nodes, and it imports neither internal/lang (the
// tooling layer) nor internal/parser (the line-oriented compiler parser). That
// one-directional dependency — tooling and the compiler parser both sit above
// syntax — is what lets internal/parser adopt the recursive-descent parser at
// cutover without an import cycle.
package syntax

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
// offset of the token start in the source, the exact substrate the
// recursive-descent parser (ADR 0010) uses to build spans without re-deriving
// positions from line/column.
type Token struct {
	Kind   TokenKind
	Lexeme string
	Pos    Position
	Offset int
}

// Position is a 1-based source location. The tooling layer (internal/lang)
// re-exports it as lang.Position so editor diagnostics and the lexer share one
// position type.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Range is a 1-based source range. End is exclusive.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
