package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestReadMessageEnforcesBodyLimitBeforeAllocation(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxMessageBytes = 4
	body, err := readMessageWithLimits(bufio.NewReader(strings.NewReader("Content-Length: 4\r\n\r\n1234")), limits)
	if err != nil || string(body) != "1234" {
		t.Fatalf("expected exact-limit message, body=%q err=%v", body, err)
	}

	_, err = readMessageWithLimits(bufio.NewReader(strings.NewReader("Content-Length: 999999999\r\n\r\n")), limits)
	assertFramingCode(t, err, framingMessageTooLarge)
}

func TestReadMessageValidatesContentLengthAndHeaders(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		code  string
	}{
		{name: "missing", frame: "X-Test: yes\r\n\r\n", code: framingMissingContentLength},
		{name: "empty", frame: "Content-Length: \r\n\r\n", code: framingInvalidContentLength},
		{name: "negative", frame: "Content-Length: -1\r\n\r\n", code: framingInvalidContentLength},
		{name: "signed", frame: "Content-Length: +1\r\n\r\n", code: framingInvalidContentLength},
		{name: "overflow", frame: "Content-Length: 999999999999999999999999\r\n\r\n", code: framingInvalidContentLength},
		{name: "conflicting duplicate", frame: "Content-Length: 1\r\ncontent-length: 2\r\n\r\nx", code: framingConflictingContentLength},
		{name: "truncated", frame: "Content-Length: 2\r\n\r\nx", code: framingTruncatedMessage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := readMessageWithLimits(bufio.NewReader(strings.NewReader(test.frame)), DefaultLimits())
			assertFramingCode(t, err, test.code)
		})
	}

	body, err := readMessageWithLimits(bufio.NewReader(strings.NewReader("Content-Length: 1\nCONTENT-LENGTH: 1\n\nx")), DefaultLimits())
	if err != nil || string(body) != "x" {
		t.Fatalf("expected identical duplicate lengths and tolerant LF framing, body=%q err=%v", body, err)
	}
}

func TestReadMessageBoundsHeaderLineAggregateAndCount(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxHeaderLineBytes = 12
	_, err := readMessageWithLimits(bufio.NewReader(strings.NewReader("X-Long: 123456789\n\n")), limits)
	assertFramingCode(t, err, framingHeaderTooLarge)

	limits = DefaultLimits()
	limits.MaxHeaderBytes = 24
	_, err = readMessageWithLimits(bufio.NewReader(strings.NewReader("X-One: 1234\nX-Two: 5678\nContent-Length: 0\n\n")), limits)
	assertFramingCode(t, err, framingHeaderTooLarge)

	limits = DefaultLimits()
	limits.MaxHeaderCount = 1
	_, err = readMessageWithLimits(bufio.NewReader(strings.NewReader("X-One: 1\nContent-Length: 0\n\n")), limits)
	assertFramingCode(t, err, framingTooManyHeaders)
}

func TestReadMessageAcceptsOneByteChunks(t *testing.T) {
	reader := bufio.NewReader(&oneByteReader{data: []byte("Content-Length: 2\r\n\r\n{}")})
	body, err := readMessageWithLimits(reader, DefaultLimits())
	if err != nil || string(body) != "{}" {
		t.Fatalf("expected chunked frame, body=%q err=%v", body, err)
	}
}

func TestDocumentLimitsPreserveLastValidSnapshotAndAccounting(t *testing.T) {
	server := NewProjectServer(gowdk.Config{}, ProjectOptions{Limits: Limits{
		MaxDocumentBytes:     4,
		MaxOpenDocumentBytes: 6,
		MaxOpenDocuments:     2,
	}})

	notifyServer(t, server, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": "file:///one.gwdk", "version": 1, "text": "1234",
	}})
	if got := server.documents["file:///one.gwdk"].Text; got != "1234" || server.openDocumentBytes != 4 {
		t.Fatalf("unexpected first document state: text=%q bytes=%d", got, server.openDocumentBytes)
	}

	messages := notifyServer(t, server, "textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": "file:///one.gwdk", "version": 2},
		"contentChanges": []map[string]any{{"text": "12345"}},
	})
	if len(messages) != 2 || server.documents["file:///one.gwdk"].Text != "1234" || server.openDocumentBytes != 4 {
		t.Fatalf("over-limit update must preserve valid snapshot; messages=%d doc=%#v bytes=%d", len(messages), server.documents["file:///one.gwdk"], server.openDocumentBytes)
	}

	notifyServer(t, server, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": "file:///two.gwdk", "version": 1, "text": "é",
	}})
	if server.openDocumentBytes != 6 {
		t.Fatalf("expected UTF-8 byte accounting, got %d", server.openDocumentBytes)
	}

	messages = notifyServer(t, server, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": "file:///three.gwdk", "version": 1, "text": "x",
	}})
	if len(messages) != 2 || len(server.documents) != 2 {
		t.Fatalf("expected open-document count rejection, messages=%d docs=%d", len(messages), len(server.documents))
	}

	notifyServer(t, server, "textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": "file:///two.gwdk"}})
	if server.openDocumentBytes != 4 || len(server.documents) != 1 {
		t.Fatalf("close accounting drifted: bytes=%d docs=%d", server.openDocumentBytes, len(server.documents))
	}
}

func notifyServer(t *testing.T, server *Server, method string, params any) [][]byte {
	t.Helper()
	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return server.handleNotification(rpcRequest{JSONRPC: jsonRPCVersion, Method: method, Params: payload})
}

func assertFramingCode(t *testing.T, err error, code string) {
	t.Helper()
	var framing *FramingError
	if !errors.As(err, &framing) || framing.Code != code || !framing.Fatal {
		t.Fatalf("expected fatal framing code %q, got %T %v", code, err, err)
	}
}

type oneByteReader struct {
	data []byte
}

func FuzzReadMessageWithLimits(f *testing.F) {
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Add([]byte("Content-Length: 999999999999999999999\r\n\r\n"))
	f.Add([]byte("X-Test: value\nContent-Length: 0\n\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		limits := Limits{
			MaxHeaderBytes:       256,
			MaxHeaderLineBytes:   64,
			MaxHeaderCount:       8,
			MaxMessageBytes:      128,
			MaxDocumentBytes:     128,
			MaxOpenDocumentBytes: 256,
			MaxOpenDocuments:     4,
		}
		_, _ = readMessageWithLimits(bufio.NewReaderSize(strings.NewReader(string(input)), 16), limits)
	})
}

func (reader *oneByteReader) Read(buffer []byte) (int, error) {
	if len(reader.data) == 0 {
		return 0, io.EOF
	}
	buffer[0] = reader.data[0]
	reader.data = reader.data[1:]
	return 1, nil
}
