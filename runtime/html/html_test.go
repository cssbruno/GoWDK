package html

import "testing"

func TestEscapeEscapesHTMLText(t *testing.T) {
	if got := Escape(`<script>alert("x")</script>`); got != `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;` {
		t.Fatalf("unexpected escaped text: %q", got)
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

func TestClassesJoinsTrimmedNonEmptyTokens(t *testing.T) {
	if got := Classes(" btn ", "", "primary", "  "); got != "btn primary" {
		t.Fatalf("unexpected classes: %q", got)
	}
}
