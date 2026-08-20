package parser

import (
	"strings"
	"testing"
)

func TestDirectiveLaneDeclarationsAreRequiredAndRecorded(t *testing.T) {
	page, err := ParsePage([]byte(`
page issues
route "/issues"
server {
	issues := LoadIssues()
	visible := true
	=> { issues, visible }
}
view {
  <ul g:if={visible} g:lane="server">
    <li g:for={issue in issues} g:lane="server">{issue.title}</li>
  </ul>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Blocks.DirectiveLanes) != 2 {
		t.Fatalf("directive lanes = %#v", page.Blocks.DirectiveLanes)
	}
	if first := page.Blocks.DirectiveLanes[0]; first.Directive != "g:if" || first.Lane != "server" || first.Expression != "visible" {
		t.Fatalf("first directive lane = %#v", first)
	}
	if second := page.Blocks.DirectiveLanes[1]; second.Directive != "g:for" || second.Lane != "server" || second.Expression != "issue in issues" {
		t.Fatalf("second directive lane = %#v", second)
	}

	_, err = ParsePage([]byte(`page p
route "/"
view {
  <p g:if={Open}>open</p>
}
`))
	diagnostic, ok := ParserDiagnostic(err)
	if !ok || diagnostic.Code != DiagnosticDirectiveLaneRequired || !strings.Contains(diagnostic.Message, "implicit directive lanes were removed") {
		t.Fatalf("missing-lane diagnostic = %#v, %v", diagnostic, err)
	}
}

func TestDirectiveLaneDeclarationsRejectInvalidShapes(t *testing.T) {
	tests := []struct {
		view string
		want string
	}{
		{`<p g:if={Open} g:lane>open</p>`, "string literal"},
		{`<p g:if={Open} g:lane={Lane}>open</p>`, "string literal"},
		{`<p g:if={Open} g:lane="worker">open</p>`, "server"},
		{`<p g:lane="client">open</p>`, "requires g:for or g:if"},
	}
	for _, test := range tests {
		_, err := ParsePage([]byte("page p\nroute \"/\"\nview {\n  " + test.view + "\n}\n"))
		diagnostic, ok := ParserDiagnostic(err)
		if !ok || diagnostic.Code != DiagnosticDirectiveLaneInvalid || !strings.Contains(diagnostic.Message, test.want) {
			t.Fatalf("view %q diagnostic = %#v, %v", test.view, diagnostic, err)
		}
	}
}
