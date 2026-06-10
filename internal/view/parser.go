package view

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type parser struct {
	source []rune
	index  int
}

func (parser *parser) nodes(until string) ([]Node, error) {
	var nodes []Node
	for {
		if parser.done() {
			if until != "" {
				return nil, parser.errorf("missing closing tag </%s>", until)
			}
			return nodes, nil
		}
		if until != "" && parser.startsWith("</") {
			name, err := parser.closeTag()
			if err != nil {
				return nil, err
			}
			if name != until {
				return nil, parser.errorf("expected closing tag </%s>, got </%s>", until, name)
			}
			return nodes, nil
		}
		if parser.startsWith("</") {
			return nil, parser.errorf("unexpected closing tag")
		}
		if parser.peek() == '<' {
			node, err := parser.element()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}
		start := parser.index
		if text := parser.text(); strings.TrimSpace(text) != "" {
			if offset, message, ok := unsupportedTemplateSyntax(text); ok {
				parser.index = start + offset
				return nil, parser.errorf("%s", message)
			}
			nodes = append(nodes, Text{Value: text})
		}
	}
}

func unsupportedTemplateSyntax(text string) (int, string, bool) {
	for _, marker := range []string{"{#", "{:", "{/", "{@"} {
		offset := strings.Index(text, marker)
		if offset < 0 {
			continue
		}
		fragment := strings.TrimSpace(text[offset:])
		if len(fragment) > 32 {
			fragment = fragment[:32]
		}
		switch {
		case strings.HasPrefix(fragment, "{#if"), strings.HasPrefix(fragment, "{:else"), strings.HasPrefix(fragment, "{/if"):
			return offset, "unsupported template conditional syntax; use g:if, g:else-if, and g:else on elements", true
		case strings.HasPrefix(fragment, "{#each"), strings.HasPrefix(fragment, "{/each"):
			return offset, "unsupported template loop syntax; use g:for with g:key on elements inside stateful components", true
		case strings.HasPrefix(fragment, "{#await"), strings.HasPrefix(fragment, "{/await"):
			return offset, "unsupported template await syntax; use build/load data, actions, APIs, or fragments for asynchronous data", true
		case strings.HasPrefix(fragment, "{#snippet"), strings.HasPrefix(fragment, "{/snippet"):
			return offset, "unsupported template snippet syntax; use GOWDK component slots for supported reusable markup", true
		case strings.HasPrefix(fragment, "{@html"):
			return offset, "unsupported raw HTML syntax; GOWDK escapes rendered text by default — use the explicit g:html={Expr} directive on an element to opt into trusted raw HTML", true
		case strings.HasPrefix(fragment, "{@const"), strings.HasPrefix(fragment, "{@debug"):
			return offset, "unsupported template tag syntax; declare data in build/load blocks or component client code", true
		default:
			return offset, fmt.Sprintf("unsupported template syntax near %q", fragment), true
		}
	}
	return 0, "", false
}

