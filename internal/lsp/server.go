// Package lsp implements the GOWDK Language Server Protocol entrypoint.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cssbruno/gowdk"
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

	completionItemKindText      = 1
	completionItemKindFunction  = 3
	completionItemKindClass     = 7
	completionItemKindProperty  = 10
	completionItemKindReference = 18
	completionItemKindKeyword   = 14
)

var (
	semanticTokenTypes     = []string{"decorator", "variable", "string", "operator"}
	semanticTokenTypeIndex = map[string]int{"decorator": 0, "variable": 1, "string": 2, "operator": 3}
)

// Server handles one LSP session.
type Server struct {
	config                  gowdk.Config
	documents               map[string]document
	projectCache            projectIRCache
	workspaceComponentCache workspaceComponentDefinitionCache
	shutdown                bool
	log                     io.Writer
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
