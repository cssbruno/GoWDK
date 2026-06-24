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

func TestEndpointHeaderFixQuotesRouteSafely(t *testing.T) {
	edits, err := Edits(diagnostics.Fix{Title: "Replace endpoint", Rewriter: diagnostics.FixEndpointHeaderFromMessage}, `package app

page home
route "/quote\"here"

act Submit {
}
`, Diagnostic{
		Code:    "old_action_block_syntax",
		Message: `line 6: old action block syntax is not supported; use ` + "`" + `act Submit POST "<path>"` + "`" + ` and move behavior to Go`,
		Range:   Range{Start: Position{Line: 6, Column: 1}, End: Position{Line: 6, Column: 13}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || edits[0].NewText != `act Submit POST "/quote\"here"` {
		t.Fatalf("unexpected edits: %#v", edits)
	}
}
