package viewparse

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

// Parse parses a view markup fragment into the pure view model.
func Parse(source string) ([]Node, error) {
	parser := parser{source: []rune(source)}
	nodes, err := parser.nodes("")
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if !parser.done() {
		return nil, parser.errorf("unexpected content")
	}
	return nodes, nil
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
		if parser.startsWith("{#await") {
			node, err := parser.awaitBlock()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}
		if parser.startsWith("{:then") || parser.startsWith("{:catch") || parser.startsWith("{/await") {
			return nil, parser.errorf("unexpected await branch marker")
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
			nodes = append(nodes, Text{Value: text, Start: start, End: parser.index})
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
		case strings.HasPrefix(fragment, "{#snippet"), strings.HasPrefix(fragment, "{/snippet"):
			return offset, "unsupported template snippet syntax; use GOWDK component slots for supported reusable markup", true
		case strings.HasPrefix(fragment, "{@html"):
			return offset, "unsupported raw HTML syntax; GOWDK escapes rendered text by default — use the explicit g:unsafe-html={Expr} directive on an element to opt into trusted raw HTML", true
		case strings.HasPrefix(fragment, "{@const"), strings.HasPrefix(fragment, "{@debug"):
			return offset, "unsupported template tag syntax; declare data in build/load blocks or component client code", true
		default:
			return offset, fmt.Sprintf("unsupported template syntax near %q", fragment), true
		}
	}
	return 0, "", false
}

type awaitBranchMarker struct {
	Kind string
	Name string
}

func (parser *parser) awaitBlock() (AwaitBlock, error) {
	start := parser.index
	expr, err := parser.templateTagBody("{#await", "await block")
	if err != nil {
		return AwaitBlock{}, err
	}
	if strings.TrimSpace(expr) == "" {
		return AwaitBlock{}, parser.errorf("await block requires an expression")
	}
	pending, marker, err := parser.awaitNodes()
	if err != nil {
		return AwaitBlock{}, err
	}
	if marker.Kind != "then" {
		return AwaitBlock{}, parser.errorf("await block requires a {:then name} branch before {/await}")
	}
	if marker.Name == "" {
		return AwaitBlock{}, parser.errorf("await then branch requires a result binding name")
	}
	resultName := marker.Name
	thenNodes, marker, err := parser.awaitNodes()
	if err != nil {
		return AwaitBlock{}, err
	}
	var catchNodes []Node
	var errorName string
	if marker.Kind == "catch" {
		if marker.Name == "" {
			return AwaitBlock{}, parser.errorf("await catch branch requires an error binding name")
		}
		errorName = marker.Name
		if errorName == resultName {
			return AwaitBlock{}, parser.errorf("await catch binding %q must differ from then binding", errorName)
		}
		catchNodes, marker, err = parser.awaitNodes()
		if err != nil {
			return AwaitBlock{}, err
		}
	}
	if marker.Kind != "end" {
		return AwaitBlock{}, parser.errorf("await block has duplicate %s branch", marker.Kind)
	}
	return AwaitBlock{
		Expression: strings.TrimSpace(expr),
		ResultName: resultName,
		ErrorName:  errorName,
		Pending:    pending,
		Then:       thenNodes,
		Catch:      catchNodes,
		Start:      start,
		End:        parser.index,
	}, nil
}

func (parser *parser) awaitNodes() ([]Node, awaitBranchMarker, error) {
	var nodes []Node
	for {
		if parser.done() {
			return nil, awaitBranchMarker{}, parser.errorf("missing closing {/await}")
		}
		if parser.startsWith("{:then") || parser.startsWith("{:catch") || parser.startsWith("{/await") {
			marker, err := parser.awaitBranchMarker()
			return nodes, marker, err
		}
		if parser.startsWith("{#await") {
			node, err := parser.awaitBlock()
			if err != nil {
				return nil, awaitBranchMarker{}, err
			}
			nodes = append(nodes, node)
			continue
		}
		if parser.startsWith("</") {
			return nil, awaitBranchMarker{}, parser.errorf("unexpected closing tag")
		}
		if parser.peek() == '<' {
			node, err := parser.element()
			if err != nil {
				return nil, awaitBranchMarker{}, err
			}
			nodes = append(nodes, node)
			continue
		}
		start := parser.index
		if text := parser.text(); strings.TrimSpace(text) != "" {
			if offset, message, ok := unsupportedTemplateSyntax(text); ok {
				parser.index = start + offset
				return nil, awaitBranchMarker{}, parser.errorf("%s", message)
			}
			nodes = append(nodes, Text{Value: text, Start: start, End: parser.index})
		}
	}
}

func (parser *parser) awaitBranchMarker() (awaitBranchMarker, error) {
	switch {
	case parser.startsWith("{:then"):
		name, err := parser.templateTagBody("{:then", "await then branch")
		if err != nil {
			return awaitBranchMarker{}, err
		}
		name, err = awaitBindingName("then", name)
		if err != nil {
			return awaitBranchMarker{}, parser.errorf("%s", err)
		}
		return awaitBranchMarker{Kind: "then", Name: name}, nil
	case parser.startsWith("{:catch"):
		name, err := parser.templateTagBody("{:catch", "await catch branch")
		if err != nil {
			return awaitBranchMarker{}, err
		}
		name, err = awaitBindingName("catch", name)
		if err != nil {
			return awaitBranchMarker{}, parser.errorf("%s", err)
		}
		return awaitBranchMarker{Kind: "catch", Name: name}, nil
	case parser.startsWith("{/await"):
		body, err := parser.templateTagBody("{/await", "await end")
		if err != nil {
			return awaitBranchMarker{}, err
		}
		if strings.TrimSpace(body) != "" {
			return awaitBranchMarker{}, parser.errorf("await end tag must not contain content")
		}
		return awaitBranchMarker{Kind: "end"}, nil
	default:
		return awaitBranchMarker{}, parser.errorf("expected await branch marker")
	}
}

