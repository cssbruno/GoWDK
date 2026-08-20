package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestValidateDirectiveLanesRejectsDataOwnershipMismatch(t *testing.T) {
	program := gwdkir.Program{Pages: []gwdkir.Page{{
		ID: "issues",
		Blocks: gwdkir.Blocks{
			Server:       true,
			ServerFields: []string{"issues", "visible"},
			DirectiveLanes: []gwdkir.DirectiveLane{
				{Path: "0", Directive: "g:if", Lane: "client", Expression: "visible"},
				{Path: "1", Directive: "g:for", Lane: "server", Expression: "item in localItems"},
			},
		},
	}}}
	diagnostics := validateDirectiveLanes(program)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "directive_lane_mismatch" || !strings.Contains(diagnostic.Message, "resolves to") {
			t.Fatalf("unexpected diagnostic: %#v", diagnostic)
		}
	}
}

func TestValidateDirectiveLanesRejectsServerLaneInComponent(t *testing.T) {
	program := gwdkir.Program{Components: []gwdkir.Component{{Name: "List", Blocks: gwdkir.Blocks{DirectiveLanes: []gwdkir.DirectiveLane{{Path: "0", Directive: "g:for", Lane: "server", Expression: "item in Items"}}}}}}
	diagnostics := validateDirectiveLanes(program)
	if len(diagnostics) != 1 || diagnostics[0].Code != "directive_lane_mismatch" || !strings.Contains(diagnostics[0].Message, `g:lane="client"`) {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
