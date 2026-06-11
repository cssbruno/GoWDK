package diagnosticfix

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

func TestMissingUseFixUsesParseablePlaceholder(t *testing.T) {
	edits, err := Edits(diagnostics.Fix{Title: "Add use", Rewriter: diagnostics.FixInsertMissingUse}, `package app

page home
route "/"

view {
  <ui.Button />
}
`, Diagnostic{
		Code:    "unknown_gowdk_use_alias",
		Message: `home references component <ui.Button />, but alias "ui" is not declared. Add ` + "`" + `use ui "<package>"` + "`" + ` before the view block`,
		Range:   Range{Start: Position{Line: 7, Column: 4}, End: Position{Line: 7, Column: 6}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || edits[0].NewText != "use ui \"package\"\n" {
		t.Fatalf("unexpected edits: %#v", edits)
	}
}
