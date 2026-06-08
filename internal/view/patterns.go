package view

import "strings"

type viewPattern struct {
	match    func(string) bool
	submatch func(string) []string
}

func (pattern viewPattern) MatchString(source string) bool {
	if pattern.match != nil {
		return pattern.match(source)
	}
	return pattern.FindStringSubmatch(source) != nil
}

func (pattern viewPattern) FindStringSubmatch(source string) []string {
	if pattern.submatch != nil {
		return pattern.submatch(source)
	}
	if pattern.match != nil && pattern.match(source) {
		return []string{source}
	}
	return nil
}

var (
	islandFieldPattern         = viewPattern{match: isIdentifier}
	islandIncDecPattern        = viewPattern{submatch: parseIslandIncDec}
	islandAssignPattern        = viewPattern{submatch: parseIslandAssign}
	islandTogglePattern        = viewPattern{submatch: parseIslandToggle}
	islandNumberPattern        = viewPattern{match: isIslandNumber}
	islandTextBindingPattern   = viewPattern{submatch: parseIslandTextBinding}
	islandRefCallPattern       = viewPattern{submatch: parseIslandRefCall}
	islandLetPattern           = viewPattern{submatch: parseIslandLet}
	islandAwaitFetchPattern    = viewPattern{submatch: parseIslandAwaitFetch}
	forDirectivePattern        = viewPattern{submatch: parseForDirectivePattern}
	contractReferencePattern   = viewPattern{match: isContractReference}
	eventNamePattern           = viewPattern{match: isEventName}
	stylePropertyPattern       = viewPattern{match: isStyleProperty}
	styleCustomPropertyPattern = viewPattern{match: isStyleCustomProperty}
)

func parseIslandIncDec(source string) []string {
	for _, operator := range []string{"++", "--"} {
		name, ok := strings.CutSuffix(source, operator)
		if ok && isIdentifier(name) {
			return []string{source, name, operator}
		}
	}
	return nil
}

func parseIslandAssign(source string) []string {
	if source == "" || !isIdentStart(source[0]) {
		return nil
	}
	cursor := 1
	for cursor < len(source) && isIdentPart(source[cursor]) {
		cursor++
	}
	name := source[:cursor]
	for cursor < len(source) && isPatternSpace(source[cursor]) {
		cursor++
	}
	if cursor >= len(source) || source[cursor] != '=' {
		return nil
	}
	cursor++
	for cursor < len(source) && isPatternSpace(source[cursor]) {
		cursor++
	}
	if cursor >= len(source) {
		return nil
	}
	return []string{source, name, source[cursor:]}
}

func parseIslandToggle(source string) []string {
	if !strings.HasPrefix(source, "!") {
		return nil
	}
	name := strings.TrimSpace(source[1:])
	if !isIdentifier(name) {
		return nil
	}
	return []string{source, name}
}

func isIslandNumber(source string) bool {
	if source == "" {
		return false
	}
	cursor := 0
	if source[cursor] == '-' {
		cursor++
		if cursor == len(source) {
			return false
		}
	}
	digits := 0
	for cursor < len(source) && source[cursor] >= '0' && source[cursor] <= '9' {
		cursor++
		digits++
	}
	if digits == 0 {
		return false
	}
	if cursor == len(source) {
		return true
	}
	if source[cursor] != '.' {
		return false
	}
	cursor++
	fractionDigits := 0
	for cursor < len(source) && source[cursor] >= '0' && source[cursor] <= '9' {
		cursor++
		fractionDigits++
	}
	return fractionDigits > 0 && cursor == len(source)
}

func parseIslandTextBinding(source string) []string {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return nil
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if !isIdentifier(name) {
		return nil
	}
	return []string{source, name}
}

func parseIslandRefCall(source string) []string {
	name, method, ok := strings.Cut(source, ".")
	if !ok || !isIdentifier(name) {
		return nil
	}
	method, ok = strings.CutSuffix(method, "()")
	if !ok {
		return nil
	}
	switch method {
	case "Focus", "Blur", "ScrollIntoView":
		return []string{source, name, method}
	default:
		return nil
	}
}

