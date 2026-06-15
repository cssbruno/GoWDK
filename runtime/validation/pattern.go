package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const maxGoRepeatCount = 1000

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
	rewritten, err := rewriteLargeRepeatQuantifiers(source)
	if err != nil {
		return "", err
	}
	source = rewritten
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

func rewriteLargeRepeatQuantifiers(source []rune) ([]rune, error) {
	var rewritten []rune
	inClass := false
	escaped := false
	for index := 0; index < len(source); index++ {
		r := source[index]
		if escaped {
			rewritten = append(rewritten, r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			rewritten = append(rewritten, r)
			escaped = true
		case '[':
			rewritten = append(rewritten, r)
			inClass = true
		case ']':
			rewritten = append(rewritten, r)
			inClass = false
		case '{':
			if inClass {
				rewritten = append(rewritten, r)
				continue
			}
			quantifier, ok, err := parseRepeatQuantifier(source, index)
			if err != nil {
				return nil, err
			}
			if !ok {
				rewritten = append(rewritten, r)
				continue
			}
			if !quantifier.large() {
				rewritten = append(rewritten, source[index:quantifier.end+1]...)
				index = quantifier.end
				continue
			}
			atomStart, ok := repeatAtomStart(rewritten)
			if !ok {
				return nil, fmt.Errorf("repeat quantifier has no target")
			}
			atom := append([]rune(nil), rewritten[atomStart:]...)
			rewritten = rewritten[:atomStart]
			rewritten = appendExpandedRepeat(rewritten, atom, quantifier)
			index = quantifier.end
		default:
			rewritten = append(rewritten, r)
		}
	}
	return rewritten, nil
}

type repeatQuantifier struct {
	min    int
	max    int
	maxSet bool
	end    int
}

func (quantifier repeatQuantifier) large() bool {
	return quantifier.min > maxGoRepeatCount || (quantifier.maxSet && quantifier.max > maxGoRepeatCount)
}

func parseRepeatQuantifier(source []rune, start int) (repeatQuantifier, bool, error) {
	index := start + 1
	minStart := index
	for index < len(source) && source[index] >= '0' && source[index] <= '9' {
		index++
	}
	if index == minStart {
		return repeatQuantifier{}, false, nil
	}
	min, err := strconv.Atoi(string(source[minStart:index]))
	if err != nil {
		return repeatQuantifier{}, false, err
	}
	quantifier := repeatQuantifier{min: min, max: min, maxSet: true}
	if index < len(source) && source[index] == ',' {
		index++
		maxStart := index
		for index < len(source) && source[index] >= '0' && source[index] <= '9' {
			index++
		}
		quantifier.maxSet = index > maxStart
		if quantifier.maxSet {
			max, err := strconv.Atoi(string(source[maxStart:index]))
			if err != nil {
				return repeatQuantifier{}, false, err
			}
			quantifier.max = max
		}
	}
	if index >= len(source) || source[index] != '}' {
		return repeatQuantifier{}, false, nil
	}
	if quantifier.maxSet && quantifier.max < quantifier.min {
		return repeatQuantifier{}, false, fmt.Errorf("invalid repeat count")
	}
	quantifier.end = index
	return quantifier, true, nil
}

func repeatAtomStart(source []rune) (int, bool) {
	if len(source) == 0 {
		return 0, false
	}
	last := len(source) - 1
	switch source[last] {
	case ']':
		for index := last - 1; index >= 0; index-- {
			if source[index] == '[' && !patternRuneEscaped(source, index) {
				return index, true
			}
		}
		return 0, false
	case ')':
		inClass := false
		depth := 0
		for index := last; index >= 0; index-- {
			if patternRuneEscaped(source, index) {
				continue
			}
			switch source[index] {
			case ']':
				inClass = true
			case '[':
				inClass = false
			case ')':
				if !inClass {
					depth++
				}
			case '(':
				if !inClass {
					depth--
					if depth == 0 {
						return index, true
					}
				}
			}
		}
		return 0, false
	default:
		if patternRuneEscaped(source, last) {
			return last - 1, true
		}
		return last, true
	}
}

func appendExpandedRepeat(target []rune, atom []rune, quantifier repeatQuantifier) []rune {
	target = appendExactRepeat(target, atom, quantifier.min)
	if !quantifier.maxSet {
		target = append(target, atom...)
		target = append(target, '*')
		return target
	}
	for remaining := quantifier.max - quantifier.min; remaining > 0; {
		chunk := remaining
		if chunk > maxGoRepeatCount {
			chunk = maxGoRepeatCount
		}
		target = append(target, atom...)
		target = append(target, []rune(fmt.Sprintf("{0,%d}", chunk))...)
		remaining -= chunk
	}
	return target
}

func appendExactRepeat(target []rune, atom []rune, count int) []rune {
	for remaining := count; remaining > 0; {
		chunk := remaining
		if chunk > maxGoRepeatCount {
			chunk = maxGoRepeatCount
		}
		target = append(target, atom...)
		if chunk > 1 {
			target = append(target, []rune(fmt.Sprintf("{%d}", chunk))...)
		}
		remaining -= chunk
	}
	return target
}

func patternRuneEscaped(source []rune, index int) bool {
	backslashes := 0
	for cursor := index - 1; cursor >= 0 && source[cursor] == '\\'; cursor-- {
		backslashes++
	}
	return backslashes%2 == 1
}
