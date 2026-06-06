package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestServerHandlesInitializeDiagnosticsFormattingCompletionAndShutdown(t *testing.T) {
	uri := "file:///tmp/bad.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\n@page bad\n@route \"/bad\"\n\nview {\n<h1>Bad</h1>\n}\n"}}}`) +
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
	if edit["newText"] != "package app\n\n@page bad\n@route \"/bad\"\n\nview {\n  <h1>Bad</h1>\n}\n" {
		t.Fatalf("unexpected formatted text: %#v", edit["newText"])
	}

	assertResponseID(t, messages[3], float64(3))
	completion := messages[3]["result"].(map[string]any)
	if !hasCompletionLabel(completion["items"].([]any), "@page") {
		t.Fatalf("expected @page completion, got %#v", completion["items"])
	}
	if !hasCompletionLabel(completion["items"].([]any), "g:bind:value") {
		t.Fatalf("expected g:bind:value completion, got %#v", completion["items"])
	}

	assertResponseID(t, messages[4], float64(4))
}

func TestServerPublishesDiagnosticsAndClearsOnClose(t *testing.T) {
	uri := "file:///tmp/bad.page.gwdk"
	input := framed(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\n@page bad\n@render nope\n"}}}`) +
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
		end["line"] != float64(3) || end["character"] != float64(12) {
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
		framed(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"gwdk","version":1,"text":"package app\n\n@component Counter\n\nclient {\n  fn Bad() {\n    Missing++\n  }\n}\n\nview {\n  <button g:on:click={Bad()}>Bad</button>\n}\n"}}}`) +
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

func stringsReader(input string) *bytes.Reader {
	return bytes.NewReader([]byte(input))
}
