package lang

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/source"
)

// TestLexTokenOffsetsAreByteAccurate verifies the tokenizer records each token's
// 0-based byte offset and that it stays consistent with the token's line/column
// via the source conversion helpers, including across a multi-byte rune. This is
// the substrate contract the recursive-descent parser (ADR 0010) depends on.
func TestLexTokenOffsetsAreByteAccurate(t *testing.T) {
	// The euro sign is three bytes, so byte offsets and rune columns diverge
	// after it.
	src := "page home\ntitle \"€\"\nroute \"/\"\n"
	tokens, _ := Lex(src)

	buffer := []byte(src)
	for _, token := range tokens {
		if token.Kind == TokenEOF {
			continue
		}
		// The token's recorded byte offset must point at its lexeme in the
		// source buffer.
		if token.Offset < 0 || token.Offset > len(buffer) {
			t.Fatalf("token %q offset %d out of bounds", token.Lexeme, token.Offset)
		}
		if token.Kind != TokenNewline && token.Lexeme != "" {
			got := string(buffer[token.Offset : token.Offset+len(token.Lexeme)])
			if got != token.Lexeme {
				t.Fatalf("token %q at offset %d points at %q", token.Lexeme, token.Offset, got)
			}
		}
		// The byte offset and the line/column must describe the same position.
		want := source.SourcePosition{Line: token.Pos.Line, Column: token.Pos.Column}
		if off := source.OffsetOf(buffer, want); off != token.Offset {
			t.Fatalf("token %q: OffsetOf(line %d,col %d)=%d, token offset=%d",
				token.Lexeme, token.Pos.Line, token.Pos.Column, off, token.Offset)
		}
	}
}
