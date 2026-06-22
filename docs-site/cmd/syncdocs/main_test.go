package main

import (
	"strings"
	"testing"
)

func TestStripFirstH1AndLeadRemovesPromotedLead(t *testing.T) {
	body := stripFirstH1AndLead(`# Language

The promoted lead should appear only in the page header.

## Status

Body content.
`)

	if strings.Contains(body, "promoted lead") {
		t.Fatalf("body still contains promoted lead:\n%s", body)
	}
	if !strings.Contains(body, "## Status") {
		t.Fatalf("body lost first real section:\n%s", body)
	}
}

func TestStripFirstH1AndLeadPreservesImmediateStructuredContent(t *testing.T) {
	listBody := stripFirstH1AndLead(`# Language

- Keep this list.
`)
	if !strings.Contains(listBody, "- Keep this list.") {
		t.Fatalf("body lost immediate list:\n%s", listBody)
	}

	codeBody := stripFirstH1AndLead("# Language\n\n```gwdk\nroute \"/\"\n```\n")
	if !strings.Contains(codeBody, "```gwdk") {
		t.Fatalf("body lost immediate code fence:\n%s", codeBody)
	}
}

func TestHighlightCodeBlocksWrapsLanguageAndTokens(t *testing.T) {
	article := highlightCodeBlocks(`<pre><code class="language-gwdk">route "/"
view {
  <button g:post="/save">Save</button>
}
</code></pre>`)

	for _, want := range []string{
		`<figure class="code" data-language="gwdk">`,
		`<span class="code-lang">GOWDK</span>`,
		`<span class="tok tok-keyword">route</span>`,
		`<span class="tok tok-string">&#34;/&#34;</span>`,
		`<span class="tok tok-directive">g:post</span>`,
	} {
		if !strings.Contains(article, want) {
			t.Fatalf("highlighted article missing %q:\n%s", want, article)
		}
	}
}

func TestStripHTMLCommentsRemovesGoldmarkRawHTMLMarker(t *testing.T) {
	body := stripHTMLComments(`<p>Text <!-- raw HTML omitted --> continues.</p>`)
	if strings.Contains(body, "raw HTML omitted") || strings.Contains(body, "<!--") {
		t.Fatalf("comment marker was not stripped: %s", body)
	}
	if !strings.Contains(body, "<p>Text  continues.</p>") {
		t.Fatalf("unexpected stripped body: %s", body)
	}
}
