package lsp

import (
	"fmt"
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
		if messages := server.rejectDocumentUpdate(doc, false); messages != nil {
			return messages
		}
		server.storeDocument(doc)
		server.invalidateProjectCaches()
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didChange":
		var params didChangeTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didChange params: %v", err)
			return nil
		}
		doc, existed := server.documents[params.TextDocument.URI]
		doc.URI = params.TextDocument.URI
		doc.Path = pathFromURI(params.TextDocument.URI)
		doc.Version = params.TextDocument.Version
		if len(params.ContentChanges) > 0 {
			doc.Text = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		if messages := server.rejectDocumentUpdate(doc, existed); messages != nil {
			return messages
		}
		server.storeDocument(doc)
		server.invalidateProjectCaches()
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
			candidate := doc
			candidate.Text = *params.Text
			if messages := server.rejectDocumentUpdate(candidate, true); messages != nil {
				return messages
			}
			doc = candidate
			server.storeDocument(doc)
		}
		server.invalidateProjectCaches()
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didClose":
		var params didCloseTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didClose params: %v", err)
			return nil
		}
		if doc, ok := server.documents[params.TextDocument.URI]; ok {
			server.openDocumentBytes -= int64(len(doc.Text))
			delete(server.documents, params.TextDocument.URI)
		}
		server.invalidateProjectCaches()
		return singleMessage(publishDiagnostics(params.TextDocument.URI, nil))
	default:
		return nil
	}
}

func (server *Server) storeDocument(doc document) {
	if previous, ok := server.documents[doc.URI]; ok {
		server.openDocumentBytes -= int64(len(previous.Text))
	}
	server.documents[doc.URI] = doc
	server.openDocumentBytes += int64(len(doc.Text))
}

func (server *Server) rejectDocumentUpdate(doc document, replacing bool) [][]byte {
	bytes := int64(len(doc.Text))
	code := ""
	message := ""
	switch {
	case bytes > server.limits.MaxDocumentBytes:
		code = "lsp_document_too_large"
		message = fmt.Sprintf("GOWDK analysis skipped: document is %d bytes; limit is %d bytes", bytes, server.limits.MaxDocumentBytes)
	case !replacing && len(server.documents) >= server.limits.MaxOpenDocuments:
		code = "lsp_too_many_documents"
		message = fmt.Sprintf("GOWDK analysis skipped: open-document limit is %d", server.limits.MaxOpenDocuments)
	default:
		retained := server.openDocumentBytes + bytes
		if previous, ok := server.documents[doc.URI]; ok {
			retained -= int64(len(previous.Text))
		}
		if retained > server.limits.MaxOpenDocumentBytes {
			code = "lsp_open_documents_too_large"
			message = fmt.Sprintf("GOWDK analysis skipped: retained open-document text would exceed %d bytes", server.limits.MaxOpenDocumentBytes)
		}
	}
	if code == "" {
		return nil
	}
	server.logf("lsp document rejected code=%s bytes=%d retained=%d", code, bytes, server.openDocumentBytes)
	return [][]byte{
		publishDiagnostics(doc.URI, nil),
		notification("window/showMessage", map[string]any{"type": 2, "message": message + " (" + code + ")"}),
	}
}

func (server *Server) publishDiagnostics(doc document) []byte {
	if !strings.HasSuffix(doc.Path, ".gwdk") {
		return publishDiagnostics(doc.URI, nil)
	}
	_, diagnostics := lang.CheckSourceWithOptions(server.config, doc.Path, []byte(doc.Text), lang.CheckOptions{ProjectRoot: server.workspaceRootForPath(doc.Path)})
	items := make([]diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		items = append(items, diagnosticFromLang(item, doc.URI, doc.Text))
	}
	return publishDiagnostics(doc.URI, items)
}