func (parser *parser) element() (Node, error) {
	start := parser.index
	if !parser.consume("<") {
		return nil, parser.errorf("expected element")
	}
	name, err := parser.name()
	if err != nil {
		return nil, err
	}
	if isComponentName(name) {
		return parser.componentCall(name)
	}
	if !isLowerHTMLName(name) {
		return nil, parser.errorf("unsupported element <%s>; this build slice supports lowercase HTML tags only", name)
	}

	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			if err := validateRawHTMLDirective(name, attrs, nil); err != nil {
				return nil, parser.errorf("%s", err)
			}
			return Element{Name: name, Attrs: attrs, Start: start, End: parser.index}, nil
		case parser.consume(">"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			children, err := parser.nodes(name)
			if err != nil {
				return nil, err
			}
			if err := validateRawHTMLDirective(name, attrs, children); err != nil {
				return nil, parser.errorf("%s", err)
			}
			return Element{Name: name, Attrs: attrs, Children: children, Start: start, End: parser.index}, nil
		case parser.done():
			return nil, parser.errorf("unterminated <%s> tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) componentCall(name string) (ComponentCall, error) {
	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			return ComponentCall{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
			children, err := parser.nodes(name)
			if err != nil {
				return ComponentCall{}, err
			}
			return ComponentCall{Name: name, Attrs: attrs, Children: children}, nil
		case parser.done():
			return ComponentCall{}, parser.errorf("unterminated <%s> component tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return ComponentCall{}, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) attr() (Attr, error) {
	if attr, ok, err := parser.shorthandAttr(); ok || err != nil {
		return attr, err
	}
	start := parser.index
	name, err := parser.attrName()
	if err != nil {
		return Attr{}, err
	}
	if !isAttrName(name) {
		return Attr{}, parser.errorf("unsupported attribute name %q", name)
	}
	if strings.HasPrefix(name, "g:") && !isSupportedDirectiveName(name) {
		return Attr{}, parser.errorf("%s", unsupportedDirectiveMessage(name))
	}

	parser.skipSpace()
	if !parser.consume("=") {
		return Attr{Name: name, Boolean: true, Start: start, End: parser.index}, nil
	}
	parser.skipSpace()
	if strings.HasPrefix(name, "g:") {
		attr, err := parser.directiveAttr(name)
		attr.Start = start
		attr.End = parser.index
		return attr, err
	}
	if parser.startsWith("{") {
		value, err := parser.expressionAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value, Expression: true, Start: start, End: parser.index}, nil
	}
	value, err := parser.quotedAttrValue(name)
	if err != nil {
		return Attr{}, err
	}
	return Attr{Name: name, Value: value, Start: start, End: parser.index}, nil
}

func (parser *parser) attrName() (string, error) {
	if parser.done() || !isNameStart(parser.peek()) {
		return "", parser.errorf("expected attribute name")
	}
	start := parser.index
	parser.advance()
	for !parser.done() && isAttrNamePart(parser.peek()) {
		parser.advance()
	}
	return string(parser.source[start:parser.index]), nil
}

func (parser *parser) expressionAttrValue(name string) (string, error) {
	expr, err := parser.bracedAttrExpression(name)
	if err != nil {
		return "", err
	}
	if expr == "" {
		return "", parser.errorf("empty expression attribute %q", name)
	}
	return "{" + expr + "}", nil
}

func (parser *parser) shorthandAttr() (Attr, bool, error) {
	if parser.done() {
		return Attr{}, false, nil
	}
	prefix := parser.peek()
	if prefix != '.' && prefix != '#' {
		return Attr{}, false, nil
	}
	start := parser.index
	parser.advance()
	valueStart := parser.index
	for !parser.done() && isShorthandPart(parser.peek()) {
		parser.advance()
	}
	if valueStart == parser.index {
		return Attr{}, true, parser.errorf("empty shorthand attribute")
	}
	value := string(parser.source[valueStart:parser.index])
	switch prefix {
	case '.':
		return Attr{Name: "class", Value: value, Start: start, End: parser.index}, true, nil
	case '#':
		return Attr{Name: "id", Value: value, Start: start, End: parser.index}, true, nil
	default:
		return Attr{}, false, nil
	}
}

func normalizeHTMLAttrs(attrs []Attr) ([]Attr, error) {
	var classValues []string
	var out []Attr
	id := ""
	for _, attr := range attrs {
		switch attr.Name {
		case "class":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			for _, className := range strings.Fields(attr.Value) {
				classValues = append(classValues, className)
			}
		case "id":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			if id != "" {
				return nil, fmt.Errorf("element declares multiple id attributes")
			}
			id = attr.Value
			out = append(out, attr)
		default:
			out = append(out, attr)
		}
	}
	if len(classValues) > 0 {
		out = append([]Attr{{Name: "class", Value: strings.Join(classValues, " ")}}, out...)
	}
	return out, nil
}

func (parser *parser) directiveAttr(name string) (Attr, error) {
	if parser.startsWith("{") {
		value, err := parser.bracedAttrExpression(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value, Expression: name == "g:html"}, nil
	}
	if parser.startsWith(`"`) {
		if name == "g:html" {
			return Attr{}, parser.errorf("g:html must use an expression value such as g:html={Body}, not a string literal")
		}
		value, err := parser.quotedAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		value = strings.TrimSpace(value)
		return Attr{Name: name, Value: value}, nil
	}
	return Attr{}, parser.errorf("directive attribute %q must use {name}", name)
}

func (parser *parser) bracedAttrExpression(name string) (string, error) {
	if !parser.consume("{") {
		return "", parser.errorf("attribute %q must use an expression value", name)
	}
	start := parser.index
	depth := 0
	inString := false
	escaped := false
	for !parser.done() {
		char := parser.peek()
		if escaped {
			escaped = false
			parser.advance()
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			parser.advance()
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			if depth == 0 {
				expr := strings.TrimSpace(string(parser.source[start:parser.index]))
				parser.advance()
				return expr, nil
			}
			depth--
		}
		parser.advance()
	}
	return "", parser.errorf("unterminated expression attribute %q", name)
}

func (parser *parser) quotedAttrValue(name string) (string, error) {
	if !parser.consume(`"`) {
		return "", parser.errorf("attribute %q must use a quoted string value", name)
	}
	start := parser.index - 1
	escaped := false
	for !parser.done() {
		switch parser.peek() {
		case '\\':
			escaped = !escaped
			parser.advance()
		case '"':
			if escaped {
				escaped = false
				parser.advance()
				continue
			}
			parser.advance()
			value, err := strconv.Unquote(string(parser.source[start:parser.index]))
			if err != nil {
				return "", parser.errorf("invalid attribute %q string: %v", name, err)
			}
			return value, nil
		default:
			escaped = false
			parser.advance()
		}
	}
	return "", parser.errorf("unterminated attribute %q", name)
}

func (parser *parser) closeTag() (string, error) {
	if !parser.consume("</") {
		return "", parser.errorf("expected closing tag")
	}
	name, err := parser.name()
	if err != nil {
		return "", err
	}
	parser.skipSpace()
	if !parser.consume(">") {
		return "", parser.errorf("expected > after closing tag")
	}
	return name, nil
}

func (parser *parser) name() (string, error) {
	if parser.done() || !isNameStart(parser.peek()) {
		return "", parser.errorf("expected name")
	}
	start := parser.index
	parser.advance()
	for !parser.done() && isNamePart(parser.peek()) {
		parser.advance()
	}
	return string(parser.source[start:parser.index]), nil
}

func (parser *parser) text() string {
	start := parser.index
	for !parser.done() && parser.peek() != '<' {
		parser.advance()
	}
	return string(parser.source[start:parser.index])
}

func (parser *parser) skipSpace() {
	for !parser.done() && unicode.IsSpace(parser.peek()) {
		parser.advance()
	}
}

func (parser *parser) consume(value string) bool {
	if !parser.startsWith(value) {
		return false
	}
	parser.index += len([]rune(value))
	return true
}

func (parser *parser) startsWith(value string) bool {
	runes := []rune(value)
	if parser.index+len(runes) > len(parser.source) {
		return false
	}
	for offset, r := range runes {
		if parser.source[parser.index+offset] != r {
			return false
		}
	}
	return true
}

func (parser *parser) done() bool {
	return parser.index >= len(parser.source)
}

func (parser *parser) peek() rune {
	return parser.source[parser.index]
}

func (parser *parser) advance() {
	parser.index++
}

func (parser *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("view parse error at offset %d: %s", parser.index, fmt.Sprintf(format, args...))
}

func isLowerHTMLName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		case i > 0 && r == '-':
		default:
			return false
		}
	}
	return true
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, ".") {
		alias, name, ok := strings.Cut(value, ".")
		if !ok || strings.Contains(name, ".") {
			return false
		}
		return isComponentAlias(alias) && isExportedComponentName(name)
	}
	return isExportedComponentName(value)
}

func isExportedComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}

func isComponentAlias(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		switch {
		case index == 0 && isNameStart(r):
		case index > 0 && (isNameStart(r) || unicode.IsDigit(r)):
		default:
			return false
		}
	}
	return true
}

func isAttrName(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "g:on:") {
		_, err := ParseEventDirective(value)
		return err == nil
	}
	if strings.HasPrefix(value, "style:") {
		_, err := parseStyleBindingAttr(value)
		return err == nil
	}
	for i, r := range value {
		switch {
		case isNameStart(r):
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == ':' || r == '_'):
		default:
			return false
		}
	}
	return true
}

func isNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isNamePart(r rune) bool {
	return isNameStart(r) || unicode.IsDigit(r) || r == '-' || r == ':' || r == '.'
}

func isAttrNamePart(r rune) bool {
	return isNamePart(r) || r == '.' || r == '%' || r == '(' || r == ')'
}

func isShorthandPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':'
}
