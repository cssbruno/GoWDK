package parser

import (
	"strings"
	"testing"
)

func TestBraceScannerSkipsStringsAndComments(t *testing.T) {
	tests := []struct {
		name string
		lang braceLang
		line string
		want int
	}{
		{"go open", braceLangGo, "func Greet() string {", 1},
		{"go brace in string", braceLangGo, `func Greet() string { return "}" }`, 0},
		{"go brace in rune", braceLangGo, `if c == '}' {`, 1},
		{"go brace in line comment", braceLangGo, `x := 1 // closes with }`, 0},
		{"go brace in block comment", braceLangGo, `y := 2 /* } } */ + 3`, 0},
		{"js brace in single quotes", braceLangJS, `const s = '}'`, 0},
		{"js one-liner balanced", braceLangJS, `fn Inc() { Count++ }`, 0},
		{"js else", braceLangJS, `} else {`, 0},
		{"css brace in string", braceLangCSS, `content: "}";`, 0},
		{"css no line comment", braceLangCSS, `a { // not a comment }`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scanner := braceScanner{lang: test.lang}
			if got := scanner.delta(test.line); got != test.want {
				t.Fatalf("delta(%q) = %d, want %d", test.line, got, test.want)
			}
		})
	}
}

func TestBraceScannerCarriesMultilineState(t *testing.T) {
	scanner := braceScanner{lang: braceLangGo}
	// Opening a block comment that contains a lone "}" on its own line must not
	// be counted, and inMultiline must report the open comment between lines.
	if got := scanner.delta("x := 1 /* start"); got != 0 {
		t.Fatalf("line 1 delta = %d, want 0", got)
	}
	if !scanner.inMultiline() {
		t.Fatal("expected scanner to be inside a block comment after line 1")
	}
	if got := scanner.delta("}"); got != 0 {
		t.Fatalf("brace inside comment counted: delta = %d, want 0", got)
	}
	if got := scanner.delta("end */"); got != 0 {
		t.Fatalf("line 3 delta = %d, want 0", got)
	}
	if scanner.inMultiline() {
		t.Fatal("expected block comment to be closed after line 3")
	}
}

func TestBraceScannerRawStringSpansLines(t *testing.T) {
	scanner := braceScanner{lang: braceLangGo}
	if got := scanner.delta("s := `start {"); got != 0 {
		t.Fatalf("raw string open delta = %d, want 0", got)
	}
	if !scanner.inMultiline() {
		t.Fatal("expected scanner inside raw string")
	}
	if got := scanner.delta("still } in string`"); got != 0 {
		t.Fatalf("raw string close delta = %d, want 0", got)
	}
}

func TestParseSyntaxGoBlockWithBraceInString(t *testing.T) {
	file, err := ParseSyntax([]byte("go {\n\tfunc Greet() string { return \"}\" }\n}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(file.Blocks) != 1 || file.Blocks[0].Kind != "go" {
		t.Fatalf("expected one go block, got %#v", file.Blocks)
	}
	if !strings.Contains(file.Blocks[0].Body, `return "}"`) {
		t.Fatalf("go block body lost content: %q", file.Blocks[0].Body)
	}
}

func TestParseSyntaxClientBlockOneLinersAndStrings(t *testing.T) {
	file, err := ParseSyntax([]byte("client {\n\tfn Inc() { Count++ }\n\tif x { y() } else { z() }\n\tlet s = \"}\"\n}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var client *SyntaxBlock
	for i := range file.Blocks {
		if file.Blocks[i].Kind == "client" {
			client = &file.Blocks[i]
		}
	}
	if client == nil {
		t.Fatalf("expected a client block, got %#v", file.Blocks)
	}
	if !strings.Contains(client.Body, "} else {") {
		t.Fatalf("client block body lost content: %q", client.Body)
	}
}

func TestParseComponentClientBlockOneLiner(t *testing.T) {
	component, err := ParseComponent([]byte("component Counter\nclient {\n\tfn Inc() { Count++ }\n}\nview {\n\t<div></div>\n}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(component.Blocks.ClientBody, "Count++") {
		t.Fatalf("client body lost content: %q", component.Blocks.ClientBody)
	}
}
