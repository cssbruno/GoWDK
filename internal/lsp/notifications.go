package lsp

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/lang"
)

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
	if !strings.HasSuffix(doc.Path, ".gwdk") {
		return publishDiagnostics(doc.URI, nil)
	}
	_, diagnostics := lang.CheckSource(server.config, doc.Path, []byte(doc.Text))
	items := make([]diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		items = append(items, diagnosticFromLang(item, doc.Text))
	}
	return publishDiagnostics(doc.URI, items)
}
