package buildgen

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

// TestComponentInitialStateSeedsTypedUseStoreFields verifies that a component
// which binds a store with `use cart ui.CounterState` (and no local state
// declaration) seeds the island with the store type's field shape, so SSR and
// the initial client state carry the right keys before the store registry merges
// the real value on mount.
func TestComponentInitialStateSeedsTypedUseStoreFields(t *testing.T) {
	component := gwdkir.Component{
		Name:    "CartBadge",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		Blocks: gwdkir.Blocks{
			Client:     true,
			ClientBody: "use cart ui.CounterState",
			View:       true,
			ViewBody:   "<span>{Count}</span>",
		},
	}

	state, stateTypes, stateJSON, err := componentInitialState(component)
	if err != nil {
		t.Fatalf("componentInitialState: %v", err)
	}
	if stateTypes["Count"] != clientlang.TypeInt {
		t.Fatalf("Count type = %v, want int", stateTypes["Count"])
	}
	if stateTypes["Open"] != clientlang.TypeBool {
		t.Fatalf("Open type = %v, want bool", stateTypes["Open"])
	}
	if state["Count"] != "0" || state["Open"] != "false" {
		t.Fatalf("seed should be zero values, got Count=%q Open=%q", state["Count"], state["Open"])
	}
	if !strings.Contains(stateJSON, `"Count"`) || !strings.Contains(stateJSON, `"Open"`) {
		t.Fatalf("state JSON should carry store field keys, got %s", stateJSON)
	}
}

// TestComponentInitialStateUntypedUseAddsNoFields verifies a plain `use cart`
// (no type annotation) does not introduce store fields into the seed, preserving
// the prior behavior where a matching local state declaration is required.
func TestComponentInitialStateUntypedUseAddsNoFields(t *testing.T) {
	component := gwdkir.Component{
		Name: "CartBadge",
		Blocks: gwdkir.Blocks{
			Client:     true,
			ClientBody: "use cart",
			View:       true,
			ViewBody:   "<span>x</span>",
		},
	}

	state, _, stateJSON, err := componentInitialState(component)
	if err != nil {
		t.Fatalf("componentInitialState: %v", err)
	}
	if len(state) != 0 || stateJSON != "" {
		t.Fatalf("untyped use should add no seed, got state=%v json=%q", state, stateJSON)
	}
}