func parseIslandLet(source string) []string {
	rest, ok := strings.CutPrefix(source, "let")
	if !ok || rest == "" || !isPatternSpace(rest[0]) {
		return nil
	}
	rest = strings.TrimSpace(rest)
	name, rest, ok := nextPatternIdent(rest)
	if !ok {
		return nil
	}
	rest = strings.TrimLeftFunc(rest, func(r rune) bool { return isPatternSpaceRune(r) })
	typ, rest, ok := nextPatternIdent(rest)
	if !ok {
		return nil
	}
	rest = strings.TrimLeftFunc(rest, func(r rune) bool { return isPatternSpaceRune(r) })
	if !strings.HasPrefix(rest, "=") {
		return nil
	}
	expr := strings.TrimSpace(rest[1:])
	if expr == "" {
		return nil
	}
	return []string{source, name, typ, expr}
}

func parseIslandAwaitFetch(source string) []string {
	rest, ok := strings.CutPrefix(source, "await")
	if !ok || rest == "" || !isPatternSpace(rest[0]) {
		return nil
	}
	rest = strings.TrimSpace(rest)
	rest, ok = strings.CutPrefix(rest, "fetchJSON[")
	if !ok {
		return nil
	}
	closeType := strings.LastIndex(rest, "](")
	if closeType < 0 {
		return nil
	}
	typ := rest[:closeType]
	args := rest[closeType+1:]
	if !strings.HasPrefix(args, "(") || !strings.HasSuffix(args, ")") || typ == "" {
		return nil
	}
	return []string{source, typ, args[1 : len(args)-1]}
}

func parseForDirectivePattern(source string) []string {
	inStart, inEnd, ok := findForInOperator(source)
	if !ok {
		return nil
	}
	left := source[:inStart]
	collection := strings.TrimSpace(source[inEnd:])
	if collection == "" {
		return nil
	}
	item := strings.TrimSpace(left)
	index := ""
	if before, after, ok := strings.Cut(left, ","); ok {
		item = strings.TrimSpace(before)
		index = strings.TrimSpace(after)
		if index == "" {
			return nil
		}
	}
	if !isIdentifier(item) || (index != "" && !isIdentifier(index)) {
		return nil
	}
	return []string{source, item, index, collection}
}

func findForInOperator(source string) (int, int, bool) {
	for index := 0; index < len(source); index++ {
		if source[index] != 'i' || index+2 > len(source) || source[index:index+2] != "in" {
			continue
		}
		if index == 0 || index+2 == len(source) || !isPatternSpace(source[index-1]) || !isPatternSpace(source[index+2]) {
			continue
		}
		start := index - 1
		for start > 0 && isPatternSpace(source[start-1]) {
			start--
		}
		end := index + 2
		for end < len(source) && isPatternSpace(source[end]) {
			end++
		}
		return start, end, true
	}
	return 0, 0, false
}

func nextPatternIdent(source string) (string, string, bool) {
	if source == "" || !isIdentStart(source[0]) {
		return "", "", false
	}
	cursor := 1
	for cursor < len(source) && isIdentPart(source[cursor]) {
		cursor++
	}
	return source[:cursor], source[cursor:], true
}

func isContractReference(source string) bool {
	parts := strings.Split(source, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !isIdentifier(part) {
			return false
		}
	}
	return true
}

func isEventName(source string) bool {
	if source == "" || !((source[0] >= 'A' && source[0] <= 'Z') || (source[0] >= 'a' && source[0] <= 'z')) {
		return false
	}
	for index := 1; index < len(source); index++ {
		char := source[index]
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isStyleProperty(source string) bool {
	if source == "" || source[0] < 'a' || source[0] > 'z' {
		return false
	}
	for index := 1; index < len(source); index++ {
		char := source[index]
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isStyleCustomProperty(source string) bool {
	body, ok := strings.CutPrefix(source, "--")
	if !ok || body == "" {
		return false
	}
	for index := 0; index < len(body); index++ {
		char := body[index]
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isIdentStart(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_'
}

func isIdentPart(char byte) bool {
	return isIdentStart(char) || (char >= '0' && char <= '9')
}

func isPatternSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

func isPatternSpaceRune(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