func (parser *parser) templateTagBody(prefix, name string) (string, error) {
	if !parser.consume(prefix) {
		return "", parser.errorf("expected %s tag", name)
	}
	if !parser.done() && parser.peek() != '}' && !unicode.IsSpace(parser.peek()) {
		return "", parser.errorf("expected whitespace or } after %s", prefix)
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
				body := strings.TrimSpace(string(parser.source[start:parser.index]))
				parser.advance()
				return body, nil
			}
			depth--
		}
		parser.advance()
	}
	return "", parser.errorf("unterminated %s tag", name)
}

func awaitBindingName(kind, source string) (string, error) {
	name := strings.TrimSpace(source)
	if name == "" {
		return "", fmt.Errorf("await %s branch requires a binding name", kind)
	}
	if strings.ContainsAny(name, " \t\r\n") || !isLocalIdentifier(name) {
		return "", fmt.Errorf("await %s binding %q must be a local identifier", kind, name)
	}
	return name, nil
}

func isLocalIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		switch {
		case index == 0 && (r == '_' || unicode.IsLetter(r)):
		case index > 0 && (r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)):
		default:
			return false
		}
	}
	return true
}

func (parser *parser) element() (Node, error) {
	start := parser.index
	if !parser.consume("<") {
		return nil, parser.errorf("expected element")
	}
	if parser.startsWith("{") {
		return nil, parser.errorf("dynamic component selection is not supported; component calls must name a known component directly")
	}
	name, err := parser.name()
	if err != nil {
		return nil, err
	}
	if isComponentName(name) {
		return parser.componentCall(name, start)
	}
	if !isLowerHTMLName(name) {
		return nil, parser.errorf("unsupported element <%s>; this build slice supports lowercase HTML tags only", name)
	}
	if blockedViewElement(name) {
		return nil, parser.errorf("element <%s> is not supported in view {}; use configured or scoped script assets instead", name)
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
			if err := validateParsedHTMLAttrsSafety(attrs); err != nil {
				return nil, parser.errorf("%s", err)
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
			if err := validateParsedHTMLAttrsSafety(attrs); err != nil {
				return nil, parser.errorf("%s", err)
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

func (parser *parser) componentCall(name string, start int) (ComponentCall, error) {
	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			return ComponentCall{Name: name, Attrs: attrs, Start: start, End: parser.index}, nil
		case parser.consume(">"):
			children, err := parser.nodes(name)
			if err != nil {
				return ComponentCall{}, err
			}
			return ComponentCall{Name: name, Attrs: attrs, Children: children, Start: start, End: parser.index}, nil
		case parser.done():
			return ComponentCall{}, parser.errorf("unterminated <%s> component tag", name)
		default:
			if parser.startsWith("{...") {
				attr, err := parser.componentSpreadAttr()
				if err != nil {
					return ComponentCall{}, err
				}
				attrs = append(attrs, attr)
				continue
			}
			if parser.startsWith("...") {
				return ComponentCall{}, parser.errorf("component spread props must use {...props}")
			}
			attr, err := parser.componentAttr()
			if err != nil {
				return ComponentCall{}, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) componentSpreadAttr() (Attr, error) {
	start := parser.index
	if !parser.consume("{...") {
		return Attr{}, parser.errorf("component spread props must use {...props}")
	}
	parser.skipSpace()
	source, err := parser.name()
	if err != nil {
		return Attr{}, err
	}
	parser.skipSpace()
	if !parser.consume("}") {
		return Attr{}, parser.errorf("component spread props must use {...props}")
	}
	if source != "props" {
		return Attr{}, parser.errorf("component spread props only support {...props} in this build slice")
	}
	return Attr{Name: source, Spread: true, Start: start, End: parser.index}, nil
}

func (parser *parser) attr() (Attr, error) {
	return parser.attrWithOptions(false)
}

func (parser *parser) componentAttr() (Attr, error) {
	return parser.attrWithOptions(true)
}

func (parser *parser) attrWithOptions(allowComponentBind bool) (Attr, error) {
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
	if strings.HasPrefix(name, "g:") && !isSupportedDirectiveName(name) && (!allowComponentBind || !isComponentBindDirective(name)) {
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
			classValues = append(classValues, strings.Fields(attr.Value)...)
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

func validateParsedHTMLAttrsSafety(attrs []Attr) error {
	for _, attr := range attrs {
		if err := validateParsedHTMLAttrSafety(attr); err != nil {
			return err
		}
	}
	return nil
}

func (parser *parser) directiveAttr(name string) (Attr, error) {
	if parser.startsWith("{") {
		if name == "g:transition" || name == "g:animate" {
			return Attr{}, parser.errorf("%s must use a quoted literal motion name", name)
		}
		value, err := parser.bracedAttrExpression(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value, Expression: name == "g:unsafe-html"}, nil
	}
	if parser.startsWith(`"`) {
		if name == "g:unsafe-html" {
			return Attr{}, parser.errorf("g:unsafe-html must use an expression value such as g:unsafe-html={Body}, not a string literal")
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
	for !parser.done() && parser.peek() != '<' && !parser.startsAwaitMarker() {
		parser.advance()
	}
	return string(parser.source[start:parser.index])
}

func (parser *parser) startsAwaitMarker() bool {
	return parser.startsWith("{#await") || parser.startsWith("{:then") || parser.startsWith("{:catch") || parser.startsWith("{/await")
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
