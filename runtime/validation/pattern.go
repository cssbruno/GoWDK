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
	compiled, err := regexp.Compile(`^(?:` + normalized + `)$`)
	if err != nil {
		return nil, err
	}
	return compiled, nil
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
	for index := 0; index < len(source); index++ {
		char := source[index]
		if char == '\\' {
			builder.WriteRune(char)
			if index+1 < len(source) {
				index++
				builder.WriteRune(source[index])
			}
			continue
		}
		switch char {
		case '[':
			inClass = true
			builder.WriteRune(char)
		case ']':
			inClass = false
			builder.WriteRune(char)
		case '^', '$':
			if inClass {
				builder.WriteRune(char)
			} else {
				builder.WriteRune('\\')
				builder.WriteRune(char)
			}
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String(), nil
}

func validatePatternSubset(source []rune) error {
	inClass := false
	for index := 0; index < len(source); index++ {
		char := source[index]
		if char == '\\' {
			if index+1 >= len(source) {
				return fmt.Errorf("dangling escape")
			}
			if inClass {
				next := source[index+1]
				if next == 'D' || next == 'W' || next == 'S' {
					return fmt.Errorf("negated shorthand escapes are not supported inside character classes")
				}
			}
			index++
			continue
		}
		switch char {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass && index+1 < len(source) && source[index+1] == '?' {
				if index+2 >= len(source) || source[index+2] != ':' {
					return fmt.Errorf("unsupported group operator")
				}
			}
		case '*', '+', '?':
			if !inClass && index+1 < len(source) && source[index+1] == '?' {
				return fmt.Errorf("lazy quantifiers are not supported")
			}
		case '}':
			if !inClass && index+1 < len(source) && source[index+1] == '?' {
				return fmt.Errorf("lazy quantifiers are not supported")
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
