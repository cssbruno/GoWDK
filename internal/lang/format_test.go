package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatNormalizesTopLevelBlocks(t *testing.T) {
	source := []byte("page home\n\nroute \"/\"\n\n\nview {\n<h1>GOWDK</h1>\n}\n")
	got := string(Format(source))
	want := "page home\nroute \"/\"\n\nview {\n  <h1>GOWDK</h1>\n}\n"
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

func TestFormatPreservesNestedViewMarkupDepth(t *testing.T) {
	source := []byte(`view {
<main class="shell">
<section class="panel">
<p>FlowBoard</p>
<form g:post={Login}>
<div g:if={Count > 0}>
<label for="email">Email</label>
<input id="email" />
</div>
</form>
</section>
</main>
}
`)
	got := string(Format(source))
	want := `view {
  <main class="shell">
    <section class="panel">
      <p>FlowBoard</p>
      <form g:post={Login}>
        <div g:if={Count > 0}>
          <label for="email">Email</label>
          <input id="email" />
        </div>
      </form>
    </section>
  </main>
}
`
	if got != want {
		t.Fatalf("nested view markup was flattened:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatPreservesLinesLongerThanScannerLimit(t *testing.T) {
	longLine := "<p>" + strings.Repeat("x", 70_000) + "</p>"
	source := []byte("page home\nroute \"/\"\n\nview {\n" + longLine + "\n}\n")
	got := string(Format(source))
	want := "page home\nroute \"/\"\n\nview {\n  " + longLine + "\n}\n"
	if got != want {
		t.Fatalf("long line was truncated or reformatted (got %d bytes, want %d bytes)", len(got), len(want))
	}
}

func TestFormatIndentsSiblingsAfterNestedClose(t *testing.T) {
	source := []byte("go {\nfunc Handler() {\nif ok {\nreturn\n}\nlog()\n}\n}\n")
	got := string(Format(source))
	want := "go {\n  func Handler() {\n    if ok {\n      return\n    }\n    log()\n  }\n}\n"
	if got != want {
		t.Fatalf("unexpected format:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatKeepsElseBranchesAligned(t *testing.T) {
	source := []byte("go {\nif ok {\na()\n} else {\nb()\n}\n}\n")
	got := string(Format(source))
	want := "go {\n  if ok {\n    a()\n  } else {\n    b()\n  }\n}\n"
	if got != want {
		t.Fatalf("unexpected format:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatIgnoresBracesInStrings(t *testing.T) {
	// The brace inside the title string must not open a nesting level; the
	// following route stays at top level. A naive brace count would indent it.
	source := []byte("page home\ntitle \"a { b\"\nroute \"/\"\n")
	got := string(Format(source))
	want := "page home\ntitle \"a { b\"\nroute \"/\"\n"
	if got != want {
		t.Fatalf("brace in string changed indentation:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatIgnoresBracesInComments(t *testing.T) {
	// The unbalanced brace in the comment must not change depth; the sibling
	// statement stays indented inside the block.
	source := []byte("go {\n// closes here }\na()\n}\n")
	got := string(Format(source))
	want := "go {\n  // closes here }\n  a()\n}\n"
	if got != want {
		t.Fatalf("brace in comment changed indentation:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatIgnoresBracesInTemplateLiterals(t *testing.T) {
	source := []byte("client {\nconst t = `a ${x} }`\nrun()\n}\n")
	got := string(Format(source))
	want := "client {\n  const t = `a ${x} }`\n  run()\n}\n"
	if got != want {
		t.Fatalf("brace in template literal changed indentation:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatIsIdempotentForSupportedShapes(t *testing.T) {
	tests := map[string]string{
		"page": `package app

page home


route "/"

view {
<main>
<h1>Home</h1>
</main>
}
`,
		"component": `package components

component Hero

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

page contact
route "/contact"

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

page home
route "/"

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
