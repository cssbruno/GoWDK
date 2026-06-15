package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidatePattern reports whether pattern uses the generated action validation
// syntax supported by MatchPattern.
func ValidatePattern(pattern string) error {
	_, err := compilePattern(pattern)
	return err
}

// MatchPattern reports whether value matches pattern from start to end.
func MatchPattern(pattern string, value string) (bool, error) {
	compiled, err := compilePattern(pattern)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(value), nil
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	normalized, err := normalizePattern(pattern)
	if err != nil {
		return nil, err
	}
	return regexp.Compile(`^(?:` + normalized + `)$`)
}

func normalizePattern(pattern string) (string, error) {
	source := []rune(pattern)
	if len(source) > 0 && source[0] == '^' {
		source = source[1:]
	}
	if len(source) > 0 && source[len(source)-1] == '$' && !patternRuneEscaped(source, len(source)-1) {
		source = source[:len(source)-1]
	}
	if err := validatePatternSubset(source); err != nil {
		return "", err
	}
	var builder strings.Builder
	inClass := false
	escaped := false
	for _, r := range source {
		if escaped {
			builder.WriteRune('\\')
			builder.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '[':
			inClass = true
			builder.WriteRune(r)
		case ']':
			inClass = false
			builder.WriteRune(r)
		case '^', '$':
			if inClass {
				builder.WriteRune(r)
				continue
			}
			builder.WriteRune('\\')
			builder.WriteRune(r)
		default:
			builder.WriteRune(r)
		}
	}
	if escaped {
		builder.WriteRune('\\')
	}
	return builder.String(), nil
}

func validatePatternSubset(source []rune) error {
	inClass := false
	escaped := false
	for index, r := range source {
		if escaped {
			if inClass && (r == 'D' || r == 'W' || r == 'S') {
				return fmt.Errorf("negated shorthand escapes are not supported inside character classes")
			}
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if index+1 < len(source) && source[index+1] == '?' {
				if index+2 >= len(source) || source[index+2] != ':' {
					return fmt.Errorf("unsupported group operator")
				}
			}
		case '*', '+', '?':
			if index+1 < len(source) && source[index+1] == '?' {
				return fmt.Errorf("quantifier %q has no target", source[index+1])
			}
		case '}':
			if index+1 < len(source) && source[index+1] == '?' {
				return fmt.Errorf("quantifier %q has no target", source[index+1])
			}
		}
	}
	return nil
}

func patternRuneEscaped(source []rune, index int) bool {
	backslashes := 0
	for cursor := index - 1; cursor >= 0 && source[cursor] == '\\'; cursor-- {
		backslashes++
	}
	return backslashes%2 == 1
}
