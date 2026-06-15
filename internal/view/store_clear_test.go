package view

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

func TestValidateClearStatementAllowsUsedStore(t *testing.T) {
	state := map[string]clientlang.ValueType{"Count": clientlang.TypeInt}
	stores := map[string]bool{"cart": true}
	if _, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear cart"}, state, state, nil, nil, false, nil, stores,
	); err != nil {
		t.Fatalf("clear of used store should validate, got %v", err)
	}
}

func TestValidateClearStatementRejectsUnusedStore(t *testing.T) {
	state := map[string]clientlang.ValueType{"Count": clientlang.TypeInt}
	stores := map[string]bool{"cart": true}
	_, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear prefs"}, state, state, nil, nil, false, nil, stores,
	)
	if err == nil || !strings.Contains(err.Error(), `store "prefs"`) {
		t.Fatalf("expected unused-store error, got %v", err)
	}
}

func TestValidateClearStatementRejectsWhenNoStores(t *testing.T) {
	state := map[string]clientlang.ValueType{"Count": clientlang.TypeInt}
	_, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear cart"}, state, state, nil, nil, false, nil, nil,
	)
	if err == nil || !strings.Contains(err.Error(), "does not `use`") {
		t.Fatalf("expected does-not-use error, got %v", err)
	}
}
