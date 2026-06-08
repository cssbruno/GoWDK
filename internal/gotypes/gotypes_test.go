package gotypes

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestStateInitRunnerSourceRewritesImportAndFunctionWithAST(t *testing.T) {
	source, err := stateInitRunnerSource("models", "example.com/app/models", "InitialState")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "runner.go", source, parser.AllErrors); err != nil {
		t.Fatalf("state init runner source must parse: %v\n%s", err, source)
	}
	for _, expected := range []string{
		`models "example.com/app/models"`,
		`models.InitialState()`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated source to contain %q, got:\n%s", expected, source)
		}
	}
	for _, placeholder := range []string{
		"gowdkstateinit",
		"GOWDKStateInit",
	} {
		if strings.Contains(source, placeholder) {
			t.Fatalf("placeholder %q leaked into generated source:\n%s", placeholder, source)
		}
	}
}
