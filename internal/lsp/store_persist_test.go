package lsp

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

const persistStorePage = `package app
import ui "github.com/cssbruno/gowdk/testfixture/islands"

page home
route "/products"

store cart ui.CounterState = ui.NewCounterState() persist "local"

view {
  <main></main>
}
`

func hasItemLabel(items []completionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}

func TestStorePersistCompletionItems(t *testing.T) {
	const initializer = "store cart ui.CounterState = ui.NewCounterState()"

	if items := storePersistCompletionItems(initializer); !hasItemLabel(items, "persist") {
		t.Fatalf("expected persist completion after a complete store initializer, got %#v", items)
	}
	scopes := storePersistCompletionItems(initializer + " persist ")
	if !hasItemLabel(scopes, `"local"`) || !hasItemLabel(scopes, `"session"`) {
		t.Fatalf("expected local/session completions after persist, got %#v", scopes)
	}
	if items := storePersistCompletionItems(initializer + ` persist "local"`); len(items) != 0 {
		t.Fatalf("expected no completions once a scope is typed, got %#v", items)
	}
	if items := storePersistCompletionItems("store cart ui.CounterState = ui.NewCounterState"); len(items) != 0 {
		t.Fatalf("expected no completions before the initializer call closes, got %#v", items)
	}
	if items := storePersistCompletionItems(`route "/products"`); len(items) != 0 {
		t.Fatalf("expected no persist completions outside a store declaration, got %#v", items)
	}
}

func TestServerCompletionOffersPersistAfterStoreInitializer(t *testing.T) {
	server := NewServer(gowdk.Config{})
	server.log = nil
	uri := "file:///tmp/home.page.gwdk"
	server.documents[uri] = document{URI: uri, Path: "/tmp/home.page.gwdk", Version: 1, Text: persistStorePage}

	items := server.completionItems(completionParams{
		TextDocument: textDocumentIdentifier{URI: uri},
		Position:     position{Line: 6, Character: 49},
	})
	if !hasItemLabel(items, "persist") {
		t.Fatalf("expected the server to offer persist after the store initializer, got %#v", items)
	}
}

func TestServerHoverExplainsPersistModifier(t *testing.T) {
	server := NewServer(gowdk.Config{})
	server.log = nil
	uri := "file:///tmp/home.page.gwdk"
	server.documents[uri] = document{URI: uri, Path: "/tmp/home.page.gwdk", Version: 1, Text: persistStorePage}

	hover := server.hover(hoverParams{
		TextDocument: textDocumentIdentifier{URI: uri},
		Position:     position{Line: 6, Character: 53},
	})
	if hover == nil {
		t.Fatal("expected hover for the persist keyword")
	}
	if !strings.Contains(hover.Contents.Value, "page_store_persist_scope_invalid") {
		t.Fatalf("expected persist hover to link the explain code, got %q", hover.Contents.Value)
	}

	// A non-store line with the same word must not produce the store hover.
	if got := storePersistHover("route \"/products\"", position{Line: 0, Character: 2}, "persist"); got != nil {
		t.Fatalf("expected no persist hover outside a store declaration, got %#v", got)
	}
}

func TestSemanticTokensClassifyPersistAsDecorator(t *testing.T) {
	server := NewServer(gowdk.Config{})
	server.log = nil
	uri := "file:///tmp/home.page.gwdk"
	server.documents[uri] = document{URI: uri, Path: "/tmp/home.page.gwdk", Version: 1, Text: persistStorePage}

	result := server.semanticTokens(semanticTokensParams{TextDocument: textDocumentIdentifier{URI: uri}})
	tokens := decodeSemanticTokens(result.Data)

	var found bool
	for _, token := range tokens {
		if token.line == 6 && token.length == len("persist") && token.typeIndex == semanticTokenTypeIndex["decorator"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected persist on the store line to be a decorator token, got %#v", tokens)
	}
}

type decodedToken struct {
	line      int
	character int
	length    int
	typeIndex int
}

func decodeSemanticTokens(data []int) []decodedToken {
	var tokens []decodedToken
	line, character := 0, 0
	for index := 0; index+5 <= len(data); index += 5 {
		deltaLine, deltaStart, length, typeIndex := data[index], data[index+1], data[index+2], data[index+3]
		if deltaLine == 0 {
			character += deltaStart
		} else {
			line += deltaLine
			character = deltaStart
		}
		tokens = append(tokens, decodedToken{line: line, character: character, length: length, typeIndex: typeIndex})
	}
	return tokens
}
