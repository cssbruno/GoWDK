// Package lsp implements the GOWDK Language Server Protocol entrypoint.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/lang"
)

const (
	jsonRPCVersion = "2.0"

	parseError     = -32700
	invalidRequest = -32600
	methodNotFound = -32601
	invalidParams  = -32602
	internalError  = -32603

	textDocumentSyncFull = 1

	diagnosticSeverityError   = 1
	diagnosticSeverityWarning = 2

	completionItemKindKeyword = 14
)

// Server handles one LSP session.
type Server struct {
	config    gowdk.Config
	documents map[string]document
	shutdown  bool
	log       io.Writer
}

type document struct {
	URI     string
	Path    string
	Version int
	Text    string
}

// NewServer returns a language server using the provided compiler config.
func NewServer(config gowdk.Config) *Server {
	return &Server{
		config:    config,
		documents: map[string]document{},
		log:       os.Stderr,
	}
}

// Serve runs the JSON-RPC message loop until the client sends exit or input closes.
func (server *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		messages, exit := server.handle(body)
		for _, message := range messages {
			if err := writeMessage(out, message); err != nil {
				return err
			}
		}
		if exit {
			return nil
		}
	}
}

func (server *Server) handle(body []byte) ([][]byte, bool) {
	var request rpcRequest
	if err := json.Unmarshal(body, &request); err != nil {
		server.logf("invalid lsp message: %v", err)
		return singleMessage(errorResponse(nil, parseError, fmt.Sprintf("parse error: %v", err))), false
	}

	if request.Method == "exit" {
		return nil, true
	}
	if request.Method == "" {
		if request.ID != nil {
			return singleMessage(errorResponse(request.ID, invalidRequest, "missing method")), false
		}
		return nil, false
	}
	if request.ID == nil {
		return server.handleNotification(request), false
	}
	return server.handleRequest(request), false
}

func (server *Server) handleRequest(request rpcRequest) [][]byte {
	switch request.Method {
	case "initialize":
		return singleMessage(response(request.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync: textDocumentSyncOptions{
					OpenClose: true,
					Change:    textDocumentSyncFull,
					Save:      saveOptions{IncludeText: true},
				},
				DocumentFormattingProvider: true,
				CompletionProvider: completionOptions{
					TriggerCharacters: []string{"@", ":", "<", " "},
				},
			},
			ServerInfo: serverInfo{
				Name:    "gowdk",
				Version: "0.1.0-dev",
			},
		}))
	case "shutdown":
		server.shutdown = true
		return singleMessage(response(request.ID, nil))
	case "textDocument/formatting":
		var params documentFormattingParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		doc, ok := server.documents[params.TextDocument.URI]
		if !ok {
			return singleMessage(response(request.ID, []textEdit{}))
		}
		formatted := string(lang.Format([]byte(doc.Text)))
		if formatted == doc.Text {
			return singleMessage(response(request.ID, []textEdit{}))
		}
		return singleMessage(response(request.ID, []textEdit{{
			Range:   fullRange(doc.Text),
			NewText: formatted,
		}}))
	case "textDocument/completion":
		return singleMessage(response(request.ID, completionList{
			IsIncomplete: false,
			Items:        completionItems(),
		}))
	default:
		return singleMessage(errorResponse(request.ID, methodNotFound, fmt.Sprintf("method not found: %s", request.Method)))
	}
}

func (server *Server) handleNotification(request rpcRequest) [][]byte {
	switch request.Method {
	case "initialized":
		return nil
	case "textDocument/didOpen":
		var params didOpenTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didOpen params: %v", err)
			return nil
		}
		doc := document{
			URI:     params.TextDocument.URI,
			Path:    pathFromURI(params.TextDocument.URI),
			Version: params.TextDocument.Version,
			Text:    params.TextDocument.Text,
		}
		server.documents[doc.URI] = doc
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didChange":
		var params didChangeTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didChange params: %v", err)
			return nil
		}
		doc := server.documents[params.TextDocument.URI]
		doc.URI = params.TextDocument.URI
		doc.Path = pathFromURI(params.TextDocument.URI)
		doc.Version = params.TextDocument.Version
		if len(params.ContentChanges) > 0 {
			doc.Text = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		server.documents[doc.URI] = doc
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didSave":
		var params didSaveTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didSave params: %v", err)
			return nil
		}
		doc, ok := server.documents[params.TextDocument.URI]
		if !ok {
			return nil
		}
		if params.Text != nil {
			doc.Text = *params.Text
			server.documents[doc.URI] = doc
		}
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didClose":
		var params didCloseTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didClose params: %v", err)
			return nil
		}
		delete(server.documents, params.TextDocument.URI)
		return singleMessage(publishDiagnostics(params.TextDocument.URI, nil))
	default:
		return nil
	}
}

