package buildgen

import "strings"

// This file owns component CSS scoping: rewriting keyframes/animation names and
// appending the scope selector to rule selectors so component styles only apply
// inside their scoped subtree. It is a pure string-to-string transform with no
// dependency on the CSS planning types in css.go.

func scopeComponentCSS(contents []byte, scopeID string) []byte {
	if strings.TrimSpace(scopeID) == "" {
		return append([]byte(nil), contents...)
	}
	css := rewriteCSSKeyframes(string(contents), scopeID)
	return []byte(scopeCSSRules(css, componentCSSScopeSelector(scopeID)))
}

func rewriteCSSKeyframes(contents string, scopeID string) string {
	renames := map[string]string{}
	contents = rewriteCSSKeyframeDeclarations(contents, scopeID, renames)
	if len(renames) == 0 {
		return contents
	}
	return rewriteCSSAnimationDeclarations(contents, renames)
}

func rewriteCSSKeyframeDeclarations(contents string, scopeID string, renames map[string]string) string {
	var builder strings.Builder
	cursor := 0
	for cursor < len(contents) {
		at := strings.IndexByte(contents[cursor:], '@')
		if at < 0 {
			builder.WriteString(contents[cursor:])
			break
		}
		at += cursor
		nameStart, nameEnd, ok := cssKeyframeNameRange(contents, at)
		if !ok {
			builder.WriteString(contents[cursor : at+1])
			cursor = at + 1
			continue
		}
		name := contents[nameStart:nameEnd]
		scoped := name + "-" + scopeID
		renames[name] = scoped
		builder.WriteString(contents[cursor:nameStart])
		builder.WriteString(scoped)
		cursor = nameEnd
	}
	return builder.String()
}

func cssKeyframeNameRange(contents string, at int) (int, int, bool) {
	cursor := at + 1
	if cursor < len(contents) && contents[cursor] == '-' {
		cursor++
		for cursor < len(contents) && isCSSLetter(contents[cursor]) {
			cursor++
		}
		if cursor >= len(contents) || contents[cursor] != '-' {
			return 0, 0, false
		}
		cursor++
	}
	if !hasCSSWordAt(contents, cursor, "keyframes") {
		return 0, 0, false
	}
	cursor += len("keyframes")
	if cursor >= len(contents) || !isCSSSpace(contents[cursor]) {
		return 0, 0, false
	}
	for cursor < len(contents) && isCSSSpace(contents[cursor]) {
		cursor++
	}
	if cursor >= len(contents) || !isCSSNameStart(contents[cursor]) {
		return 0, 0, false
	}
	start := cursor
	cursor++
	for cursor < len(contents) && isCSSNamePart(contents[cursor]) {
		cursor++
	}
	return start, cursor, true
}

func rewriteCSSAnimationDeclarations(contents string, renames map[string]string) string {
	var builder strings.Builder
	cursor := 0
	for cursor < len(contents) {
		colon := strings.IndexByte(contents[cursor:], ':')
		if colon < 0 {
			builder.WriteString(contents[cursor:])
			break
		}
		colon += cursor
		propStart := cssDeclarationPropertyStart(contents, colon)
		property := strings.TrimSpace(contents[propStart:colon])
		if !isCSSAnimationProperty(property) {
			builder.WriteString(contents[cursor : colon+1])
			cursor = colon + 1
			continue
		}
		valueEnd := cssDeclarationValueEnd(contents, colon+1)
		value := replaceCSSAnimationNames(contents[colon+1:valueEnd], renames)
		builder.WriteString(contents[cursor : colon+1])
		builder.WriteString(value)
		cursor = valueEnd
	}
	return builder.String()
}

func cssDeclarationPropertyStart(contents string, colon int) int {
	start := colon
	for start > 0 {
		switch contents[start-1] {
		case '{', '}', ';':
			return start
		default:
			start--
		}
	}
	return start
}

func cssDeclarationValueEnd(contents string, start int) int {
	for cursor := start; cursor < len(contents); cursor++ {
		switch contents[cursor] {
		case ';', '}':
			return cursor
		case '\'', '"':
			cursor = cssStringEnd(contents, cursor)
		}
	}
	return len(contents)
}

func replaceCSSAnimationNames(value string, renames map[string]string) string {
	var builder strings.Builder
	for cursor := 0; cursor < len(value); {
		if !isCSSNameStart(value[cursor]) {
			builder.WriteByte(value[cursor])
			cursor++
			continue
		}
		start := cursor
		cursor++
		for cursor < len(value) && isCSSNamePart(value[cursor]) {
			cursor++
		}
		token := value[start:cursor]
		if scoped, ok := renames[token]; ok {
			builder.WriteString(scoped)
		} else {
			builder.WriteString(token)
		}
	}
	return builder.String()
}

