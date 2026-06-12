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

// TestLexTokenOffsetsSurviveMalformedUTF8 guards against offset drift after an
// invalid byte: []rune turns a malformed byte into a 3-byte U+FFFD, so deriving
// offsets from utf8.RuneLen would push every later token two bytes past its true
// position. Offsets must stay anchored to the original byte buffer.
func TestLexTokenOffsetsSurviveMalformedUTF8(t *testing.T) {
	// "x" then a lone 0xff byte, a newline, then "y": bytes x=0, 0xff=1, \n=2, y=3.
	src := "x\xff\ny"
	buffer := []byte(src)
	tokens, _ := Lex(src)

	for _, token := range tokens {
		// Offset and line/column must agree against the real buffer (OffsetOf
		// ranges the bytes, so it reports true positions even past a bad byte).
		want := source.SourcePosition{Line: token.Pos.Line, Column: token.Pos.Column}
		if off := source.OffsetOf(buffer, want); off != token.Offset {
			t.Fatalf("token %q (kind %s): OffsetOf=%d, token offset=%d", token.Lexeme, token.Kind, off, token.Offset)
		}
	}

	// The trailing valid token must land at byte 3, not 5 (the drifted value).
	var found bool
	for _, token := range tokens {
		if token.Kind == TokenIdentifier && token.Lexeme == "y" {
			found = true
			if token.Offset != 3 {
				t.Fatalf("trailing token y offset = %d, want 3", token.Offset)
			}
		}
	}
	if !found {
		t.Fatal("expected to find the trailing identifier token y")
	}
}
