package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestServerHandlesInitializeDiagnosticsFormattingCompletionAndShutdown(t *testing.T) {
	uri := "file:///tmp/bad.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\npage bad\nroute \"/bad\"\n\nview {\n<h1>Bad</h1>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/formatting","params":{"textDocument":{"uri":"`+uri+`"}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/completion","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":0,"character":0}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 5 {
		t.Fatalf("expected 5 output messages, got %d", len(messages))
	}

	assertResponseID(t, messages[0], float64(1))
	if messages[1]["method"] != "textDocument/publishDiagnostics" {
		t.Fatalf("expected diagnostics notification, got %#v", messages[1])
	}
	params := messages[1]["params"].(map[string]any)
	if params["uri"] != uri {
		t.Fatalf("unexpected diagnostic uri: %#v", params["uri"])
	}
	if diagnostics := params["diagnostics"].([]any); len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for valid document, got %#v", diagnostics)
	}

	assertResponseID(t, messages[2], float64(2))
	edits := messages[2]["result"].([]any)
	if len(edits) != 1 {
		t.Fatalf("expected one formatting edit, got %#v", edits)
	}
	edit := edits[0].(map[string]any)
	if edit["newText"] != "package app\n\npage bad\nroute \"/bad\"\n\nview {\n  <h1>Bad</h1>\n}\n" {
		t.Fatalf("unexpected formatted text: %#v", edit["newText"])
	}

	assertResponseID(t, messages[3], float64(3))
	completion := messages[3]["result"].(map[string]any)
	if !hasCompletionLabel(completion["items"].([]any), "page") {
		t.Fatalf("expected page completion, got %#v", completion["items"])
	}
	if !hasCompletionLabel(completion["items"].([]any), "g:bind:value") {
		t.Fatalf("expected g:bind:value completion, got %#v", completion["items"])
	}

	assertResponseID(t, messages[4], float64(4))
}

func TestServerPublishesDiagnosticsAndClearsOnClose(t *testing.T) {
	uri := "file:///tmp/bad.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\npage bad\n@unknown nope\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"`+uri+`"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 4 {
		t.Fatalf("expected 4 output messages, got %d", len(messages))
	}

	firstDiagnostics := messages[1]["params"].(map[string]any)["diagnostics"].([]any)
	if len(firstDiagnostics) == 0 {
		t.Fatal("expected diagnostics for invalid document")
	}
	first := firstDiagnostics[0].(map[string]any)
	if first["code"] != "parse_error" {
		t.Fatalf("expected parse_error code, got %#v", first)
	}
	firstRange := first["range"].(map[string]any)
	start := firstRange["start"].(map[string]any)
	end := firstRange["end"].(map[string]any)
	if start["line"] != float64(3) || start["character"] != float64(0) ||
		end["line"] != float64(3) || end["character"] != float64(13) {
		t.Fatalf("expected full parse-error line range, got %#v", firstRange)
	}
	secondDiagnostics := messages[2]["params"].(map[string]any)["diagnostics"].([]any)
	if len(secondDiagnostics) != 0 {
		t.Fatalf("expected diagnostics to clear on close, got %#v", secondDiagnostics)
	}
}

func TestServerPublishesComponentClientDiagnostics(t *testing.T) {
	uri := "file:///tmp/counter.cmp.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\ncomponent Counter\n\nclient {\n  fn Bad() {\n    Missing++\n  }\n}\n\nview {\n  <button g:on:click={Bad()}>Bad</button>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 3 {
		t.Fatalf("expected 3 output messages, got %d", len(messages))
	}

	params := messages[1]["params"].(map[string]any)
	if params["uri"] != uri {
		t.Fatalf("unexpected diagnostic uri: %#v", params["uri"])
	}
	diagnostics := params["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one component client diagnostic, got %#v", diagnostics)
	}
	diagnostic := diagnostics[0].(map[string]any)
	if diagnostic["code"] != "component_client_error" {
		t.Fatalf("expected component_client_error code, got %#v", diagnostic)
	}
	if diagnostic["source"] != "gowdk" {
		t.Fatalf("expected gowdk diagnostic source, got %#v", diagnostic)
	}
	if message := diagnostic["message"].(string); !strings.Contains(message, `unknown island field "Missing"`) {
		t.Fatalf("unexpected diagnostic message: %q", message)
	}
	diagnosticRange := diagnostic["range"].(map[string]any)
	start := diagnosticRange["start"].(map[string]any)
	end := diagnosticRange["end"].(map[string]any)
	if start["line"] != float64(6) || start["character"] != float64(0) ||
		end["line"] != float64(6) || end["character"] != float64(1) {
		t.Fatalf("expected client statement range, got %#v", diagnosticRange)
	}

	assertResponseID(t, messages[2], float64(2))
}

