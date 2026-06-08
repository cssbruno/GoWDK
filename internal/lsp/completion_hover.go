package lsp

import "github.com/cssbruno/gowdk/internal/lang"

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
	for _, doc := range server.documents {
		switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
		case lang.FileKindPage:
			page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			if page.ID != "" {
				items = append(items, completionItem{Label: page.ID, Kind: completionItemKindReference, Detail: "GOWDK page id"})
			}
			if page.Route != "" {
				items = append(items, completionItem{Label: page.Route, Kind: completionItemKindText, Detail: "GOWDK route"})
			}
			for _, guard := range page.Guard {
				items = append(items, completionItem{Label: guard, Kind: completionItemKindFunction, Detail: "GOWDK guard"})
			}
			for _, store := range page.Stores {
				items = append(items, completionItem{Label: store.Name, Kind: completionItemKindProperty, Detail: "GOWDK store"})
			}
		case lang.FileKindComponent:
			component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			if component.Name != "" {
				items = append(items, completionItem{Label: component.Name, Kind: completionItemKindClass, Detail: "GOWDK component"})
			}
			if doc.URI == currentURI {
				for _, prop := range component.Props {
					items = append(items, completionItem{Label: prop.Name, Kind: completionItemKindProperty, Detail: "component prop"})
				}
				for _, field := range inferredComponentFields(component.Blocks.ViewBody, component.Blocks.ClientBody) {
					items = append(items, completionItem{Label: field, Kind: completionItemKindProperty, Detail: "component state/value"})
				}
			}
		case lang.FileKindLayout:
			layout, diagnostics := lang.ParseLayoutSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() || layout.ID == "" {
				continue
			}
			items = append(items, completionItem{Label: layout.ID, Kind: completionItemKindReference, Detail: "GOWDK layout"})
		}
	}
	return items
}

func (server *Server) projectHoverItems(currentURI string) []completionItem {
	var items []completionItem
	for _, doc := range server.documents {
		switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
		case lang.FileKindPage:
			page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			for _, action := range page.Blocks.Actions {
				items = append(items, completionItem{Label: action.Name, Kind: completionItemKindFunction, Detail: "GOWDK action handler"})
			}
			for _, api := range page.Blocks.APIs {
				items = append(items, completionItem{Label: api.Name, Kind: completionItemKindFunction, Detail: "GOWDK API handler"})
			}
			for _, fragment := range page.Blocks.Fragments {
				items = append(items, completionItem{Label: fragment.Name, Kind: completionItemKindFunction, Detail: "GOWDK fragment handler"})
			}
		case lang.FileKindComponent:
			component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() || doc.URI != currentURI {
				continue
			}
			for _, emit := range component.Emits {
				items = append(items, completionItem{Label: emit.Name, Kind: completionItemKindFunction, Detail: "component event"})
			}
		}
	}
	return items
}