func (server *Server) publishDiagnostics(doc document) []byte {
	_, diagnostics := lang.CheckSource(server.config, doc.Path, []byte(doc.Text))
	items := make([]diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		items = append(items, diagnosticFromLang(item, doc.Text))
	}
	return publishDiagnostics(doc.URI, items)
}

func completionItems() []completionItem {
	completions := lang.Completions()
	items := make([]completionItem, 0, len(completions))
	for _, completion := range completions {
		items = append(items, completionItem{
			Label:  completion.Label,
			Kind:   completionItemKindKeyword,
			Detail: completion.Detail,
		})
	}
	return items
}

func diagnosticFromLang(item lang.Diagnostic, source string) diagnostic {
	severity := diagnosticSeverityError
	if item.Severity == "warning" {
		severity = diagnosticSeverityWarning
	}
	return diagnostic{
		Range:    rangeFromLangDiagnostic(item, source),
		Severity: severity,
		Code:     item.Code,
		Source:   "gowdk",
		Message:  item.Message,
	}
}

func rangeFromLangDiagnostic(item lang.Diagnostic, source string) lspRange {
	if item.Range != nil {
		return rangeFromLangRange(*item.Range, source)
	}
	return rangeFromPosition(item.Pos, source)
}

func rangeFromLangRange(item lang.Range, source string) lspRange {
	start := positionFromLangPosition(item.Start, source)
	end := positionFromLangPosition(item.End, source)
	if end.Line < start.Line || (end.Line == start.Line && end.Character <= start.Character) {
		end = position{Line: start.Line, Character: start.Character + 1}
	}
	return lspRange{Start: start, End: end}
}

func rangeFromPosition(pos lang.Position, source string) lspRange {
	if pos.Line <= 0 {
		return lspRange{
			Start: position{Line: 0, Character: 0},
			End:   position{Line: 0, Character: 1},
		}
	}

	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	end := character + 1
	if end > lineLength {
		end = character
	}
	return lspRange{
		Start: position{Line: lineIndex, Character: character},
		End:   position{Line: lineIndex, Character: end},
	}
}

func positionFromLangPosition(pos lang.Position, source string) position {
	if pos.Line <= 0 {
		return position{Line: 0, Character: 0}
	}
	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	return position{Line: lineIndex, Character: character}
}

func fullRange(text string) lspRange {
	lines := strings.Split(text, "\n")
	lastLine := len(lines) - 1
	return lspRange{
		Start: position{Line: 0, Character: 0},
		End: position{
			Line:      lastLine,
			Character: utf16Length(lines[lastLine]),
		},
	}
}

func utf16Column(line string, oneBasedColumn int) int {
	if oneBasedColumn <= 0 {
		return 0
	}
	runes := []rune(line)
	if oneBasedColumn > len(runes) {
		oneBasedColumn = len(runes)
	}
	return len(utf16.Encode(runes[:oneBasedColumn]))
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func clamp(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func publishDiagnostics(uri string, diagnostics []diagnostic) []byte {
	if diagnostics == nil {
		diagnostics = []diagnostic{}
	}
	return notification("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func singleMessage(message []byte) [][]byte {
	if len(message) == 0 {
		return nil
	}
	return [][]byte{message}
}

func response(id *json.RawMessage, result any) []byte {
	payload, err := json.Marshal(rpcResultResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
	if err != nil {
		return errorResponse(id, internalError, err.Error())
	}
	return payload
}

func errorResponse(id *json.RawMessage, code int, message string) []byte {
	payload, err := json.Marshal(rpcErrorResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return nil
	}
	return payload
}

func notification(method string, params any) []byte {
	payload, err := json.Marshal(rpcNotification{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil
	}
	return payload
}

func pathFromURI(rawURI string) string {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "file" {
		return rawURI
	}
	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(parsed.Path, "/")
	}
	return parsed.Path
}

func (server *Server) logf(format string, args ...any) {
	if server.log == nil {
		return
	}
	fmt.Fprintf(server.log, format+"\n", args...)
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q", value)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeMessage(writer io.Writer, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResultResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result"`
}

type rpcErrorResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *rpcError        `json:"error"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	TextDocumentSync           textDocumentSyncOptions `json:"textDocumentSync"`
	DocumentFormattingProvider bool                    `json:"documentFormattingProvider"`
	CompletionProvider         completionOptions       `json:"completionProvider"`
}

type textDocumentSyncOptions struct {
	OpenClose bool        `json:"openClose"`
	Change    int         `json:"change"`
	Save      saveOptions `json:"save"`
}

type saveOptions struct {
	IncludeText bool `json:"includeText"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type didOpenTextDocumentParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeTextDocumentParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type didSaveTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

type didCloseTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentFormattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}
