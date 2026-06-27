package buildgen

import "strings"

// This file owns the dependency-free CSS minifier used when emitting generated
// stylesheets. It collapses insignificant whitespace and strips comments while
// preserving string contents. Like css_scope.go, it is a pure transform with no
// dependency on the CSS planning types in css.go.

func minifyCSS(contents []byte) []byte {
	out := make([]rune, 0, len(contents))
	inString := rune(0)
	escaped := false
	pendingSpace := false
	last := rune(0)
	parenDepth := 0
	runes := []rune(string(contents))
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if inString != 0 {
			out = append(out, current)
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
			last = current
			continue
		}
		if current == '/' && index+1 < len(runes) && runes[index+1] == '*' {
			index++
			for index+1 < len(runes) && (runes[index] != '*' || runes[index+1] != '/') {
				index++
			}
			if index+1 < len(runes) {
				index++
			}
			continue
		}
		if current == '"' || current == '\'' {
			if pendingSpace && cssNeedsSpaceBefore(last, current, parenDepth) {
				out = append(out, ' ')
			}
			pendingSpace = false
			out = append(out, current)
			inString = current
			last = current
			continue
		}
		if isCSSWhitespace(current) {
			pendingSpace = true
			continue
		}
		if isCSSPunctuation(current) && (parenDepth <= 0 || current != '+') {
			if current == '(' {
				parenDepth++
			}
			if current == ')' && parenDepth > 0 {
				parenDepth--
			}
			if current == '(' && pendingSpace && isCSSIdentRune(last) {
				out = append(out, ' ')
			} else {
				out = trimTrailingCSSSpace(out)
			}
			pendingSpace = false
			out = append(out, current)
			last = current
			continue
		}
		if pendingSpace && cssNeedsSpaceBefore(last, current, parenDepth) {
			out = append(out, ' ')
		}
		pendingSpace = false
		out = append(out, current)
		last = current
	}
	return []byte(strings.TrimSpace(string(out)))
}

func isCSSWhitespace(value rune) bool {
	return value == ' ' || value == '\n' || value == '\r' || value == '\t' || value == '\f'
}

func isCSSPunctuation(value rune) bool {
	switch value {
	case '{', '}', ':', ';', ',', '>', '+', '~', '(', ')':
		return true
	default:
		return false
	}
}

func isCSSIdentRune(value rune) bool {
	if value == '-' || value == '_' {
		return true
	}
	if value >= 'a' && value <= 'z' {
		return true
	}
	if value >= 'A' && value <= 'Z' {
		return true
	}
	return value >= '0' && value <= '9'
}

func cssNeedsSpaceBefore(previous rune, current rune, parenDepth int) bool {
	if parenDepth > 0 && (previous == '+' || current == '+') {
		return true
	}
	if previous == ')' && !isCSSPunctuation(current) {
		return true
	}
	if previous == 0 || isCSSPunctuation(previous) {
		return false
	}
	return !isCSSPunctuation(current)
}

func trimTrailingCSSSpace(values []rune) []rune {
	for len(values) > 0 && isCSSWhitespace(values[len(values)-1]) {
		values = values[:len(values)-1]
	}
	return values
}