func TestServerReturnsProjectAwareCompletions(t *testing.T) {
	componentURI := "file:///tmp/card.cmp.gwdk"
	layoutURI := "file:///tmp/root.layout.gwdk"
	pageURI := "file:///tmp/home.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+componentURI+`","languageId":"gwdk","version":1,"text":"package app\n\ncomponent ProductCard\n\nprops {\n  title string\n}\n\nclient {\n  fn Increment() {\n    Count++\n  }\n}\n\nview {\n  <article>{title}{Count}</article>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+layoutURI+`","languageId":"gwdk","version":1,"text":"package app\n\nlayout root\n\nview {\n  <slot />\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+pageURI+`","languageId":"gwdk","version":1,"text":"package app\nimport ui \"github.com/cssbruno/gowdk/testfixture/islands\"\n\npage home\nroute \"/products\"\nlayout root\nguard RequireUser\n\nstore cart ui.CounterState = ui.NewCounterState()\n\nview {\n  <main></main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":6,"character":8}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/completion","params":{"textDocument":{"uri":"`+componentURI+`"},"position":{"line":8,"character":13}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 7 {
		t.Fatalf("expected 7 output messages, got %d", len(messages))
	}
	assertResponseID(t, messages[4], float64(2))
	completion := messages[4]["result"].(map[string]any)
	items := completion["items"].([]any)
	for _, label := range []string{"ProductCard", "/products", "home", "root", "RequireUser", "cart"} {
		if !hasCompletionLabel(items, label) {
			t.Fatalf("expected %q completion, got %#v", label, items)
		}
	}
	assertResponseID(t, messages[5], float64(3))
	componentCompletion := messages[5]["result"].(map[string]any)
	if !hasCompletionLabel(componentCompletion["items"].([]any), "title") {
		t.Fatalf("expected prop/value completion, got %#v", componentCompletion["items"])
	}
	if !hasCompletionLabel(componentCompletion["items"].([]any), "Count") {
		t.Fatalf("expected state/value completion, got %#v", componentCompletion["items"])
	}
	assertResponseID(t, messages[6], float64(4))
}

func TestServerReturnsHoverForLanguageAndProjectSymbols(t *testing.T) {
	uri := "file:///tmp/signup.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\npage signup\nroute \"/signup\"\n\nact Submit POST \"/signup\"\n\nview {\n  <form g:post={Submit}></form>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":3,"character":2}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/hover","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":5,"character":6}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 5 {
		t.Fatalf("expected 5 output messages, got %d", len(messages))
	}
	assertResponseID(t, messages[2], float64(2))
	assertHoverContains(t, messages[2], "route", "Declare the route path")
	assertResponseID(t, messages[3], float64(3))
	assertHoverContains(t, messages[3], "Submit", "GOWDK action handler")
}

func TestServerReturnsDefinitionForComponentCalls(t *testing.T) {
	localComponentURI := "file:///tmp/product-card.cmp.gwdk"
	importedComponentURI := "file:///tmp/button.cmp.gwdk"
	pageURI := "file:///tmp/home.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+localComponentURI+`","languageId":"gwdk","version":1,"text":"package app\n\ncomponent ProductCard\n\nview {\n  <article></article>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+importedComponentURI+`","languageId":"gwdk","version":1,"text":"package design\n\ncomponent Button\n\nview {\n  <button></button>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+pageURI+`","languageId":"gwdk","version":1,"text":"package app\nuse ui \"design\"\n\npage home\nroute \"/\"\n\nview {\n  <main><ProductCard /><ui.Button /></main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":7,"character":11}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":7,"character":27}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 7 {
		t.Fatalf("expected 7 output messages, got %d", len(messages))
	}

	capabilities := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["definitionProvider"] != true {
		t.Fatalf("expected definition provider capability, got %#v", capabilities)
	}
	assertResponseID(t, messages[4], float64(2))
	assertLocation(t, messages[4], localComponentURI, 2, 0)
	assertResponseID(t, messages[5], float64(3))
	assertLocation(t, messages[5], importedComponentURI, 2, 0)
	assertResponseID(t, messages[6], float64(4))
}

