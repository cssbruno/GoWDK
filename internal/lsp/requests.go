package lsp

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/lang"
)

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
				HoverProvider:              true,
				DefinitionProvider:         true,
				ReferencesProvider:         true,
				CodeActionProvider:         true,
				DocumentFormattingProvider: true,
				CompletionProvider: completionOptions{
					TriggerCharacters: []string{"@", ":", "<", " "},
				},
				SemanticTokensProvider: semanticTokensOptions{
					Legend: semanticTokensLegend{
						TokenTypes:     semanticTokenTypes,
						TokenModifiers: []string{},
					},
					Full: true,
				},
			},
			ServerInfo: serverInfo{
				Name:    "gowdk",
				Version: "0.1.5",
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
		var params completionParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, completionList{
			IsIncomplete: false,
			Items:        server.completionItems(params),
		}))
	case "textDocument/hover":
		var params hoverParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.hover(params)))
	case "textDocument/definition":
		var params definitionParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.definition(params)))
	case "textDocument/references":
		var params referenceParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.references(params)))
	case "textDocument/codeAction":
		var params codeActionParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.codeActions(params)))
	case "textDocument/semanticTokens/full":
		var params semanticTokensParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.semanticTokens(params)))
	default:
		return singleMessage(errorResponse(request.ID, methodNotFound, fmt.Sprintf("method not found: %s", request.Method)))
	}
}
