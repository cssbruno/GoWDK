package validation

import (
	"fmt"
	"strconv"
	"strings"
)

type patternAtom struct {
	kind   patternAtomKind
	lit    rune
	ranges [][2]rune
	negate bool
	min    int
	max    int
}

type patternAtomKind int

const (
	patternLiteral patternAtomKind = iota
	patternAny
	patternClass
)

// ValidatePattern reports whether pattern uses the generated action validation
// subset supported by MatchPattern.
func ValidatePattern(pattern string) error {
	_, err := parsePattern(pattern)
	return err
}

// MatchPattern reports whether value matches pattern from start to end.
func MatchPattern(pattern string, value string) (bool, error) {
	atoms, err := parsePattern(pattern)
	if err != nil {
		return false, err
	}
	runes := []rune(value)
	memo := map[[2]int]bool{}
	seen := map[[2]int]bool{}
	var match func(int, int) bool
	match = func(atomIndex, valueIndex int) bool {
		key := [2]int{atomIndex, valueIndex}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true
		if atomIndex == len(atoms) {
			memo[key] = valueIndex == len(runes)
			return memo[key]
		}
		atom := atoms[atomIndex]
		max := atom.max
		if max < 0 || max > len(runes)-valueIndex {
			max = len(runes) - valueIndex
		}
		cursor := valueIndex
		count := 0
		for count < max && cursor < len(runes) && atom.matches(runes[cursor]) {
			cursor++
			count++
		}
		for count >= atom.min {
			if match(atomIndex+1, valueIndex+count) {
				memo[key] = true
				return true
			}
			count--
		}
		memo[key] = false
		return false
	}
	return match(0, 0), nil
}

func parsePattern(pattern string) ([]patternAtom, error) {
	pattern = trimPatternAnchors(pattern)
	var atoms []patternAtom
	for cursor := 0; cursor < len(pattern); {
		atom, next, err := parsePatternAtom(pattern, cursor)
		if err != nil {
			return nil, err
		}
		atom.min = 1
		atom.max = 1
		cursor = next
		if cursor < len(pattern) {
			min, max, next, ok, err := parsePatternQuantifier(pattern, cursor)
			if err != nil {
				return nil, err
			}
			if ok {
				atom.min = min
				atom.max = max
				cursor = next
			}
		}
		atoms = append(atoms, atom)
	}
	return atoms, nil
}

func trimPatternAnchors(pattern string) string {
	if strings.HasPrefix(pattern, "^") {
		pattern = pattern[1:]
	}
	if strings.HasSuffix(pattern, "$") && !strings.HasSuffix(pattern, `\$`) {
		pattern = pattern[:len(pattern)-1]
	}
	return pattern
}

func parsePatternAtom(pattern string, cursor int) (patternAtom, int, error) {
	switch pattern[cursor] {
	case '.', '\\':
		if pattern[cursor] == '.' {
			return patternAtom{kind: patternAny}, cursor + 1, nil
		}
		if cursor+1 >= len(pattern) {
			return patternAtom{}, cursor, fmt.Errorf("dangling escape")
		}
		return patternAtom{kind: patternLiteral, lit: rune(pattern[cursor+1])}, cursor + 2, nil
	case '[':
		return parsePatternClass(pattern, cursor)
	case '(', ')', '|':
		return patternAtom{}, cursor, fmt.Errorf("unsupported pattern operator %q", pattern[cursor])
	case '*', '+', '?', '{', '}':
		return patternAtom{}, cursor, fmt.Errorf("quantifier %q has no target", pattern[cursor])
	default:
		r, size := nextPatternRune(pattern[cursor:])
		return patternAtom{kind: patternLiteral, lit: r}, cursor + size, nil
	}
}

