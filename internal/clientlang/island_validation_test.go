package clientlang

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateIslandClientStatementsTypedWithEventsTracksLocalsRefsAndMutations(t *testing.T) {
	writeSymbols := map[string]ValueType{
		"Count": TypeInt,
		"Items": TypeArray,
	}
	readSymbols := map[string]ValueType{
		"Count":        TypeInt,
		"Items":        TypeArray,
		"Items[].ID":   TypeString,
		"Items[].Name": TypeString,
		"Name":         TypeString,
	}
	refs := map[string]Ref{
		"NameInput": {Name: "NameInput", Kind: "input"},
	}
	helpers := map[string]ExprFunction{
		"Slug": {Params: []ValueType{TypeString}, Return: TypeString},
	}
	statements := []string{
		`let id string = Slug(Name)`,
		`append(Items, { ID: id, Name: Name })`,
		`Count++`,
		`NameInput.Focus()`,
	}

	usedRefs, err := ValidateIslandClientStatementsTypedWithEvents(statements, writeSymbols, readSymbols, refs, helpers, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !usedRefs["NameInput"] || len(usedRefs) != 1 {
		t.Fatalf("unexpected used refs: %#v", usedRefs)
	}
}

func TestValidateIslandClientStatementsTypedWithEventsReportsStatementIndex(t *testing.T) {
	_, err := ValidateIslandClientStatementsTypedWithEvents([]string{
		`Count++`,
		`let Count int = 1`,
	}, map[string]ValueType{"Count": TypeInt}, map[string]ValueType{"Count": TypeInt}, nil, nil, false, nil, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var statementErr StatementValidationError
	if !errors.As(err, &statementErr) {
		t.Fatalf("expected StatementValidationError, got %T", err)
	}
	if statementErr.Index != 1 {
		t.Fatalf("expected statement index 1, got %d", statementErr.Index)
	}
	if !strings.Contains(statementErr.Error(), `local "Count" conflicts with a state field`) {
		t.Fatalf("unexpected error: %v", statementErr)
	}
}

func TestValidateIslandEventExpressionTypedWithEventsAllowsDeclaredEmit(t *testing.T) {
	err := ValidateIslandEventExpressionTypedWithEvents(
		`emit Saved(Count, Label)`,
		map[string]ValueType{"Count": TypeInt, "Label": TypeString},
		nil,
		nil,
		nil,
		map[string]Emit{
			"Saved": {
				Name:       "Saved",
				Params:     []string{"count", "label"},
				ParamTypes: []ValueType{TypeInt, TypeString},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIslandExpressionFieldsIncludesMutationInputs(t *testing.T) {
	fields := IslandExpressionFields(`append(Items, { ID: NextID, Name: lower(Name) })`)
	if got, want := strings.Join(fields, ","), "Items,Name,NextID"; got != want {
		t.Fatalf("unexpected fields: got %s want %s", got, want)
	}
}

func TestValidateClearStatementChecksUsedStores(t *testing.T) {
	state := map[string]ValueType{"Count": TypeInt}
	stores := map[string]bool{"cart": true}
	if _, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear cart"}, state, state, nil, nil, false, nil, stores,
	); err != nil {
		t.Fatalf("clear of a used store should validate, got %v", err)
	}
	_, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear prefs"}, state, state, nil, nil, false, nil, stores,
	)
	if err == nil || !strings.Contains(err.Error(), `store "prefs"`) {
		t.Fatalf("expected unused-store error, got %v", err)
	}
	if _, err := ValidateIslandClientStatementsTypedWithEvents(
		[]string{"clear cart"}, state, state, nil, nil, false, nil, nil,
	); err == nil {
		t.Fatal("expected error when the component uses no stores")
	}
}
