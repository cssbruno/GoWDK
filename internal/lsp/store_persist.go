package lsp

import "strings"

// persistHoverMarkdown explains the store persist modifier and points at the
// explain entry that documents the valid scopes.
const persistHoverMarkdown = "**persist**\n\n" +
	"Persist a page store to browser storage. Use `persist \"local\"` for " +
	"`window.localStorage` (survives a browser restart) or `persist \"session\"` " +
	"for `window.sessionStorage` (survives reload and SPA navigation, cleared when " +
	"the tab closes). No other scope is supported.\n\n" +
	"See: `gowdk explain page_store_persist_scope_invalid`"

// storePersistCompletionItems offers persist-modifier completions based on the
// store declaration text typed before the cursor: `persist` once the store
// initializer call is complete, then the `"local"` / `"session"` scopes once
// `persist` has been typed.
func storePersistCompletionItems(linePrefix string) []completionItem {
	if !strings.HasPrefix(strings.TrimSpace(linePrefix), "store ") {
		return nil
	}
	closeParen := strings.LastIndex(linePrefix, ")")
	if closeParen < 0 {
		// The initializer call is not complete yet; nothing to suggest.
		return nil
	}
	afterInit := linePrefix[closeParen+1:]
	if strings.Contains(afterInit, "persist") {
		// Past the persist keyword: suggest the scopes until one is typed.
		if strings.Contains(afterInit, `"`) {
			return nil
		}
		return []completionItem{
			{Label: `"local"`, Kind: completionItemKindText, Detail: "Persist to window.localStorage (survives a browser restart)."},
			{Label: `"session"`, Kind: completionItemKindText, Detail: "Persist to window.sessionStorage (cleared when the tab closes)."},
		}
	}
	return []completionItem{
		{Label: "persist", Kind: completionItemKindKeyword, Detail: `Persist the store to browser storage: persist "local" or persist "session".`},
	}
}

// storePersistHover returns rich hover text when the cursor is on the persist
// keyword or one of its scope literals within a store declaration.
func storePersistHover(text string, pos position, token string) *hoverResult {
	switch token {
	case "persist", "local", "session":
	default:
		return nil
	}
	line := lineTextAt(text, pos.Line)
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "store ") || !strings.Contains(line, "persist") {
		return nil
	}
	return &hoverResult{Contents: markupContent{Kind: "markdown", Value: persistHoverMarkdown}}
}

// isStorePersistKeyword reports whether the persist identifier token at line is
// the store persist modifier, so semantic highlighting can classify it as a
// keyword rather than a plain identifier.
func isStorePersistKeyword(text string, line int) bool {
	trimmed := strings.TrimSpace(lineTextAt(text, line))
	return strings.HasPrefix(trimmed, "store ") && strings.Contains(trimmed, "persist")
}

func linePrefixAt(text string, pos position) string {
	line := lineTextAt(text, pos.Line)
	index := byteIndexFromUTF16Column(line, pos.Character)
	if index > len(line) {
		index = len(line)
	}
	return line[:index]
}

func lineTextAt(text string, line int) string {
	if line < 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return ""
	}
	return lines[line]
}
