package lsp

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/diagnosticfix"
	"github.com/cssbruno/gowdk/internal/diagnostics"
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
	items = appendProjectCompletionItems(items, server.projectCompletions(params.TextDocument.URI))
	if doc, ok := server.documents[params.TextDocument.URI]; ok {
		items = append(items, storePersistCompletionItems(linePrefixAt(doc.Text, params.Position))...)
	}
	return items
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
	if hover := storePersistHover(doc.Text, params.Position, token); hover != nil {
		return hover
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
		fix, ok := diagnostics.FixFor(diagnostic.Code)
		if !ok {
			continue
		}
		if action, ok := registryFixCodeAction(params.TextDocument.URI, doc.Text, diagnostic, fix); ok {
			actions = append(actions, action)
		}
	}
	if actions == nil {
		return []codeAction{}
	}
	return actions
}

func registryFixCodeAction(uri string, sourceText string, item diagnostic, fix diagnostics.Fix) (codeAction, bool) {
	edits, err := diagnosticfix.Edits(fix, sourceText, diagnosticfix.Diagnostic{
		Code:    item.Code,
		Message: item.Message,
		Range:   fixRangeFromLSP(item.Range),
	})
	if err != nil || len(edits) == 0 {
		return codeAction{}, false
	}
	textEdits := make([]textEdit, 0, len(edits))
	for _, edit := range edits {
		textEdits = append(textEdits, textEdit{
			Range:   lspRangeFromFix(edit.Range),
			NewText: edit.NewText,
		})
	}
	return codeAction{
		Title:       fix.Title,
		Kind:        "quickfix",
		Diagnostics: []diagnostic{item},
		Edit:        workspaceEdit{Changes: map[string][]textEdit{uri: textEdits}},
	}, true
}

func fixRangeFromLSP(item lspRange) diagnosticfix.Range {
	return diagnosticfix.Range{
		Start: diagnosticfix.Position{Line: item.Start.Line + 1, Column: item.Start.Character + 1},
		End:   diagnosticfix.Position{Line: item.End.Line + 1, Column: item.End.Character + 1},
	}
}

func lspRangeFromFix(item diagnosticfix.Range) lspRange {
	return lspRange{
		Start: positionFromFix(item.Start),
		End:   positionFromFix(item.End),
	}
}

func positionFromFix(item diagnosticfix.Position) position {
	line := item.Line - 1
	if line < 0 {
		line = 0
	}
	character := item.Column - 1
	if character < 0 {
		character = 0
	}
	return position{Line: line, Character: character}
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
		if token.Kind == lang.TokenIdentifier && token.Lexeme == "persist" && isStorePersistKeyword(doc.Text, start.Line) {
			tokenType = "decorator"
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
