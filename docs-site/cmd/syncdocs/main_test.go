package main

import (
	"strings"
	"testing"
)

func TestRewriteLinksOnlyRoutesGeneratedPages(t *testing.T) {
	routes := map[string]bool{
		"/docs/reference": true,
	}
	html := `<a href="reference/README.md">Reference</a><a href="cookbook/README.md#coverage">Cookbook</a><a href="https://example.com/guide.md">External</a>`

	got := rewriteLinks(html, "getting-started.md", routes)

	for _, expected := range []string{
		`href="/docs/reference/"`,
		`href="https://github.com/cssbruno/GoWDK/blob/main/docs/cookbook/README.md#coverage"`,
		`href="https://example.com/guide.md"`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in rewritten links:\n%s", expected, got)
		}
	}
}

func TestRewriteLinksFallsBackToRepoRootForOutsideDocsTargets(t *testing.T) {
	routes := map[string]bool{}
	html := `<a href="../../examples/flagship/README.md">Flagship</a>`

	got := rewriteLinks(html, "product/playground.md", routes)

	expected := `href="https://github.com/cssbruno/GoWDK/blob/main/examples/flagship/README.md"`
	if !strings.Contains(got, expected) {
		t.Fatalf("expected %q in rewritten links:\n%s", expected, got)
	}
}
