package parser

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	metadataPattern         = linePattern{parse: parseMetadataLine}
	packagePattern          = linePattern{parse: parsePackageLine}
	importPattern           = linePattern{parse: parseImportLine}
	usePattern              = linePattern{parse: parseUseLine}
	jsPattern               = linePattern{parse: parseJSLine}
	jsBlockPattern          = linePattern{parse: parseJSBlockLine}
	buildCallPattern        = linePattern{parse: parseBuildCallLine}
	actionEndpointPattern   = linePattern{parse: parseActionEndpointLine}
	apiEndpointPattern      = linePattern{parse: parseAPIEndpointLine}
	fragmentEndpointPattern = linePattern{parse: parseFragmentEndpointLine}
	actionPattern           = linePattern{parse: parseActionBlockLine}
	apiPattern              = linePattern{parse: parseAPIBlockLine}
	propPattern             = linePattern{parse: parsePropLine}
	emitPattern             = linePattern{parse: parseEmitLine}
	identifierPattern       = linePattern{parse: parseIdentifierLine}
	componentTypePattern    = linePattern{parse: parseComponentTypeLine}
	storePattern            = linePattern{parse: parseStoreLine}
	actionInputPattern      = linePattern{parse: parseActionInputLine}
	actionValidPattern      = linePattern{parse: parseActionValidLine}
	actionRedirectPattern   = linePattern{parse: parseActionRedirectLine}
	actionFragmentPattern   = linePattern{parse: parseActionFragmentLine}
	apiRoutePattern         = linePattern{parse: parseAPIRouteLine}
	literalRecordPattern    = linePattern{parse: parseLiteralRecordLine}
	syntaxBlockPattern      = linePattern{parse: parseSyntaxBlockLine}
	goBlockPattern          = linePattern{parse: parseGoBlockLine}
	routeParamPattern       = routeParamPatternScanner{}
)

type linePattern struct {
	parse func(string) []string
}

func (pattern linePattern) FindStringSubmatch(line string) []string {
	return pattern.parse(line)
}

func (pattern linePattern) MatchString(line string) bool {
	return pattern.parse(line) != nil
}

type lineParser struct {
	tokens []lineToken
	pos    int
}

type lineTokenKind int

const (
	lineTokenEOF lineTokenKind = iota
	lineTokenIdent
	lineTokenString
	lineTokenRest
	lineTokenAt
	lineTokenArrow
	lineTokenFatArrow
	lineTokenAssign
	lineTokenDeclare
	lineTokenLBrace
	lineTokenRBrace
	lineTokenLParen
	lineTokenRParen
	lineTokenDot
	lineTokenQuestion
	lineTokenColon
)

type lineToken struct {
	kind lineTokenKind
	text string
}

func newLineParser(line string) lineParser {
	return lineParser{tokens: lexLine(line)}
}

func lexLine(line string) []lineToken {
	var tokens []lineToken
	for index := 0; index < len(line); {
		r, size := runeAt(line, index)
		if unicode.IsSpace(r) {
			index += size
			continue
		}
		if isLineIdentStart(r) {
			start := index
			index += size
			for index < len(line) {
				next, nextSize := runeAt(line, index)
				if !isLineIdentPart(next) {
					break
				}
				index += nextSize
			}
			tokens = append(tokens, lineToken{kind: lineTokenIdent, text: line[start:index]})
			continue
		}
		if r == '"' {
			value, next, ok := lexLineString(line, index)
			if !ok {
				return append(tokens, lineToken{kind: lineTokenRest, text: line[index:]}, lineToken{kind: lineTokenEOF})
			}
			tokens = append(tokens, lineToken{kind: lineTokenString, text: value})
			index = next
			continue
		}
		if strings.HasPrefix(line[index:], "=>") {
			tokens = append(tokens, lineToken{kind: lineTokenFatArrow, text: "=>"})
			index += 2
			continue
		}
		if strings.HasPrefix(line[index:], "->") {
			tokens = append(tokens, lineToken{kind: lineTokenArrow, text: "->"})
			index += 2
			continue
		}
		if strings.HasPrefix(line[index:], ":=") {
			tokens = append(tokens, lineToken{kind: lineTokenDeclare, text: ":="})
			index += 2
			continue
		}
		switch r {
		case '@':
			tokens = append(tokens, lineToken{kind: lineTokenAt, text: "@"})
		case '=':
			tokens = append(tokens, lineToken{kind: lineTokenAssign, text: "="})
		case '{':
			tokens = append(tokens, lineToken{kind: lineTokenLBrace, text: "{"})
		case '}':
			tokens = append(tokens, lineToken{kind: lineTokenRBrace, text: "}"})
		case '(':
			tokens = append(tokens, lineToken{kind: lineTokenLParen, text: "("})
		case ')':
			tokens = append(tokens, lineToken{kind: lineTokenRParen, text: ")"})
		case '.':
			tokens = append(tokens, lineToken{kind: lineTokenDot, text: "."})
		case '?':
			tokens = append(tokens, lineToken{kind: lineTokenQuestion, text: "?"})
		case ':':
			tokens = append(tokens, lineToken{kind: lineTokenColon, text: ":"})
		default:
			tokens = append(tokens, lineToken{kind: lineTokenRest, text: line[index:]})
			index = len(line)
			continue
		}
		index += size
	}
	tokens = append(tokens, lineToken{kind: lineTokenEOF})
	return tokens
}

