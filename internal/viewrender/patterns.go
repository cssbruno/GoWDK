package viewrender

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
	islandTextBindingPattern   = viewPattern{submatch: parseIslandTextBinding}
	forDirectivePattern        = viewPattern{submatch: parseForDirectivePattern}
	contractReferencePattern   = viewPattern{match: isContractReference}
	eventNamePattern           = viewPattern{match: isEventName}
	stylePropertyPattern       = viewPattern{match: isStyleProperty}
	styleCustomPropertyPattern = viewPattern{match: isStyleCustomProperty}
)

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

func isPatternSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