func parsePatternClass(pattern string, cursor int) (patternAtom, int, error) {
	cursor++
	atom := patternAtom{kind: patternClass}
	if cursor < len(pattern) && pattern[cursor] == '^' {
		atom.negate = true
		cursor++
	}
	if cursor >= len(pattern) || pattern[cursor] == ']' {
		return patternAtom{}, cursor, fmt.Errorf("empty character class")
	}
	for cursor < len(pattern) && pattern[cursor] != ']' {
		start, next, err := parsePatternClassRune(pattern, cursor)
		if err != nil {
			return patternAtom{}, cursor, err
		}
		cursor = next
		if cursor+1 < len(pattern) && pattern[cursor] == '-' && pattern[cursor+1] != ']' {
			end, afterRange, err := parsePatternClassRune(pattern, cursor+1)
			if err != nil {
				return patternAtom{}, cursor, err
			}
			if end < start {
				return patternAtom{}, cursor, fmt.Errorf("invalid character range")
			}
			atom.ranges = append(atom.ranges, [2]rune{start, end})
			cursor = afterRange
			continue
		}
		atom.ranges = append(atom.ranges, [2]rune{start, start})
	}
	if cursor >= len(pattern) || pattern[cursor] != ']' {
		return patternAtom{}, cursor, fmt.Errorf("unterminated character class")
	}
	return atom, cursor + 1, nil
}

func parsePatternClassRune(pattern string, cursor int) (rune, int, error) {
	if cursor >= len(pattern) {
		return 0, cursor, fmt.Errorf("unterminated character class")
	}
	if pattern[cursor] == '\\' {
		if cursor+1 >= len(pattern) {
			return 0, cursor, fmt.Errorf("dangling escape")
		}
		return rune(pattern[cursor+1]), cursor + 2, nil
	}
	r, size := nextPatternRune(pattern[cursor:])
	return r, cursor + size, nil
}

func parsePatternQuantifier(pattern string, cursor int) (int, int, int, bool, error) {
	switch pattern[cursor] {
	case '*':
		return 0, -1, cursor + 1, true, nil
	case '+':
		return 1, -1, cursor + 1, true, nil
	case '?':
		return 0, 1, cursor + 1, true, nil
	case '{':
		end := strings.IndexByte(pattern[cursor:], '}')
		if end < 0 {
			return 0, 0, cursor, false, fmt.Errorf("unterminated quantifier")
		}
		end += cursor
		min, max, err := parsePatternCountRange(pattern[cursor+1 : end])
		if err != nil {
			return 0, 0, cursor, false, err
		}
		return min, max, end + 1, true, nil
	default:
		return 0, 0, cursor, false, nil
	}
}

func parsePatternCountRange(body string) (int, int, error) {
	if body == "" {
		return 0, 0, fmt.Errorf("empty quantifier")
	}
	if !strings.Contains(body, ",") {
		count, err := strconv.Atoi(body)
		if err != nil || count < 0 {
			return 0, 0, fmt.Errorf("invalid quantifier")
		}
		return count, count, nil
	}
	left, right, _ := strings.Cut(body, ",")
	min, err := strconv.Atoi(left)
	if err != nil || min < 0 {
		return 0, 0, fmt.Errorf("invalid quantifier")
	}
	if right == "" {
		return min, -1, nil
	}
	max, err := strconv.Atoi(right)
	if err != nil || max < min {
		return 0, 0, fmt.Errorf("invalid quantifier")
	}
	return min, max, nil
}

func (atom patternAtom) matches(value rune) bool {
	switch atom.kind {
	case patternAny:
		return value != '\n' && value != '\r'
	case patternClass:
		contains := false
		for _, item := range atom.ranges {
			if value >= item[0] && value <= item[1] {
				contains = true
				break
			}
		}
		if atom.negate {
			return !contains
		}
		return contains
	default:
		return value == atom.lit
	}
}

func nextPatternRune(value string) (rune, int) {
	for index, r := range value {
		if index == 0 {
			return r, len(string(r))
		}
	}
	return 0, 0
}
