package lsp

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/lang"
)

func (server *Server) hoverItems(currentURI string) []completionItem {
	items := server.completionItems(completionParams{
		TextDocument: textDocumentIdentifier{URI: currentURI},
	})
	return appendProjectCompletionItems(items, server.projectHoverItems(currentURI))
}

func appendProjectCompletionItems(items []completionItem, project []completionItem) []completionItem {
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Label+"\x00"+item.Detail] = true
	}
	for _, item := range project {
		key := item.Label + "\x00" + item.Detail
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}
	return items
}

func (server *Server) projectCompletions(currentURI string) []completionItem {
	var items []completionItem
	ir, docsBySource := server.openProjectIR()
	for _, page := range ir.Pages {
		if page.ID != "" {
			items = append(items, completionItem{Label: page.ID, Kind: completionItemKindReference, Detail: "GOWDK page id"})
		}
		if page.Route != "" {
			items = append(items, completionItem{Label: page.Route, Kind: completionItemKindText, Detail: "GOWDK route"})
		}
		for _, guard := range page.Guards {
			items = append(items, completionItem{Label: guard, Kind: completionItemKindFunction, Detail: "GOWDK guard"})
		}
		for _, store := range page.Stores {
			items = append(items, completionItem{Label: store.Name, Kind: completionItemKindProperty, Detail: "GOWDK store"})
		}
	}
	for _, component := range ir.Components {
		if component.Name != "" {
			items = append(items, completionItem{Label: component.Name, Kind: completionItemKindClass, Detail: "GOWDK component"})
		}
		if doc, ok := docsBySource[component.Source]; ok && doc.URI == currentURI {
			for _, prop := range component.Props {
				items = append(items, completionItem{Label: prop.Name, Kind: completionItemKindProperty, Detail: "component prop"})
			}
			for _, field := range inferredComponentFields(component.Blocks.ViewBody, component.Blocks.ClientBody) {
				items = append(items, completionItem{Label: field, Kind: completionItemKindProperty, Detail: "component state/value"})
			}
		}
	}
	for _, layout := range ir.Layouts {
		if layout.ID != "" {
			items = append(items, completionItem{Label: layout.ID, Kind: completionItemKindReference, Detail: "GOWDK layout"})
		}
	}
	return items
}

func (server *Server) projectHoverItems(currentURI string) []completionItem {
	var items []completionItem
	ir, docsBySource := server.openProjectIR()
	for _, endpoint := range ir.Endpoints {
		if endpoint.Symbol == "" {
			continue
		}
		switch endpoint.Kind {
		case gwdkir.EndpointAction:
			items = append(items, completionItem{Label: endpoint.Symbol, Kind: completionItemKindFunction, Detail: "GOWDK action handler"})
		case gwdkir.EndpointAPI:
			items = append(items, completionItem{Label: endpoint.Symbol, Kind: completionItemKindFunction, Detail: "GOWDK API handler"})
		case gwdkir.EndpointFragment:
			items = append(items, completionItem{Label: endpoint.Symbol, Kind: completionItemKindFunction, Detail: "GOWDK fragment handler"})
		}
	}
	for _, component := range ir.Components {
		doc, ok := docsBySource[component.Source]
		if !ok || doc.URI != currentURI {
			continue
		}
		for _, emit := range component.Emits {
			items = append(items, completionItem{Label: emit.Name, Kind: completionItemKindFunction, Detail: "component event"})
		}
	}
	return items
}

func (server *Server) openProjectIR() (gwdkir.Program, map[string]document) {
	key := server.openProjectIRCacheKey()
	if server.projectCache.key == key {
		return server.projectCache.ir, cloneDocumentsBySource(server.projectCache.docsBySource)
	}

	var sources gwdkanalysis.Sources
	docsBySource := map[string]document{}
	for _, doc := range server.documents {
		source := []byte(doc.Text)
		switch lang.ClassifySource(doc.Path, source) {
		case lang.FileKindPage:
			page, diagnostics := lang.ParseSource(doc.Path, source)
			if diagnostics.HasErrors() {
				continue
			}
			sources.Pages = append(sources.Pages, page)
			docsBySource[page.Source] = doc
		case lang.FileKindComponent:
			component, diagnostics := lang.ParseComponentSource(doc.Path, source)
			if diagnostics.HasErrors() {
				continue
			}
			sources.Components = append(sources.Components, component)
			docsBySource[component.Source] = doc
		case lang.FileKindLayout:
			layout, diagnostics := lang.ParseLayoutSource(doc.Path, source)
			if diagnostics.HasErrors() {
				continue
			}
			sources.Layouts = append(sources.Layouts, layout)
			docsBySource[layout.Source] = doc
		}
	}
	ir := gwdkanalysis.BuildProgram(server.config, sources)
	server.projectCache = projectIRCache{
		key:          key,
		ir:           ir,
		docsBySource: cloneDocumentsBySource(docsBySource),
	}
	return ir, docsBySource
}

func (server *Server) openProjectIRCacheKey() string {
	parts := make([]string, 0, len(server.documents))
	for _, doc := range server.documents {
		if !strings.HasSuffix(doc.Path, ".gwdk") {
			continue
		}
		sum := sha256.Sum256([]byte(doc.Text))
		parts = append(parts, fmt.Sprintf("%s\x00%s\x00%d\x00%x", doc.URI, doc.Path, doc.Version, sum))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

func cloneDocumentsBySource(docs map[string]document) map[string]document {
	if len(docs) == 0 {
		return map[string]document{}
	}
	clone := make(map[string]document, len(docs))
	for source, doc := range docs {
		clone[source] = doc
	}
	return clone
}
