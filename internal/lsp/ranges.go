package lsp

import (
	"strings"
	"unicode/utf16"
)

func tokenAtPosition(source string, pos position) string {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	index := byteIndexFromUTF16Column(line, pos.Character)
	if index > len(line) {
		index = len(line)
	}
	start := index
	for start > 0 && hoverTokenByte(line[start-1]) {
		start--
	}
	end := index
	for end < len(line) && hoverTokenByte(line[end]) {
		end++
	}
	return strings.TrimSpace(line[start:end])
}

func referenceRanges(source string, token string) []lspRange {
	if token == "" {
		return nil
	}
	var ranges []lspRange
	searchStart := 0
	for {
		index := strings.Index(source[searchStart:], token)
		if index < 0 {
			return ranges
		}
		start := searchStart + index
		end := start + len(token)
		if isReferenceBoundary(source, token, start, end) {
			ranges = append(ranges, rangeFromByteSpan(source, start, end))
		}
		searchStart = end
	}
}

func isReferenceBoundary(source string, token string, start int, end int) bool {
	if start > 0 && isIdentifierLikeByte(token[0]) && isIdentifierLikeByte(source[start-1]) {
		return false
	}
	if end < len(source) && isIdentifierLikeByte(token[len(token)-1]) && isIdentifierLikeByte(source[end]) {
		return false
	}
	return true
}

func componentCallAtPosition(source string, pos position) (string, bool) {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", false
	}
	line := lines[pos.Line]
	index := byteIndexFromUTF16Column(line, pos.Character)
	if index > len(line) {
		index = len(line)
	}
	start := index
	for start > 0 && componentCallNameByte(line[start-1]) {
		start--
	}
	end := index
	for end < len(line) && componentCallNameByte(line[end]) {
		end++
	}
	if start == end || start == 0 {
		return "", false
	}
	name := line[start:end]
	if !isGOWDKComponentCallName(name) {
		return "", false
	}
	before := start - 1
	if line[before] == '<' {
		return name, true
	}
	if line[before] == '/' && before > 0 && line[before-1] == '<' {
		return name, true
	}
	return "", false
}

func byteIndexFromUTF16Column(line string, column int) int {
	if column <= 0 {
		return 0
	}
	units := 0
	for index, r := range line {
		next := units + len(utf16.Encode([]rune{r}))
		if next > column {
			return index
		}
		units = next
	}
	return len(line)
}

func rangeFromByteSpan(source string, start int, end int) lspRange {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	startPosition := positionFromByteOffset(source, start)
	endPosition := positionFromByteOffset(source, end)
	return lspRange{Start: startPosition, End: endPosition}
}

func positionFromByteOffset(source string, offset int) position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	prefix := source[:offset]
	line := strings.Count(prefix, "\n")
	lineStart := strings.LastIndex(prefix, "\n") + 1
	return position{
		Line:      line,
		Character: utf16Length(source[lineStart:offset]),
	}
}

func componentCallNameByte(value byte) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= '0' && value <= '9':
		return true
	case strings.ContainsRune("_-:.", rune(value)):
		return true
	default:
		return false
	}
}

func isIdentifierLikeByte(value byte) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= '0' && value <= '9':
		return true
	case value == '_':
		return true
	default:
		return false
	}
}

func isGOWDKComponentCallName(value string) bool {
	if alias, name, ok := strings.Cut(value, "."); ok {
		return isGOWDKIdentifier(alias) && isExportedGOWDKName(name)
	}
	return isExportedGOWDKName(value)
}

func isGOWDKIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch {
		case index == 0 && (char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'):
		case index > 0 && (char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9'):
		default:
			return false
		}
	}
	return true
}

func isExportedGOWDKName(value string) bool {
	return value != "" && value[0] >= 'A' && value[0] <= 'Z'
}

func hoverTokenByte(value byte) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= '0' && value <= '9':
		return true
	case strings.ContainsRune("@:_-./", rune(value)):
		return true
	default:
		return false
	}
}
