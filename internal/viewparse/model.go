package viewparse

import (
	stdhtml "html"
	"strings"

	"github.com/cssbruno/gowdk/internal/viewmodel"
)

type Attr = viewmodel.Attr
type AwaitBlock = viewmodel.AwaitBlock
type ComponentCall = viewmodel.ComponentCall
type Element = viewmodel.Element
type Node = viewmodel.Node
type Text = viewmodel.Text

const (
	escapedOpenBrace  = "\x00GOWDK_OPEN_BRACE\x00"
	escapedCloseBrace = "\x00GOWDK_CLOSE_BRACE\x00"
)

func decodeSourceTextEntities(value string) string {
	if !strings.Contains(value, "&") {
		return value
	}
	value = strings.ReplaceAll(value, "&#123;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#x7b;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#X7B;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#125;", escapedCloseBrace)
	value = strings.ReplaceAll(value, "&#x7d;", escapedCloseBrace)
	value = strings.ReplaceAll(value, "&#X7D;", escapedCloseBrace)
	return stdhtml.UnescapeString(value)
}

var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char == '_':
		case index > 0 && char >= '0' && char <= '9':
		default:
			return false
		}
	}
	return true
}
