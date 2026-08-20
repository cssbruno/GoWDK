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
	projectRoot             string
	moduleNames             []string
	documents               map[string]document
	openDocumentBytes       int64
	limits                  Limits
	projectCache            projectIRCache
	workspaceComponentCache workspaceComponentDefinitionCache
	shutdown                bool
	log                     io.Writer
}

// Limits bounds one LSP session. Byte limits count UTF-8 bytes, matching LSP
// transport framing and Go string storage rather than Unicode code points.
type Limits struct {
	MaxHeaderBytes       int64
	MaxHeaderLineBytes   int
	MaxHeaderCount       int
	MaxMessageBytes      int64
	MaxDocumentBytes     int64
	MaxOpenDocumentBytes int64
	MaxOpenDocuments     int
}

// DefaultLimits returns generous finite defaults for ordinary GOWDK projects.
func DefaultLimits() Limits {
	return Limits{
		MaxHeaderBytes:       64 << 10,
		MaxHeaderLineBytes:   8 << 10,
		MaxHeaderCount:       64,
		MaxMessageBytes:      16 << 20,
		MaxDocumentBytes:     8 << 20,
		MaxOpenDocumentBytes: 64 << 20,
		MaxOpenDocuments:     256,
	}
}

func (limits Limits) normalized() Limits {
	defaults := DefaultLimits()
	if limits.MaxHeaderBytes <= 0 {
		limits.MaxHeaderBytes = defaults.MaxHeaderBytes
	}
	if limits.MaxHeaderLineBytes <= 0 {
		limits.MaxHeaderLineBytes = defaults.MaxHeaderLineBytes
	}
	if limits.MaxHeaderCount <= 0 {
		limits.MaxHeaderCount = defaults.MaxHeaderCount
	}
	if limits.MaxMessageBytes <= 0 {
		limits.MaxMessageBytes = defaults.MaxMessageBytes
	}
	if limits.MaxDocumentBytes <= 0 {
		limits.MaxDocumentBytes = defaults.MaxDocumentBytes
	}
	if limits.MaxOpenDocumentBytes <= 0 {
		limits.MaxOpenDocumentBytes = defaults.MaxOpenDocumentBytes
	}
	if limits.MaxOpenDocuments <= 0 {
		limits.MaxOpenDocuments = defaults.MaxOpenDocuments
	}
	return limits
}

type document struct {
	URI     string
	Path    string
	Version int
	Text    string
}

// ProjectOptions scopes configured source discovery for one LSP session.
type ProjectOptions struct {
	Root    string
	Modules []string
	Limits  Limits
}

// NewServer returns a language server using the provided compiler config.
func NewServer(config gowdk.Config) *Server {
	return NewProjectServer(config, ProjectOptions{})
}

// NewProjectServer returns a language server scoped to one configured project.
func NewProjectServer(config gowdk.Config, options ProjectOptions) *Server {
	return &Server{
		config:      config,
		projectRoot: options.Root,
		moduleNames: append([]string(nil), options.Modules...),
		documents:   map[string]document{},
		limits:      options.Limits.normalized(),
		log:         os.Stderr,
	}
}

// Serve runs the JSON-RPC message loop until the client sends exit or input closes.
func (server *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readMessageWithLimits(reader, server.limits)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			var framing *FramingError
			if errors.As(err, &framing) {
				server.logf("lsp framing rejected code=%s actual=%d limit=%d", framing.Code, framing.Actual, framing.Limit)
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
