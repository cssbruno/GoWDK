package lsp

import "github.com/cssbruno/gowdk/internal/lang"

// LSP SymbolKind values (subset) from the language server protocol.
const (
	symbolKindModule   = 2
	symbolKindPackage  = 4
	symbolKindClass    = 5
	symbolKindMethod   = 6
	symbolKindProperty = 7
	symbolKindField    = 8
)

// documentSymbols returns the top-level outline of a .gwdk document, parsed by
// the recursive-descent outline pass over the shared tokenizer.
func (server *Server) documentSymbols(params documentSymbolParams) []documentSymbol {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return []documentSymbol{}
	}

	outline := lang.Outline(doc.Text)
	symbols := make([]documentSymbol, 0, len(outline))
	for _, item := range outline {
		rng := lspRangeFromSourceSpan(item.Span, doc.Text)
		symbols = append(symbols, documentSymbol{
			Name:           item.Name,
			Detail:         item.Detail,
			Kind:           outlineSymbolKind(item.Kind),
			Range:          rng,
			SelectionRange: rng,
		})
	}
	return symbols
}

func outlineSymbolKind(kind lang.OutlineKind) int {
	switch kind {
	case lang.OutlineKindPackage:
		return symbolKindPackage
	case lang.OutlineKindComponent:
		return symbolKindClass
	case lang.OutlineKindPage:
		return symbolKindModule
	case lang.OutlineKindEndpoint:
		return symbolKindMethod
	case lang.OutlineKindBlock:
		return symbolKindField
	case lang.OutlineKindImport, lang.OutlineKindUse:
		return symbolKindModule
	default:
		return symbolKindProperty
	}
}
