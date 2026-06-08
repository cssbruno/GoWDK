package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatNormalizesTopLevelBlocks(t *testing.T) {
	source := []byte("@page home\n\n@route \"/\"\n\n\nview {\n<h1>GOWDK</h1>\n}\n")
	got := string(Format(source))
	want := "@page home\n@route \"/\"\n\nview {\n  <h1>GOWDK</h1>\n}\n"
	if got != want {
		t.Fatalf("unexpected format:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatGoldenPreservesCommentsAndNestedMarkup(t *testing.T) {
	source, err := os.ReadFile(filepath.FromSlash("testdata/format_golden/input.gwdk"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.FromSlash("testdata/format_golden/expected.gwdk"))
	if err != nil {
		t.Fatal(err)
	}

	if got := string(Format(source)); got != string(expected) {
		t.Fatalf("format golden mismatch\nexpected:\n%s\nactual:\n%s", expected, got)
	}
}

func TestFormatIsIdempotentForSupportedShapes(t *testing.T) {
	tests := map[string]string{
		"page": `package app

@page home


@route "/"

view {
<main>
<h1>Home</h1>
</main>
}
`,
		"component": `package components

@component Hero

props {
title string
}

// Keep comments attached to the next block.
view {
<section>
<h1>{title}</h1>
</section>
}
`,
		"endpoints": `package app

@page contact
@route "/contact"

act Submit POST "/contact"
api Status GET "/api/status"

view {
<main>
<form g:post={Submit}>
<button>Send</button>
</form>
</main>
}
`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			once := Format([]byte(source))
			twice := Format(once)
			if string(twice) != string(once) {
				t.Fatalf("format is not idempotent\nonce:\n%s\ntwice:\n%s", once, twice)
			}
		})
	}
}

func TestFormatDoesNotHideParserDiagnostics(t *testing.T) {
	source := []byte(`package app

@page home
@route "/"

act submit {
}

view {
<main>Home</main>
}
`)
	formatted := Format(source)
	_, diagnostics := ParseSource("bad.page.gwdk", formatted)
	if !diagnostics.HasErrors() {
		t.Fatalf("expected parser diagnostic after formatting:\n%s", formatted)
	}
	if diagnostics[0].Code != "old_action_block_syntax" {
		t.Fatalf("expected old_action_block_syntax after formatting, got %#v", diagnostics[0])
	}
	if !strings.Contains(diagnostics[0].Message, "old action block syntax") {
		t.Fatalf("expected old action migration message, got %#v", diagnostics[0])
	}
}
