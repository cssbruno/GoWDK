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
	if source == "" || (source[0] < 'A' || source[0] > 'Z') && (source[0] < 'a' || source[0] > 'z') {
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
