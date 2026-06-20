package html

import (
	"strings"
	"testing"
)

func TestEscapeEscapesHTMLText(t *testing.T) {
	if got := Escape(`<script>alert("x")</script>`); got != `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;` {
		t.Fatalf("unexpected escaped text: %q", got)
	}
}

func TestEscapeURLEncodesBeforeHTMLEscaping(t *testing.T) {
	if got := EscapeURL(`\\evil.example/x?q="bad"&ok=1`); got != `%5C%5Cevil.example%2Fx%3Fq=%22bad%22&amp;ok=1` {
		t.Fatalf("unexpected escaped URL segment: %q", got)
	}
}

func TestAttrOmitsEmptyValuesAndEscapesNonEmptyValues(t *testing.T) {
	if got := Attr("href", ""); got != "" {
		t.Fatalf("expected empty attr to be omitted, got %q", got)
	}
	if got := Attr("href", `/search?q=go&sort="new"`); got != ` href="/search?q=go&amp;sort=&#34;new&#34;"` {
		t.Fatalf("unexpected attr: %q", got)
	}
}

func TestAttrEscapesJSONAttributeValues(t *testing.T) {
	got := Attr("data-gowdk-state", `{"name":"hero","quote":"\"","html":"</gowdk-island>","amp":"&"}`)
	const prefix = ` data-gowdk-state="`
	if !strings.HasPrefix(got, prefix) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("unexpected attr shape: %q", got)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(got, prefix), `"`)
	if strings.Contains(inner, `"`) {
		t.Fatalf("attribute value contains an unescaped double quote: %q", got)
	}
	for _, escaped := range []string{`&#34;name&#34;`, `&lt;/gowdk-island&gt;`, `&amp;`} {
		if !strings.Contains(inner, escaped) {
			t.Fatalf("attribute value is missing escaped fragment %q: %q", escaped, got)
		}
	}
}

func TestAttrRejectsUnsafeAttributeNames(t *testing.T) {
	for _, name := range []string{
		``,
		`href" onclick="alert(1)`,
		`data-gowdk-state onmouseover`,
		`data-gowdk-state=bad`,
	} {
		if got := Attr(name, "value"); got != "" {
			t.Fatalf("expected unsafe attr name %q to be omitted, got %q", name, got)
		}
	}
	if got := Attr("data-gowdk-parent-on-submit", "Save()"); got == "" {
		t.Fatal("expected generated data attribute name to be accepted")
	}
}

func TestClassesJoinsTrimmedNonEmptyTokens(t *testing.T) {
	if got := Classes(" btn ", "", "primary", "  "); got != "btn primary" {
		t.Fatalf("unexpected classes: %q", got)
	}
}
