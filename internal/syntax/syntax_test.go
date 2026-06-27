package syntax

import "testing"

func TestLexEmitsTokensAndOffsets(t *testing.T) {
	tokens, errs := Lex("page home\nstore Cart cart.Cart = cart.NewCart()\n")
	if len(errs) != 0 {
		t.Fatalf("unexpected lex errors: %#v", errs)
	}
	if tokens[0].Kind != TokenMetadata || tokens[0].Lexeme != "page" {
		t.Fatalf("first token = %s %q, want metadata page", tokens[0].Kind, tokens[0].Lexeme)
	}
	if tokens[0].Offset != 0 {
		t.Fatalf("page offset = %d, want 0", tokens[0].Offset)
	}
	var assigns, arrows int
	for _, token := range tokens {
		switch token.Kind {
		case TokenAssign:
			assigns++
		case TokenArrow:
			arrows++
		}
	}
	if assigns != 1 || arrows != 0 {
		t.Fatalf("assign=%d arrow=%d, want 1 and 0", assigns, arrows)
	}
}

func TestLexReportsUnterminatedStringAsLexError(t *testing.T) {
	_, errs := Lex("route \"unterminated\n")
	if len(errs) != 1 {
		t.Fatalf("got %d lex errors, want 1", len(errs))
	}
	if errs[0].Code != "unterminated_string" {
		t.Fatalf("code = %q, want unterminated_string", errs[0].Code)
	}
	if errs[0].Pos.Line != 1 || errs[0].Pos.Column != 7 {
		t.Fatalf("pos = %#v, want 1:7", errs[0].Pos)
	}
	if errs[0].Range == nil {
		t.Fatalf("expected a range on the lex error")
	}
}

func TestLineExtentAndMatchBrace(t *testing.T) {
	tokens, _ := Lex("view {\n  x\n}\n")
	end, hasBrace := LineExtent(tokens, 0)
	if !hasBrace {
		t.Fatalf("LineExtent did not see the block-opening brace")
	}
	if tokens[end].Kind != TokenNewline {
		t.Fatalf("LineExtent stopped at %s, want newline", tokens[end].Kind)
	}
	closeIndex := MatchBrace(tokens, 0)
	if tokens[closeIndex].Kind != TokenRBrace {
		t.Fatalf("MatchBrace landed on %s, want rbrace", tokens[closeIndex].Kind)
	}
}

func TestParseTopLevelRecoversDeclarations(t *testing.T) {
	top := ParseTopLevel("package home\nimport \"fmt\"\nuse ui \"ui\"\npage index\nact Submit POST \"/submit\"\n")
	if top.Package == nil || top.Package.Name != "home" {
		t.Fatalf("package = %#v, want home", top.Package)
	}
	if len(top.Imports) != 1 || top.Imports[0].Path != "fmt" {
		t.Fatalf("imports = %#v, want one fmt import", top.Imports)
	}
	if len(top.Uses) != 1 || top.Uses[0].Alias != "ui" {
		t.Fatalf("uses = %#v, want one ui use", top.Uses)
	}
	if top.Page == nil || top.Page.ID != "index" {
		t.Fatalf("page = %#v, want index", top.Page)
	}
	if len(top.Actions) != 1 || top.Actions[0].Name != "Submit" || top.Actions[0].Method != "POST" {
		t.Fatalf("actions = %#v, want Submit POST", top.Actions)
	}
}
