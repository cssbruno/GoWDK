package parser

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/syntax"
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

func syntaxTokens(line string) []syntax.Token {
	if hasLineComment(line) {
		return nil
	}
	tokens, diagnostics := syntax.Lex(line)
	if len(diagnostics) > 0 {
		return nil
	}
	out := make([]syntax.Token, 0, len(tokens))
	for _, token := range tokens {
		switch token.Kind {
		case syntax.TokenEOF, syntax.TokenNewline:
			return out
		case syntax.TokenIllegal:
			return nil
		default:
			out = append(out, token)
		}
	}
	return out
}

func hasLineComment(line string) bool {
	inString := false
	escaped := false
	for index := 0; index+1 < len(line); index++ {
		ch := line[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '/' && line[index+1] == '/' {
			return true
		}
	}
	return false
}

func parseMetadataLine(line string) []string {
	line = strings.TrimSpace(line)
	nameEnd := 0
	for nameEnd < len(line) {
		ch := line[nameEnd]
		if nameEnd == 0 {
			if !isIdentStart(rune(ch)) {
				return nil
			}
		} else if !isIdentStart(rune(ch)) && (ch < '0' || ch > '9') {
			break
		}
		nameEnd++
	}
	if nameEnd == 0 {
		return nil
	}
	name := line[:nameEnd]
	if !isMetadataKeyword(name) {
		return nil
	}
	if nameEnd < len(line) {
		next := line[nameEnd]
		if next != ' ' && next != '\t' {
			return nil
		}
	}
	return []string{line, name, strings.TrimSpace(line[nameEnd:])}
}

func isMetadataKeyword(name string) bool {
	switch name {
	case "page", "route", "title", "description", "canonical", "image", "robots", "noindex", "preload", "prefetch", "layout", "cache", "revalidate", "error", "guard", "css", "component", "wasm", "asset":
		return true
	default:
		return false
	}
}

func parsePackageLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 2 || tokens[0].Lexeme != "package" || tokens[1].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[1].Lexeme) {
		return nil
	}
	return []string{line, tokens[1].Lexeme}
}

func parseImportLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 2 && len(tokens) != 3 {
		return nil
	}
	if tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "import" {
		return nil
	}
	index := 1
	alias := ""
	if len(tokens) == 3 {
		if tokens[index].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[index].Lexeme) {
			return nil
		}
		alias = tokens[index].Lexeme
		index++
	}
	if tokens[index].Kind != syntax.TokenString {
		return nil
	}
	return []string{line, alias, decodeStringLiteral(tokens[index].Lexeme)}
}

func parseUseLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 3 || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "use" {
		return nil
	}
	if tokens[1].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[1].Lexeme) || tokens[2].Kind != syntax.TokenString {
		return nil
	}
	pkg := decodeStringLiteral(tokens[2].Lexeme)
	if !isStrictIdent(pkg) {
		return nil
	}
	return []string{line, tokens[1].Lexeme, pkg}
}

func parseJSLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 2 || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "js" || tokens[1].Kind != syntax.TokenString {
		return nil
	}
	return []string{line, decodeStringLiteral(tokens[1].Lexeme)}
}

func parseJSBlockLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 2 || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "js" || tokens[1].Kind != syntax.TokenLBrace {
		return nil
	}
	return []string{line}
}

func parseBuildCallLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 3 || tokens[0].Kind != syntax.TokenArrow {
		return nil
	}
	alias, function, ok := parseQualifiedCall(tokens[1:])
	if !ok {
		return nil
	}
	return []string{line, alias, function}
}

func parseActionEndpointLine(line string) []string {
	return parseEndpointLine(line, "act", true)
}

func parseAPIEndpointLine(line string) []string {
	return parseEndpointLine(line, "api", false)
}

func parseEndpointLine(line, keyword string, action bool) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 4 && len(tokens) != 6 {
		return nil
	}
	if tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != keyword {
		return nil
	}
	if tokens[1].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[1].Lexeme) {
		return nil
	}
	if tokens[2].Kind != syntax.TokenIdentifier || !methodToken(tokens[2].Lexeme) {
		return nil
	}
	if action {
		if tokens[2].Lexeme != "POST" {
			return nil
		}
	} else if !apiMethodToken(tokens[2].Lexeme) {
		return nil
	}
	if tokens[3].Kind != syntax.TokenString {
		return nil
	}
	errorPath := ""
	if len(tokens) == 6 {
		if tokens[4].Kind != syntax.TokenIdentifier || tokens[4].Lexeme != "error" || tokens[5].Kind != syntax.TokenString {
			return nil
		}
		errorPath = decodeStringLiteral(tokens[5].Lexeme)
	}
	return []string{line, tokens[1].Lexeme, tokens[2].Lexeme, decodeStringLiteral(tokens[3].Lexeme), errorPath}
}

func parseFragmentEndpointLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 6 || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "fragment" {
		return nil
	}
	if tokens[1].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[1].Lexeme) {
		return nil
	}
	if tokens[2].Kind != syntax.TokenIdentifier || !apiMethodToken(tokens[2].Lexeme) {
		return nil
	}
	if tokens[3].Kind != syntax.TokenString || tokens[4].Kind != syntax.TokenString || tokens[5].Kind != syntax.TokenLBrace {
		return nil
	}
	return []string{line, tokens[1].Lexeme, tokens[2].Lexeme, decodeStringLiteral(tokens[3].Lexeme), decodeStringLiteral(tokens[4].Lexeme)}
}

func parseActionBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "act")
	if !ok || !isBlockName(body) {
		return nil
	}
	return []string{strings.TrimSpace(line), body}
}

func parseAPIBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "api")
	if !ok || (body != "" && !isBlockName(body)) {
		return nil
	}
	return []string{strings.TrimSpace(line), body}
}

func parsePropLine(line string) []string {
	raw := strings.TrimSpace(line)
	left, defaultValue, hasDefault := strings.Cut(raw, "=")
	if hasDefault {
		defaultValue = strings.TrimSpace(defaultValue)
		if defaultValue == "" {
			return nil
		}
	}
	tokens := syntaxTokens(left)
	if len(tokens) != 2 || !isIdentifierToken(tokens[0]) || !isIdentifierToken(tokens[1]) {
		return nil
	}
	if !isStrictIdent(tokens[0].Lexeme) || !isStrictIdent(tokens[1].Lexeme) {
		return nil
	}
	return []string{line, tokens[0].Lexeme, tokens[1].Lexeme, defaultValue}
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
	tokens := syntaxTokens(line)
	if len(tokens) != 2 && len(tokens) != 5 {
		return nil
	}
	if tokens[0].Kind != syntax.TokenIdentifier || (tokens[0].Lexeme != "props" && tokens[0].Lexeme != "state") {
		return nil
	}
	typeAlias, typeName, ok := splitQualifiedIdentifier(tokens[1])
	if !ok {
		return nil
	}
	initAlias, initName := "", ""
	if len(tokens) == 5 {
		if tokens[2].Kind != syntax.TokenAssign {
			return nil
		}
		initAlias, initName, ok = parseQualifiedCall(tokens[3:])
		if !ok {
			return nil
		}
	}
	return []string{line, tokens[0].Lexeme, typeAlias, typeName, initAlias, initName}
}

func parseStoreLine(line string) []string {
	tokens := syntaxTokens(line)
	// 6 tokens: `store <name> <pkg.Type> = <pkg.NewFn> ()`.
	// 8 tokens: the same followed by `persist "<scope>"`.
	if (len(tokens) != 6 && len(tokens) != 8) || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "store" {
		return nil
	}
	if tokens[1].Kind != syntax.TokenIdentifier || !isStrictIdent(tokens[1].Lexeme) || tokens[3].Kind != syntax.TokenAssign {
		return nil
	}
	typeAlias, typeName, ok := splitQualifiedIdentifier(tokens[2])
	if !ok {
		return nil
	}
	initAlias, initName, ok := parseQualifiedCall(tokens[4:6])
	if !ok {
		return nil
	}
	persistScope := ""
	persistSet := ""
	if len(tokens) == 8 {
		if tokens[6].Kind != syntax.TokenIdentifier || tokens[6].Lexeme != "persist" || tokens[7].Kind != syntax.TokenString {
			return nil
		}
		persistScope = syntax.Unquote(tokens[7].Lexeme)
		// Record that the clause was present so an explicit `persist ""` is
		// distinguishable from no persistence (both have an empty scope).
		persistSet = "1"
	}
	return []string{line, tokens[1].Lexeme, typeAlias, typeName, initAlias, initName, persistScope, persistSet}
}

