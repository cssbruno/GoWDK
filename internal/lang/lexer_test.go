package lang

import "testing"

func TestLexRecognizesGWDKLanguageTokens(t *testing.T) {
	tokens, diagnostics := Lex(`@page home
@route "/"
view {
  <h1 .text-4xl>GOWDK</h1>
}
`)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}

	want := []TokenKind{
		TokenAnnotation,
		TokenIdentifier,
		TokenNewline,
		TokenAnnotation,
		TokenString,
		TokenNewline,
		TokenIdentifier,
		TokenLBrace,
	}
	for i, kind := range want {
		if tokens[i].Kind != kind {
			t.Fatalf("token %d: expected %s, got %s (%q)", i, kind, tokens[i].Kind, tokens[i].Lexeme)
		}
	}
}

func TestLexReportsUnterminatedString(t *testing.T) {
	_, diagnostics := Lex("@route \"unterminated\n")
	if !diagnostics.HasErrors() {
		t.Fatal("expected unterminated string diagnostic")
	}
	if diagnostics[0].Pos.Line != 1 || diagnostics[0].Pos.Column != 8 {
		t.Fatalf("unexpected diagnostic position: %#v", diagnostics[0].Pos)
	}
}
