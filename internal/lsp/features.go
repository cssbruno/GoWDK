package lsp

import (
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/lang"
)

func (server *Server) completionItems(params completionParams) []completionItem {
	completions := lang.Completions()
	items := make([]completionItem, 0, len(completions))
	for _, completion := range completions {
		items = append(items, completionItem{
			Label:  completion.Label,
			Kind:   completionItemKindKeyword,
			Detail: completion.Detail,
		})
	}
	return appendProjectCompletionItems(items, server.projectCompletions(params.TextDocument.URI))
}

func (server *Server) hover(params hoverParams) *hoverResult {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return nil
	}
	token := tokenAtPosition(doc.Text, params.Position)
	if token == "" {
		return nil
	}
	for _, item := range server.hoverItems(params.TextDocument.URI) {
		if item.Label == token {
			return &hoverResult{
				Contents: markupContent{
					Kind:  "markdown",
					Value: "**" + item.Label + "**\n\n" + item.Detail,
				},
			}
		}
	}
	return nil
}

func (server *Server) definition(params definitionParams) *location {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return nil
	}
	if name, ok := componentCallAtPosition(doc.Text, params.Position); ok {
		component, ok := server.resolveComponentDefinition(doc, name)
		if !ok {
			return nil
		}
		return &location{
			URI:   component.URI,
			Range: lspRangeFromSourceSpan(component.Span, component.Text),
		}
	}
	token := tokenAtPosition(doc.Text, params.Position)
	if token == "" {
		return nil
	}
	location, ok := server.goDefinition(token)
	if !ok {
		return nil
	}
	return &location
}

func (server *Server) references(params referenceParams) []location {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return []location{}
	}
	token := tokenAtPosition(doc.Text, params.Position)
	if token == "" {
		return []location{}
	}
	locations := make([]location, 0)
	for _, doc := range server.documents {
		for _, item := range referenceRanges(doc.Text, token) {
			locations = append(locations, location{URI: doc.URI, Range: item})
		}
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].URI != locations[j].URI {
			return locations[i].URI < locations[j].URI
		}
		if locations[i].Range.Start.Line != locations[j].Range.Start.Line {
			return locations[i].Range.Start.Line < locations[j].Range.Start.Line
		}
		return locations[i].Range.Start.Character < locations[j].Range.Start.Character
	})
	return locations
}

func (server *Server) codeActions(params codeActionParams) []codeAction {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return []codeAction{}
	}
	var actions []codeAction
	for _, diagnostic := range params.Context.Diagnostics {
		switch diagnostic.Code {
		case "old_action_block_syntax", "old_api_block_syntax":
			if action, ok := endpointMigrationCodeAction(params.TextDocument.URI, diagnostic); ok {
				actions = append(actions, action)
			}
		case "unknown_gowdk_use_alias":
			if action, ok := missingUseCodeAction(params.TextDocument.URI, doc.Text, diagnostic); ok {
				actions = append(actions, action)
			}
		}
	}
	if actions == nil {
		return []codeAction{}
	}
	return actions
}

func endpointMigrationCodeAction(uri string, item diagnostic) (codeAction, bool) {
	replacement, ok := endpointMigrationReplacement(item.Message)
	if !ok {
		return codeAction{}, false
	}
	return codeAction{
		Title:       "Replace old endpoint block header",
		Kind:        "quickfix",
		Diagnostics: []diagnostic{item},
		Edit: workspaceEdit{Changes: map[string][]textEdit{
			uri: {{
				Range:   item.Range,
				NewText: replacement,
			}},
		}},
	}, true
}

func missingUseCodeAction(uri string, source string, item diagnostic) (codeAction, bool) {
	alias, ok := missingUseAlias(item.Message)
	if !ok {
		return codeAction{}, false
	}
	insert := useInsertionPosition(source, item.Range.Start)
	return codeAction{
		Title:       `Add use ` + alias + ` "<package>"`,
		Kind:        "quickfix",
		Diagnostics: []diagnostic{item},
		Edit: workspaceEdit{Changes: map[string][]textEdit{
			uri: {{
				Range:   lspRange{Start: insert, End: insert},
				NewText: `use ` + alias + ` "<package>"` + "\n",
			}},
		}},
	}, true
}

func endpointMigrationReplacement(message string) (string, bool) {
	start := strings.Index(message, "use `")
	if start < 0 {
		return "", false
	}
	start += len("use `")
	end := strings.IndexByte(message[start:], '`')
	if end < 0 {
		return "", false
	}
	replacement := message[start : start+end]
	if strings.HasPrefix(replacement, "act ") || strings.HasPrefix(replacement, "api ") {
		if strings.Contains(message[start+end+1:], "and move behavior to Go") {
			return replacement, true
		}
	}
	return "", false
}

func missingUseAlias(message string) (string, bool) {
	start := strings.Index(message, "Add `use ")
	if start < 0 {
		return "", false
	}
	start += len("Add `use ")
	end := strings.Index(message[start:], ` "<package>"`)
	if end < 0 {
		return "", false
	}
	alias := message[start : start+end]
	if !isLSPIdentifier(alias) {
		return "", false
	}
	return alias, true
}

func useInsertionPosition(source string, fallback position) position {
	lines := strings.Split(source, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == "view {" {
			return position{Line: index, Character: 0}
		}
	}
	return fallback
}

func (server *Server) semanticTokens(params semanticTokensParams) semanticTokensResult {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return semanticTokensResult{Data: []int{}}
	}
	tokens, _ := lang.Lex(doc.Text)
	data := make([]int, 0, len(tokens)*5)
	previousLine := 0
	previousCharacter := 0
	seen := false
	for _, token := range tokens {
		tokenType, ok := semanticTokenType(token.Kind)
		if !ok || token.Lexeme == "" {
			continue
		}
		start := positionFromLangPosition(token.Pos, doc.Text)
		length := utf16Length(token.Lexeme)
		if length == 0 {
			continue
		}

		deltaLine := start.Line
		deltaStart := start.Character
		if seen {
			deltaLine = start.Line - previousLine
			if deltaLine == 0 {
				deltaStart = start.Character - previousCharacter
			}
		}
		data = append(data, deltaLine, deltaStart, length, semanticTokenTypeIndex[tokenType], 0)
		previousLine = start.Line
		previousCharacter = start.Character
		seen = true
	}
	return semanticTokensResult{Data: data}
}
