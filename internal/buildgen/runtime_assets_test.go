package buildgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestClientGoBlockWASMSourceMergesImportsWithAST(t *testing.T) {
	page := manifest.Page{
		ID: "counter",
		Imports: []manifest.Import{
			{Alias: "dom", Path: "syscall/js"},
			{Path: "fmt"},
			{Path: "example.com/unused"},
		},
	}
	block := manifest.GoBlock{Body: `import "strings"

func GOWDKMountCounter(value string) {
	_ = dom.Global()
	_ = fmt.Sprintf("%s", value)
	_ = strings.TrimSpace(value)
}
`}

	source, err := clientGoBlockWASMSource(page, block)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.ParseComments|parser.AllErrors); err != nil {
		t.Fatalf("generated client Go must parse: %v\n%s", err, source)
	}
	for _, expected := range []string{
		`dom "syscall/js"`,
		`"fmt"`,
		`"strings"`,
		`func main()`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected generated source to contain %q, got:\n%s", expected, source)
		}
	}
	if strings.Contains(source, "example.com/unused") {
		t.Fatalf("unused page import leaked into generated client Go:\n%s", source)
	}
}