func TestServerReturnsDefinitionForWorkspaceComponentFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gowdk.config.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(root, "assets", "examples", "wasm", "src", "pages", "app.page.gwdk")
	componentPath := filepath.Join(root, "assets", "examples", "wasm", "src", "components", "runtime-card.cmp.gwdk")
	if err := os.MkdirAll(filepath.Dir(pagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(componentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	pageSource := "package demo\n\npage app\nroute \"/\"\n\nview {\n  <main><RuntimeCard /></main>\n}\n"
	componentSource := "package demo\n\ncomponent RuntimeCard\n\nview {\n  <section></section>\n}\n"
	if err := os.WriteFile(pagePath, []byte(pageSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(componentPath, []byte(componentSource), 0o644); err != nil {
		t.Fatal(err)
	}
	pageURI := fileURI(pagePath)
	componentURI := fileURI(componentPath)
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+pageURI+`","languageId":"gwdk","version":1,"text":`+strconv.Quote(pageSource)+`}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":6,"character":10}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 4 {
		t.Fatalf("expected 4 output messages, got %d", len(messages))
	}
	assertResponseID(t, messages[2], float64(2))
	assertLocation(t, messages[2], componentURI, 2, 0)
}

func TestServerReturnsDefinitionForOpenGoHandlerSymbols(t *testing.T) {
	pageURI := "file:///tmp/signup.page.gwdk"
	goURI := "file:///tmp/handlers.go"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+pageURI+`","languageId":"gwdk","version":1,"text":"package app\n\npage signup\nroute \"/signup\"\n\nact Submit POST \"/signup\"\n\nview {\n  <form g:post={Submit}></form>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+goURI+`","languageId":"go","version":1,"text":"package app\n\nfunc Submit() error {\n  return nil\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":5,"character":6}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 5 {
		t.Fatalf("expected 5 output messages, got %d", len(messages))
	}

	assertResponseID(t, messages[3], float64(2))
	assertLocation(t, messages[3], goURI, 2, 5)
	assertResponseID(t, messages[4], float64(3))
}

func TestServerReturnsReferencesForProjectSymbols(t *testing.T) {
	componentURI := "file:///tmp/product-card.cmp.gwdk"
	pageURI := "file:///tmp/home.page.gwdk"
	adminURI := "file:///tmp/admin.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+componentURI+`","languageId":"gwdk","version":1,"text":"package app\n\ncomponent ProductCard\n\nclient {\n  use cart\n}\n\nview {\n  <article></article>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+pageURI+`","languageId":"gwdk","version":1,"text":"package app\nimport ui \"github.com/cssbruno/gowdk/testfixture/islands\"\n\npage home\nroute \"/products\"\nguard RequireUser\n\nstore cart ui.CounterState = ui.NewCounterState()\n\nview {\n  <main><ProductCard /></main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+adminURI+`","languageId":"gwdk","version":1,"text":"package app\n\npage admin\nroute \"/admin\"\nguard RequireUser\n\nview {\n  <main>Admin</main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/references","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":10,"character":11},"context":{"includeDeclaration":true}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/references","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":7,"character":7},"context":{"includeDeclaration":true}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"textDocument/references","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":5,"character":10},"context":{"includeDeclaration":true}}}`) +
		framed(`{"jsonrpc":"2.0","id":5,"method":"textDocument/references","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":3,"character":7},"context":{"includeDeclaration":true}}}`) +
		framed(`{"jsonrpc":"2.0","id":6,"method":"textDocument/references","params":{"textDocument":{"uri":"`+pageURI+`"},"position":{"line":4,"character":10},"context":{"includeDeclaration":true}}}`) +
		framed(`{"jsonrpc":"2.0","id":7,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 10 {
		t.Fatalf("expected 10 output messages, got %d", len(messages))
	}

	capabilities := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["referencesProvider"] != true {
		t.Fatalf("expected references provider capability, got %#v", capabilities)
	}
	assertResponseID(t, messages[4], float64(2))
	assertReferenceURIs(t, messages[4], componentURI, pageURI)
	assertResponseID(t, messages[5], float64(3))
	assertReferenceURIs(t, messages[5], componentURI, pageURI)
	assertResponseID(t, messages[6], float64(4))
	assertReferenceURIs(t, messages[6], adminURI, pageURI)
	assertResponseID(t, messages[7], float64(5))
	assertReferenceURIs(t, messages[7], pageURI)
	assertResponseID(t, messages[8], float64(6))
	assertReferenceURIs(t, messages[8], pageURI)
	assertResponseID(t, messages[9], float64(7))
}

func TestServerReturnsCodeActionsForMigrationsAndMissingUses(t *testing.T) {
	oldURI := "file:///tmp/old.page.gwdk"
	useURI := "file:///tmp/missing-use.page.gwdk"
	oldMessage := strconv.Quote("line 6: old action block syntax is not supported; use `act Refresh POST \"<path>\"` and move behavior to Go")
	useMessage := strconv.Quote("home references component <ui.Button />, but alias \"ui\" is not declared. Add `use ui \"<package>\"` before the view block")
	oldDiagnostic := `{"range":{"start":{"line":5,"character":0},"end":{"line":5,"character":13}},"severity":1,"code":"old_action_block_syntax","source":"gowdk","message":` + oldMessage + `}`
	useDiagnostic := `{"range":{"start":{"line":7,"character":0},"end":{"line":7,"character":1}},"severity":1,"code":"unknown_gowdk_use_alias","source":"gowdk","message":` + useMessage + `}`
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+oldURI+`","languageId":"gwdk","version":1,"text":"package app\n\npage old\nroute \"/old\"\n\nact refresh {\n}\n\nview {\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+useURI+`","languageId":"gwdk","version":1,"text":"package app\n\npage home\nroute \"/\"\n\nview {\n  <main><ui.Button /></main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/codeAction","params":{"textDocument":{"uri":"`+oldURI+`"},"range":{"start":{"line":5,"character":0},"end":{"line":5,"character":13}},"context":{"diagnostics":[`+oldDiagnostic+`]}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"textDocument/codeAction","params":{"textDocument":{"uri":"`+useURI+`"},"range":{"start":{"line":7,"character":0},"end":{"line":7,"character":1}},"context":{"diagnostics":[`+useDiagnostic+`]}}}`) +
		framed(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 6 {
		t.Fatalf("expected 6 output messages, got %d", len(messages))
	}

	capabilities := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["codeActionProvider"] != true {
		t.Fatalf("expected code action provider capability, got %#v", capabilities)
	}
	assertResponseID(t, messages[3], float64(2))
	assertCodeActionEdit(t, messages[3], oldURI, "Replace old endpoint block header", `act Refresh POST "/old"`, 5)
	assertResponseID(t, messages[4], float64(3))
	assertCodeActionEdit(t, messages[4], useURI, "Add missing use declaration", "use ui \"package\"\n", 5)
	assertResponseID(t, messages[5], float64(4))
}

func TestServerReturnsSemanticTokens(t *testing.T) {
	uri := "file:///tmp/home.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\npage home\nroute \"/\"\n\nview {\n  <main>{title}</main>\n}\n"}}}`) +
		framed(`{"jsonrpc":"2.0","id":2,"method":"textDocument/semanticTokens/full","params":{"textDocument":{"uri":"`+uri+`"}}}`) +
		framed(`{"jsonrpc":"2.0","id":3,"method":"shutdown","params":null}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 4 {
		t.Fatalf("expected 4 output messages, got %d", len(messages))
	}

	capabilities := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	provider := capabilities["semanticTokensProvider"].(map[string]any)
	if provider["full"] != true {
		t.Fatalf("expected full semantic-token support, got %#v", provider)
	}
	legend := provider["legend"].(map[string]any)
	for _, tokenType := range []string{"decorator", "variable", "string", "operator"} {
		if !hasStringValue(legend["tokenTypes"].([]any), tokenType) {
			t.Fatalf("expected semantic token type %q, got %#v", tokenType, legend["tokenTypes"])
		}
	}

	assertResponseID(t, messages[2], float64(2))
	result := messages[2]["result"].(map[string]any)
	data := result["data"].([]any)
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("expected semantic-token data groups, got %#v", data)
	}
	assertNumberPrefix(t, data, []float64{
		0, 0, 7, 1, 0, // package
		0, 8, 3, 1, 0, // app
		2, 0, 4, 0, 0, // page
	})
}

func TestServerReturnsMethodNotFoundForUnknownRequests(t *testing.T) {
	input := framed(`{"jsonrpc":"2.0","id":"x","method":"gowdk/unknown","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("expected one response, got %d", len(messages))
	}
	errPayload := messages[0]["error"].(map[string]any)
	if errPayload["code"] != float64(methodNotFound) {
		t.Fatalf("expected method-not-found error, got %#v", errPayload)
	}
}

func TestServerReturnsParseErrorForMalformedJSON(t *testing.T) {
	input := framed(`{"jsonrpc":"2.0","id":1,"method":`) +
		framed(`{"jsonrpc":"2.0","method":"exit"}`)

	var output bytes.Buffer
	server := NewServer(gowdk.Config{})
	server.log = nil
	if err := server.Serve(stringsReader(input), &output); err != nil {
		t.Fatal(err)
	}

	messages := readOutputMessages(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("expected one response, got %d", len(messages))
	}
	errPayload := messages[0]["error"].(map[string]any)
	if errPayload["code"] != float64(parseError) {
		t.Fatalf("expected parse error, got %#v", errPayload)
	}
}

func framed(payload string) string {
	return "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + payload
}

func readOutputMessages(t *testing.T, output []byte) []map[string]any {
	t.Helper()
	reader := bufio.NewReader(bytes.NewReader(output))
	var messages []map[string]any
	for {
		body, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		var message map[string]any
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

func assertResponseID(t *testing.T, message map[string]any, id any) {
	t.Helper()
	if message["id"] != id {
		t.Fatalf("expected response id %#v, got %#v in %#v", id, message["id"], message)
	}
}

func hasCompletionLabel(items []any, label string) bool {
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["label"] == label {
			return true
		}
	}
	return false
}

func hasStringValue(values []any, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func assertNumberPrefix(t *testing.T, values []any, expected []float64) {
	t.Helper()
	if len(values) < len(expected) {
		t.Fatalf("expected at least %d values, got %#v", len(expected), values)
	}
	for index, want := range expected {
		if values[index] != want {
			t.Fatalf("expected value %d to be %v, got %#v in %#v", index, want, values[index], values)
		}
	}
}

func assertLocation(t *testing.T, message map[string]any, uri string, line int, character int) {
	t.Helper()
	result := message["result"].(map[string]any)
	if result["uri"] != uri {
		t.Fatalf("expected location uri %q, got %#v", uri, result)
	}
	start := result["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(line) || start["character"] != float64(character) {
		t.Fatalf("expected location start %d:%d, got %#v", line, character, result["range"])
	}
}

func assertReferenceURIs(t *testing.T, message map[string]any, expected ...string) {
	t.Helper()
	result := message["result"].([]any)
	if len(result) != len(expected) {
		t.Fatalf("expected %d references, got %#v", len(expected), result)
	}
	seen := map[string]int{}
	for _, raw := range result {
		location := raw.(map[string]any)
		seen[location["uri"].(string)]++
	}
	for _, uri := range expected {
		if seen[uri] == 0 {
			t.Fatalf("expected references to include %q, got %#v", uri, result)
		}
		seen[uri]--
	}
}

func assertCodeActionEdit(t *testing.T, message map[string]any, uri string, title string, newText string, line int) {
	t.Helper()
	actions := message["result"].([]any)
	if len(actions) != 1 {
		t.Fatalf("expected one code action, got %#v", actions)
	}
	action := actions[0].(map[string]any)
	if action["title"] != title {
		t.Fatalf("expected code action title %q, got %#v", title, action)
	}
	edit := action["edit"].(map[string]any)
	changes := edit["changes"].(map[string]any)
	edits := changes[uri].([]any)
	if len(edits) != 1 {
		t.Fatalf("expected one text edit for %q, got %#v", uri, edits)
	}
	textEdit := edits[0].(map[string]any)
	if textEdit["newText"] != newText {
		t.Fatalf("expected edit text %q, got %#v", newText, textEdit)
	}
	start := textEdit["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(line) {
		t.Fatalf("expected edit line %d, got %#v", line, textEdit["range"])
	}
}

func assertHoverContains(t *testing.T, message map[string]any, parts ...string) {
	t.Helper()
	result := message["result"].(map[string]any)
	contents := result["contents"].(map[string]any)
	value := contents["value"].(string)
	for _, part := range parts {
		if !strings.Contains(value, part) {
			t.Fatalf("expected hover to contain %q, got %q", part, value)
		}
	}
}

func stringsReader(input string) *bytes.Reader {
	return bytes.NewReader([]byte(input))
}