func parseActionInputLine(line string) []string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 4 || fields[1] != ":=" || fields[2] != "form" || !isStrictIdent(fields[0]) || !isStrictIdent(fields[3]) {
		return nil
	}
	return []string{line, fields[0], fields[3]}
}

func parseActionValidLine(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "valid(") || !strings.HasSuffix(line, ")?") {
		return nil
	}
	name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "valid("), ")?"))
	if !isStrictIdent(name) {
		return nil
	}
	return []string{line, name}
}

func parseActionRedirectLine(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "->") {
		return nil
	}
	target, ok := parseOnlyString(strings.TrimSpace(strings.TrimPrefix(line, "->")))
	if !ok {
		return nil
	}
	return []string{line, target}
}

func parseActionFragmentLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 3 || tokens[0].Kind != syntax.TokenIdentifier || tokens[0].Lexeme != "fragment" {
		return nil
	}
	if tokens[1].Kind != syntax.TokenString || tokens[2].Kind != syntax.TokenLBrace {
		return nil
	}
	return []string{line, decodeStringLiteral(tokens[1].Lexeme)}
}

func parseAPIRouteLine(line string) []string {
	tokens := syntaxTokens(line)
	if len(tokens) != 2 || tokens[0].Kind != syntax.TokenIdentifier || !apiMethodToken(tokens[0].Lexeme) || tokens[1].Kind != syntax.TokenString {
		return nil
	}
	return []string{line, tokens[0].Lexeme, decodeStringLiteral(tokens[1].Lexeme)}
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
	tokens := syntaxTokens(line)
	if len(tokens) != 2 || tokens[0].Kind != syntax.TokenIdentifier || tokens[1].Kind != syntax.TokenLBrace || !isSyntaxBlockName(tokens[0].Lexeme) {
		return nil
	}
	return []string{line, tokens[0].Lexeme}
}

func parseGoBlockLine(line string) []string {
	body, ok := keywordBlockBody(line, "go")
	if !ok || (body != "" && !isBlockName(body)) {
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
	if len(line) <= len(keyword) || (line[len(keyword)] != ' ' && line[len(keyword)] != '\t') {
		return "", false
	}
	return rest, true
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

func isIdentifierToken(token syntax.Token) bool {
	return token.Kind == syntax.TokenIdentifier || token.Kind == syntax.TokenMetadata
}

func splitQualifiedIdentifier(token syntax.Token) (string, string, bool) {
	if !isIdentifierToken(token) {
		return "", "", false
	}
	alias, name, ok := strings.Cut(token.Lexeme, ".")
	if !ok || strings.Contains(name, ".") || !isStrictIdent(alias) || !isStrictIdent(name) {
		return "", "", false
	}
	return alias, name, true
}

func parseQualifiedCall(tokens []syntax.Token) (string, string, bool) {
	if len(tokens) != 2 || tokens[1].Kind != syntax.TokenText || tokens[1].Lexeme != "()" {
		return "", "", false
	}
	return splitQualifiedIdentifier(tokens[0])
}

func methodToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func apiMethodToken(value string) bool {
	if !methodToken(value) {
		return false
	}
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func isSyntaxBlockName(name string) bool {
	switch name {
	case "paths", "build", "server", "client", "view", "style", "props", "exports", "emits":
		return true
	default:
		return false
	}
}

func parseOnlyString(value string) (string, bool) {
	tokens := syntaxTokens(value)
	if len(tokens) != 1 || tokens[0].Kind != syntax.TokenString {
		return "", false
	}
	return decodeStringLiteral(tokens[0].Lexeme), true
}

func decodeStringLiteral(lexeme string) string {
	if len(lexeme) < 2 || lexeme[0] != '"' {
		return strings.Trim(lexeme, "\"")
	}
	var builder strings.Builder
	for index := 1; index < len(lexeme); index++ {
		ch := lexeme[index]
		if ch == '"' {
			break
		}
		if ch == '\\' {
			if index+1 >= len(lexeme) {
				builder.WriteByte(ch)
				continue
			}
			index++
			switch lexeme[index] {
			case '"', '\\':
				builder.WriteByte(lexeme[index])
			case 'n':
				builder.WriteByte('\n')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteByte('\\')
				builder.WriteByte(lexeme[index])
			}
			continue
		}
		builder.WriteByte(ch)
	}
	return builder.String()
}