func runeAt(value string, index int) (rune, int) {
	r, size := utf8.DecodeRuneInString(value[index:])
	return r, size
}

func lexLineString(line string, start int) (string, int, bool) {
	var builder strings.Builder
	for index := start + 1; index < len(line); index++ {
		switch line[index] {
		case '"':
			return builder.String(), index + 1, true
		case '\\':
			if index+1 >= len(line) {
				builder.WriteByte(line[index])
				continue
			}
			index++
			switch line[index] {
			case '"', '\\':
				builder.WriteByte(line[index])
			case 'n':
				builder.WriteByte('\n')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteByte('\\')
				builder.WriteByte(line[index])
			}
		default:
			builder.WriteByte(line[index])
		}
	}
	return "", start, false
}

func isLineIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isLineIdentPart(r rune) bool {
	return isLineIdentStart(r) || unicode.IsDigit(r)
}

func isStrictIdent(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !isIdentStart(r) {
				return false
			}
			continue
		}
		if !isIdentStart(r) && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func (parser *lineParser) match(kind lineTokenKind) (lineToken, bool) {
	token := parser.peek()
	if token.kind != kind {
		return lineToken{}, false
	}
	parser.pos++
	return token, true
}

func (parser *lineParser) matchIdent(text string) bool {
	token := parser.peek()
	if token.kind != lineTokenIdent || token.text != text {
		return false
	}
	parser.pos++
	return true
}

func (parser *lineParser) ident() (string, bool) {
	token, ok := parser.match(lineTokenIdent)
	if !ok || !isStrictIdent(token.text) {
		return "", false
	}
	return token.text, true
}

func (parser *lineParser) blockName() (string, bool) {
	token, ok := parser.match(lineTokenIdent)
	if !ok || !isBlockName(token.text) {
		return "", false
	}
	return token.text, true
}

func (parser *lineParser) stringValue() (string, bool) {
	token, ok := parser.match(lineTokenString)
	return token.text, ok
}

func (parser *lineParser) eof() bool {
	return parser.peek().kind == lineTokenEOF
}

func (parser *lineParser) peek() lineToken {
	if parser.pos >= len(parser.tokens) {
		return lineToken{kind: lineTokenEOF}
	}
	return parser.tokens[parser.pos]
}

func parseMetadataLine(line string) []string {
	line = strings.TrimSpace(line)
	nameEnd := 0
	for nameEnd < len(line) {
		r, size := runeAt(line, nameEnd)
		if nameEnd == 0 {
			if !isIdentStart(r) {
				return nil
			}
		} else if !isIdentStart(r) && (r < '0' || r > '9') {
			break
		}
		nameEnd += size
	}
	if nameEnd == 0 {
		return nil
	}
	name := line[:nameEnd]
	if !isMetadataKeyword(name) {
		return nil
	}
	if nameEnd < len(line) {
		r, _ := runeAt(line, nameEnd)
		if !unicode.IsSpace(r) {
			return nil
		}
	}
	return []string{line, name, strings.TrimSpace(line[nameEnd:])}
}

func isMetadataKeyword(name string) bool {
	switch name {
	case "page", "route", "title", "description", "canonical", "image", "layout", "cache", "revalidate", "error", "guard", "css", "component", "wasm", "asset", "plugin":
		return true
	default:
		return false
	}
}

func parsePackageLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("package") {
		return nil
	}
	name, ok := parser.ident()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, name}
}

func parseImportLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("import") {
		return nil
	}
	alias := ""
	if parser.peek().kind == lineTokenIdent {
		value, ok := parser.ident()
		if !ok {
			return nil
		}
		alias = value
	}
	path, ok := parser.stringValue()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, alias, path}
}

func parseUseLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("use") {
		return nil
	}
	alias, ok := parser.ident()
	if !ok {
		return nil
	}
	pkg, ok := parser.identString()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, alias, pkg}
}

func (parser *lineParser) identString() (string, bool) {
	value, ok := parser.stringValue()
	if !ok || !isStrictIdent(value) {
		return "", false
	}
	return value, true
}

func parseJSLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("js") {
		return nil
	}
	path, ok := parser.stringValue()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, path}
}

func parseJSBlockLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("js") {
		return nil
	}
	if _, ok := parser.match(lineTokenLBrace); !ok || !parser.eof() {
		return nil
	}
	return []string{line}
}

func parseBuildCallLine(line string) []string {
	parser := newLineParser(line)
	if _, ok := parser.match(lineTokenFatArrow); !ok {
		return nil
	}
	alias, ok := parser.ident()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenDot); !ok {
		return nil
	}
	function, ok := parser.ident()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenLParen); !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenRParen); !ok || !parser.eof() {
		return nil
	}
	return []string{line, alias, function}
}

func parseActionEndpointLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("act") {
		return nil
	}
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	method, ok := parser.method()
	if !ok {
		return nil
	}
	route, ok := parser.stringValue()
	if !ok {
		return nil
	}
	errorPath, ok := parser.optionalErrorString()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, method, route, errorPath}
}

func parseAPIEndpointLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("api") {
		return nil
	}
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	method, ok := parser.apiMethod()
	if !ok {
		return nil
	}
	route, ok := parser.stringValue()
	if !ok {
		return nil
	}
	errorPath, ok := parser.optionalErrorString()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, method, route, errorPath}
}

func parseFragmentEndpointLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("fragment") {
		return nil
	}
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	method, ok := parser.apiMethod()
	if !ok {
		return nil
	}
	route, ok := parser.stringValue()
	if !ok {
		return nil
	}
	target, ok := parser.stringValue()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenLBrace); !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, method, route, target}
}

func parseActionBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "act")
	if !ok {
		return nil
	}
	if !isBlockName(body) {
		return nil
	}
	return []string{strings.TrimSpace(line), body}
}

func parseAPIBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "api")
	if !ok {
		return nil
	}
	if body != "" && !isBlockName(body) {
		return nil
	}
	return []string{strings.TrimSpace(line), body}
}

func parsePropLine(line string) []string {
	parser := newLineParser(line)
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	typ, ok := parser.ident()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, typ}
}

func parseEmitLine(line string) []string {
	line = strings.TrimSpace(line)
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open <= 0 || close != len(line)-1 {
		return nil
	}
	name := strings.TrimSpace(line[:open])
	if !isStrictIdent(name) {
		return nil
	}
	return []string{line, name, line[open+1 : close]}
}

func parseIdentifierLine(line string) []string {
	line = strings.TrimSpace(line)
	if !isStrictIdent(line) {
		return nil
	}
	return []string{line}
}

func parseComponentTypeLine(line string) []string {
	parser := newLineParser(line)
	kind, ok := parser.ident()
	if !ok || (kind != "props" && kind != "state") {
		return nil
	}
	typeAlias, typeName, ok := parser.qualifiedIdent()
	if !ok {
		return nil
	}
	initAlias, initName := "", ""
	if _, ok := parser.match(lineTokenAssign); ok {
		initAlias, initName, ok = parser.qualifiedIdent()
		if !ok {
			return nil
		}
		if _, ok := parser.match(lineTokenLParen); !ok {
			return nil
		}
		if _, ok := parser.match(lineTokenRParen); !ok {
			return nil
		}
	}
	if !parser.eof() {
		return nil
	}
	return []string{line, kind, typeAlias, typeName, initAlias, initName}
}

func parseStoreLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("store") {
		return nil
	}
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	typeAlias, typeName, ok := parser.qualifiedIdent()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenAssign); !ok {
		return nil
	}
	initAlias, initName, ok := parser.qualifiedIdent()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenLParen); !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenRParen); !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, typeAlias, typeName, initAlias, initName}
}

func parseActionInputLine(line string) []string {
	parser := newLineParser(line)
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenDeclare); !ok {
		return nil
	}
	if !parser.matchIdent("form") {
		return nil
	}
	typ, ok := parser.ident()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, name, typ}
}

func parseActionValidLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("valid") {
		return nil
	}
	if _, ok := parser.match(lineTokenLParen); !ok {
		return nil
	}
	name, ok := parser.ident()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenRParen); !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenQuestion); !ok || !parser.eof() {
		return nil
	}
	return []string{line, name}
}

func parseActionRedirectLine(line string) []string {
	parser := newLineParser(line)
	if _, ok := parser.match(lineTokenArrow); !ok {
		return nil
	}
	target, ok := parser.stringValue()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, target}
}

func parseActionFragmentLine(line string) []string {
	parser := newLineParser(line)
	if !parser.matchIdent("fragment") {
		return nil
	}
	target, ok := parser.stringValue()
	if !ok {
		return nil
	}
	if _, ok := parser.match(lineTokenLBrace); !ok || !parser.eof() {
		return nil
	}
	return []string{line, target}
}

func parseAPIRouteLine(line string) []string {
	parser := newLineParser(line)
	method, ok := parser.apiMethod()
	if !ok {
		return nil
	}
	route, ok := parser.stringValue()
	if !ok || !parser.eof() {
		return nil
	}
	return []string{line, method, route}
}

func parseLiteralRecordLine(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "=>") {
		return nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "=>"))
	if !strings.HasPrefix(rest, "{") || !strings.HasSuffix(rest, "}") {
		return nil
	}
	return []string{line, strings.TrimSpace(rest[1 : len(rest)-1])}
}

func parseSyntaxBlockLine(line string) []string {
	parser := newLineParser(line)
	name, ok := parser.ident()
	if !ok || !isSyntaxBlockName(name) {
		return nil
	}
	if _, ok := parser.match(lineTokenLBrace); !ok || !parser.eof() {
		return nil
	}
	return []string{line, name}
}

func parseGoBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "go")
	if !ok {
		return nil
	}
	if body != "" && !isBlockName(body) {
		return nil
	}
	return []string{strings.TrimSpace(line), body}
}

func keywordBlockBody(line string, keyword string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == keyword+"{" {
		return "", true
	}
	if !strings.HasPrefix(line, keyword) || !strings.HasSuffix(line, "{") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, keyword)), "{"))
	if rest == "" {
		return "", true
	}
	if !unicode.IsSpace(rune(line[len(keyword)])) {
		return "", false
	}
	return rest, true
}

func (parser *lineParser) qualifiedIdent() (string, string, bool) {
	alias, ok := parser.ident()
	if !ok {
		return "", "", false
	}
	if _, ok := parser.match(lineTokenDot); !ok {
		return "", "", false
	}
	name, ok := parser.ident()
	if !ok {
		return "", "", false
	}
	return alias, name, true
}

func (parser *lineParser) optionalErrorString() (string, bool) {
	if parser.peek().kind == lineTokenEOF {
		return "", true
	}
	if !parser.matchIdent("error") {
		return "", false
	}
	return parser.stringValue()
}

func (parser *lineParser) method() (string, bool) {
	token, ok := parser.match(lineTokenIdent)
	if !ok || token.text == "" {
		return "", false
	}
	for _, r := range token.text {
		if r < 'A' || r > 'Z' {
			return "", false
		}
	}
	return token.text, true
}

func (parser *lineParser) apiMethod() (string, bool) {
	method, ok := parser.method()
	if !ok {
		return "", false
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return method, true
	default:
		return "", false
	}
}

func isSyntaxBlockName(name string) bool {
	switch name {
	case "paths", "build", "load", "client", "view", "style", "props", "exports", "emits":
		return true
	default:
		return false
	}
}

type routeParamPatternScanner struct{}

func (routeParamPatternScanner) FindAllStringSubmatchIndex(route string, _ int) [][]int {
	var matches [][]int
	for index := 0; index < len(route); index++ {
		if route[index] != '{' {
			continue
		}
		end := strings.IndexByte(route[index:], '}')
		if end < 0 {
			continue
		}
		end += index
		body := route[index+1 : end]
		colon := strings.IndexByte(body, ':')
		name := body
		paramType := ""
		if colon >= 0 {
			name = body[:colon]
			paramType = body[colon+1:]
		}
		if !isStrictIdent(name) || (paramType != "" && !isStrictIdent(paramType)) {
			continue
		}
		match := []int{index, end + 1, index + 1, index + 1 + len(name), -1, -1}
		if colon >= 0 {
			typeStart := index + 1 + colon + 1
			match[4] = typeStart
			match[5] = typeStart + len(paramType)
		}
		matches = append(matches, match)
		index = end
	}
	return matches
}
