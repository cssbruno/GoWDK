package lsp

import "strings"

func inferredComponentFields(viewBody, clientBody string) []string {
	fields := map[string]bool{}
	collectInterpolationFields(viewBody, fields)
	collectBindingFields(viewBody, fields)
	collectAssignmentFields(clientBody, fields)
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	return out
}

func collectInterpolationFields(source string, fields map[string]bool) {
	for index := 0; index < len(source); index++ {
		if source[index] != '{' {
			continue
		}
		end := strings.IndexByte(source[index+1:], '}')
		if end < 0 {
			return
		}
		end += index + 1
		name := strings.TrimSpace(source[index+1 : end])
		if isLSPIdentifier(name) {
			fields[name] = true
		}
		index = end
	}
}

func collectBindingFields(source string, fields map[string]bool) {
	for cursor := 0; cursor < len(source); {
		index := strings.Index(source[cursor:], "g:bind:")
		if index < 0 {
			return
		}
		index += cursor
		rest := source[index+len("g:bind:"):]
		switch {
		case strings.HasPrefix(rest, "value"):
			rest = rest[len("value"):]
		case strings.HasPrefix(rest, "checked"):
			rest = rest[len("checked"):]
		default:
			cursor = index + len("g:bind:")
			continue
		}
		rest = strings.TrimLeftFunc(rest, isLSPSpace)
		if !strings.HasPrefix(rest, "=") {
			cursor = index + len("g:bind:")
			continue
		}
		rest = strings.TrimLeftFunc(rest[1:], isLSPSpace)
		if !strings.HasPrefix(rest, "{") {
			cursor = index + len("g:bind:")
			continue
		}
		end := strings.IndexByte(rest[1:], '}')
		if end < 0 {
			return
		}
		name := strings.TrimSpace(rest[1 : end+1])
		if isLSPIdentifier(name) {
			fields[name] = true
		}
		cursor = index + len("g:bind:")
	}
}

func collectAssignmentFields(source string, fields map[string]bool) {
	for cursor := 0; cursor < len(source); cursor++ {
		if !isLSPIdentStart(source[cursor]) {
			continue
		}
		start := cursor
		cursor++
		for cursor < len(source) && isLSPIdentPart(source[cursor]) {
			cursor++
		}
		name := source[start:cursor]
		rest := strings.TrimLeftFunc(source[cursor:], isLSPSpace)
		if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "++") || strings.HasPrefix(rest, "--") {
			fields[name] = true
		}
	}
}

func isLSPIdentifier(value string) bool {
	if value == "" || !isLSPIdentStart(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isLSPIdentPart(value[index]) {
			return false
		}
	}
	return true
}

func isLSPIdentStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isLSPIdentPart(char byte) bool {
	return isLSPIdentStart(char) || (char >= '0' && char <= '9')
}

func isLSPSpace(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
