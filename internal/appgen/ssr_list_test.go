package appgen

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/source"
)

func TestGenerateWritesServerListRenderer(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>App</main>")

	result, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "board",
		Route:   "/board",
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "example.com/app/board",
			PackageName:  "board",
			FunctionName: "LoadBoard",
			Signature:    source.BackendSignatureLoadError,
		},
		HTML: `<section>__GOWDK_SSR_LIST_s1__</section>`,
		ListSpecs: []SSRListSpec{{
			Placeholder: "__GOWDK_SSR_LIST_s1__",
			SourcePath:  "columns",
			RowTemplate: `<div>__GOWDK_SSR_FIELD_1____GOWDK_SSR_LIST_s2__</div>`,
			Fields:      []SSRListField{{Placeholder: "__GOWDK_SSR_FIELD_1__", Path: "title"}},
			Children: []SSRListSpec{{
				Placeholder: "__GOWDK_SSR_LIST_s2__",
				SourcePath:  "issues",
				RowTemplate: `<li>__GOWDK_SSR_FIELD_2__</li>`,
				Fields:      []SSRListField{{Placeholder: "__GOWDK_SSR_FIELD_2__", Path: "id"}},
			}},
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(result.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	generated := string(payload)

	// The generated handler must be syntactically valid Go.
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v", err)
	}

	for _, expected := range []string{
		`gowdkssr "github.com/cssbruno/gowdk/addons/ssr"`,
		`html = gowdkssr.RenderLists(html, []gowdkssr.ListSpec{`,
		`Placeholder: "__GOWDK_SSR_LIST_s1__"`,
		`SourcePath: "columns"`,
		`Fields: []gowdkssr.ListField{`,
		`Path: "title"`,
		`Children: []gowdkssr.ListSpec{`,
		`SourcePath: "issues"`,
		`}, loadData)`,
	} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("expected generated main.go to contain %q:\n%s", expected, generated)
		}
	}
}

// TestGeneratedBinaryExecutesServerList compiles and runs a real generated app
// that renders a nested g:each board from request-time load data, proving the
// whole pipeline (codegen + runtime list renderer) works against the real
// runtime.
func TestGeneratedBinaryExecutesServerList(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "dist")
	appDir := filepath.Join(root, "generated-app")
	binaryPath := filepath.Join(root, "site")
	writeTestFile(t, filepath.Join(outputDir, "index.html"), "<main>Home</main>")

	if _, err := GenerateWithOptions(outputDir, appDir, Options{SSR: []SSRRoute{{
		PageID:  "board",
		Route:   "/board",
		Guards:  []string{"public"},
		HasLoad: true,
		LoadBinding: source.BackendBinding{
			Status:       source.BackendBindingBound,
			ImportPath:   "gowdk-generated-app/board",
			PackageName:  "board",
			FunctionName: "LoadBoard",
			Signature:    source.BackendSignatureLoadError,
		},
		HTML: `<main>__GOWDK_SSR_LIST_s1__</main>`,
		ListSpecs: []SSRListSpec{{
			Placeholder: "__GOWDK_SSR_LIST_s1__",
			SourcePath:  "columns",
			RowTemplate: `<section><h2>__GOWDK_SSR_FIELD_1__</h2>__GOWDK_SSR_LIST_s2__</section>`,
			Fields:      []SSRListField{{Placeholder: "__GOWDK_SSR_FIELD_1__", Path: "title"}},
			Children: []SSRListSpec{{
				Placeholder: "__GOWDK_SSR_LIST_s2__",
				SourcePath:  "issues",
				RowTemplate: `<article><span>__GOWDK_SSR_FIELD_2__</span> __GOWDK_SSR_FIELD_3__</article>`,
				Fields: []SSRListField{
					{Placeholder: "__GOWDK_SSR_FIELD_2__", Path: "id"},
					{Placeholder: "__GOWDK_SSR_FIELD_3__", Path: "title"},
				},
			}},
		}},
	}}}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appDir, "board", "board.go"), `package board

import "github.com/cssbruno/gowdk/addons/ssr"

func LoadBoard(ssr.LoadContext) (map[string]any, error) {
	return map[string]any{
		"columns": []any{
			map[string]any{"title": "Todo", "issues": []any{
				map[string]any{"id": "T-1", "title": "Wire <auth>"},
				map[string]any{"id": "T-2", "title": "Ship board"},
			}},
			map[string]any{"title": "Done", "issues": []any{
				map[string]any{"id": "D-1", "title": "Spec"},
			}},
		},
	}, nil
}
`)
	if _, err := BuildBinary(appDir, binaryPath); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	}()

	body, _, err := waitForHTTPResponse("http://" + addr + "/board")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`<h2>Todo</h2>`,
		`<h2>Done</h2>`,
		`<article><span>T-1</span> Wire &lt;auth&gt;</article>`,
		`<article><span>T-2</span> Ship board</article>`,
		`<article><span>D-1</span> Spec</article>`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected board response to contain %q, got %s", expected, body)
		}
	}
	if strings.Contains(body, "<auth>") {
		t.Fatalf("server data was not escaped: %s", body)
	}
	if strings.Contains(body, "__GOWDK_SSR_") {
		t.Fatalf("unconsumed placeholder in response: %s", body)
	}
}
