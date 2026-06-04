package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if diagnostics[0].Range == nil {
		t.Fatalf("expected diagnostic range: %#v", diagnostics[0])
	}
	if diagnostics[0].Range.Start.Line != 1 || diagnostics[0].Range.Start.Column != 8 ||
		diagnostics[0].Range.End.Line != 1 || diagnostics[0].Range.End.Column != 21 {
		t.Fatalf("unexpected diagnostic range: %#v", diagnostics[0].Range)
	}
}

func TestLexGoldenGrammarFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.FromSlash("testdata/grammar_golden/page.gwdk"))
	if err != nil {
		t.Fatal(err)
	}
	tokens, diagnostics := Lex(string(source))
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}

	var rows []string
	for _, token := range tokens {
		rows = append(rows, fmt.Sprintf("%d:%d\t%s\t%q", token.Pos.Line, token.Pos.Column, token.Kind, token.Lexeme))
	}
	actual := strings.Join(rows, "\n")
	expected, err := os.ReadFile(filepath.FromSlash("testdata/grammar_golden/tokens.golden.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if actual != strings.TrimSpace(string(expected)) {
		t.Fatalf("token golden mismatch\nexpected:\n%s\nactual:\n%s", expected, actual)
	}
}