func isCSSAnimationProperty(property string) bool {
	return strings.EqualFold(property, "animation") || strings.EqualFold(property, "animation-name")
}

func hasCSSWordAt(contents string, start int, word string) bool {
	if start+len(word) > len(contents) || !strings.EqualFold(contents[start:start+len(word)], word) {
		return false
	}
	return start+len(word) == len(contents) || !isCSSNamePart(contents[start+len(word)])
}

func isCSSNameStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isCSSNamePart(char byte) bool {
	return isCSSNameStart(char) || char == '-' || (char >= '0' && char <= '9')
}

func isCSSLetter(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isCSSSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func cssStringEnd(contents string, quote int) int {
	cursor := quote + 1
	for cursor < len(contents) {
		if contents[cursor] == '\\' {
			cursor += 2
			continue
		}
		if contents[cursor] == contents[quote] {
			return cursor
		}
		cursor++
	}
	return len(contents) - 1
}

func componentCSSScopeSelector(scopeID string) string {
	return `:where([data-gowdk-scope~="` + scopeID + `"])`
}

func scopeCSSRules(contents string, scopeSelector string) string {
	parts := make([]string, 0, 8)
	for cursor := 0; cursor < len(contents); {
		open := strings.IndexByte(contents[cursor:], '{')
		if open < 0 {
			parts = append(parts, contents[cursor:])
			break
		}
		open += cursor
		closeIndex := matchingCSSBrace(contents, open)
		if closeIndex < 0 {
			parts = append(parts, contents[cursor:])
			break
		}
		prefix := contents[cursor:open]
		body := contents[open+1 : closeIndex]
		selector := strings.TrimSpace(prefix)
		switch {
		case selector == "":
			parts = append(parts, prefix, "{", body, "}")
		case strings.HasPrefix(selector, "@"):
			parts = append(parts, prefix, "{")
			if cssAtRuleHasNestedRules(selector) {
				parts = append(parts, scopeCSSRules(body, scopeSelector))
			} else {
				parts = append(parts, body)
			}
			parts = append(parts, "}")
		default:
			leading := prefix[:len(prefix)-len(strings.TrimLeft(prefix, " \n\r\t\f"))]
			trailing := prefix[len(strings.TrimRight(prefix, " \n\r\t\f")):]
			parts = append(parts, leading, scopeCSSSelectorList(selector, scopeSelector), trailing, "{", body, "}")
		}
		cursor = closeIndex + 1
	}
	return strings.Join(parts, "")
}

func cssAtRuleHasNestedRules(selector string) bool {
	lower := strings.ToLower(strings.TrimSpace(selector))
	if strings.Contains(lower, "keyframes") {
		return false
	}
	return strings.HasPrefix(lower, "@media") || strings.HasPrefix(lower, "@supports") || strings.HasPrefix(lower, "@container") || strings.HasPrefix(lower, "@layer")
}

func matchingCSSBrace(contents string, open int) int {
	depth := 0
	inString := rune(0)
	escaped := false
	for index, current := range contents[open:] {
		absolute := open + index
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == inString {
				inString = 0
			}
			continue
		}
		if current == '"' || current == '\'' {
			inString = current
			continue
		}
		switch current {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return absolute
			}
		}
	}
	return -1
}

func scopeCSSSelectorList(selectorList string, scopeSelector string) string {
	selectors := splitCSSSelectorList(selectorList)
	for index, selector := range selectors {
		selectors[index] = scopeCSSSelector(strings.TrimSpace(selector), scopeSelector)
	}
	return strings.Join(selectors, ", ")
}

func splitCSSSelectorList(selectorList string) []string {
	var selectors []string
	start := 0
	parenDepth := 0
	bracketDepth := 0
	for index, current := range selectorList {
		switch current {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if parenDepth == 0 && bracketDepth == 0 {
				selectors = append(selectors, selectorList[start:index])
				start = index + 1
			}
		}
	}
	selectors = append(selectors, selectorList[start:])
	return selectors
}

func scopeCSSSelector(selector string, scopeSelector string) string {
	if selector == "" || strings.Contains(selector, ":global(") {
		return selector
	}
	if index := strings.LastIndex(selector, "::"); index >= 0 {
		return strings.TrimSpace(selector[:index]) + scopeSelector + selector[index:]
	}
	return selector + scopeSelector
}
