package validation

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type patternNode struct {
	kind     patternNodeKind
	children []patternNode
	atom     patternAtom
	min      int
	max      int
}

type patternNodeKind int

const (
	patternSequence patternNodeKind = iota
	patternChoice
	patternRepeat
	patternAtomNode
)

type patternAtom struct {
	kind   patternAtomKind
	lit    rune
	ranges [][2]rune
	negate bool
}

type patternAtomKind int

const (
	patternLiteral patternAtomKind = iota
	patternAny
	patternClass
)

type patternParser struct {
	source []rune
	cursor int
}

// ValidatePattern reports whether pattern uses the generated action validation
// syntax supported by MatchPattern.
func ValidatePattern(pattern string) error {
	_, err := parsePattern(pattern)
	return err
}

// MatchPattern reports whether value matches pattern from start to end.
func MatchPattern(pattern string, value string) (bool, error) {
	root, err := parsePattern(pattern)
	if err != nil {
		return false, err
	}
	runes := []rune(value)
	ends := root.match(runes, 0)
	return slices.Contains(ends, len(runes)), nil
}

func parsePattern(pattern string) (patternNode, error) {
	source := []rune(pattern)
	if len(source) > 0 && source[0] == '^' {
		source = source[1:]
	}
	if len(source) > 0 && source[len(source)-1] == '$' && !patternRuneEscaped(source, len(source)-1) {
		source = source[:len(source)-1]
	}
	parser := patternParser{source: source}
	root, err := parser.parseChoice()
	if err != nil {
		return patternNode{}, err
	}
	if parser.cursor != len(parser.source) {
		return patternNode{}, fmt.Errorf("unexpected pattern token %q", parser.peek())
	}
	return root, nil
}

func patternRuneEscaped(source []rune, index int) bool {
	backslashes := 0
	for cursor := index - 1; cursor >= 0 && source[cursor] == '\\'; cursor-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func (parser *patternParser) parseChoice() (patternNode, error) {
	var choices []patternNode
	for {
		sequence, err := parser.parseSequence()
		if err != nil {
			return patternNode{}, err
		}
		choices = append(choices, sequence)
		if parser.peek() != '|' {
			break
		}
		parser.cursor++
	}
	if len(choices) == 1 {
		return choices[0], nil
	}
	return patternNode{kind: patternChoice, children: choices}, nil
}

func (parser *patternParser) parseSequence() (patternNode, error) {
	var nodes []patternNode
	for parser.cursor < len(parser.source) {
		switch parser.peek() {
		case '|', ')':
			return patternNode{kind: patternSequence, children: nodes}, nil
		}
		node, err := parser.parseRepeat()
		if err != nil {
			return patternNode{}, err
		}
		nodes = append(nodes, node)
	}
	return patternNode{kind: patternSequence, children: nodes}, nil
}

func (parser *patternParser) parseRepeat() (patternNode, error) {
	child, err := parser.parsePrimary()
	if err != nil {
		return patternNode{}, err
	}
	min, max, ok, err := parser.parseQuantifier()
	if err != nil {
		return patternNode{}, err
	}
	if !ok {
		return child, nil
	}
	return patternNode{kind: patternRepeat, children: []patternNode{child}, min: min, max: max}, nil
}

func (parser *patternParser) parsePrimary() (patternNode, error) {
	switch parser.peek() {
	case 0:
		return patternNode{}, fmt.Errorf("unexpected end of pattern")
	case '.':
		parser.cursor++
		return patternNode{kind: patternAtomNode, atom: patternAtom{kind: patternAny}}, nil
	case '[':
		atom, err := parser.parseClass()
		if err != nil {
			return patternNode{}, err
		}
		return patternNode{kind: patternAtomNode, atom: atom}, nil
	case '(':
		return parser.parseGroup()
	case '\\':
		atom, err := parser.parseEscape()
		if err != nil {
			return patternNode{}, err
		}
		return patternNode{kind: patternAtomNode, atom: atom}, nil
	case '*', '+', '?', '{', '}':
		return patternNode{}, fmt.Errorf("quantifier %q has no target", parser.peek())
	default:
		char := parser.peek()
		parser.cursor++
		return patternNode{kind: patternAtomNode, atom: patternAtom{kind: patternLiteral, lit: char}}, nil
	}
}

func (parser *patternParser) parseGroup() (patternNode, error) {
	parser.cursor++
	if parser.peek() == '?' {
		parser.cursor++
		if parser.peek() != ':' {
			return patternNode{}, fmt.Errorf("unsupported group operator")
		}
		parser.cursor++
	}
	node, err := parser.parseChoice()
	if err != nil {
		return patternNode{}, err
	}
	if parser.peek() != ')' {
		return patternNode{}, fmt.Errorf("unterminated group")
	}
	parser.cursor++
	return node, nil
}

func (parser *patternParser) parseClass() (patternAtom, error) {
	parser.cursor++
	atom := patternAtom{kind: patternClass}
	if parser.peek() == '^' {
		atom.negate = true
		parser.cursor++
	}
	if parser.peek() == 0 || parser.peek() == ']' {
		return patternAtom{}, fmt.Errorf("empty character class")
	}
	for parser.cursor < len(parser.source) && parser.peek() != ']' {
		ranges, err := parser.parseClassItem()
		if err != nil {
			return patternAtom{}, err
		}
		if len(ranges) == 1 && ranges[0][0] == ranges[0][1] && parser.peek() == '-' && parser.peekNext() != ']' && parser.peekNext() != 0 {
			parser.cursor++
			endRanges, err := parser.parseClassItem()
			if err != nil {
				return patternAtom{}, err
			}
			if len(endRanges) != 1 || endRanges[0][0] != endRanges[0][1] {
				return patternAtom{}, fmt.Errorf("invalid character range")
			}
			start := ranges[0][0]
			end := endRanges[0][0]
			if end < start {
				return patternAtom{}, fmt.Errorf("invalid character range")
			}
			atom.ranges = append(atom.ranges, [2]rune{start, end})
			continue
		}
		atom.ranges = append(atom.ranges, ranges...)
	}
	if parser.peek() != ']' {
		return patternAtom{}, fmt.Errorf("unterminated character class")
	}
	parser.cursor++
	return atom, nil
}

func (parser *patternParser) parseClassItem() ([][2]rune, error) {
	if parser.peek() == 0 {
		return nil, fmt.Errorf("unterminated character class")
	}
	if parser.peek() == '\\' {
		parser.cursor++
		if parser.peek() == 0 {
			return nil, fmt.Errorf("dangling escape")
		}
		char := parser.peek()
		parser.cursor++
		switch char {
		case 'd':
			return [][2]rune{{'0', '9'}}, nil
		case 'w':
			return [][2]rune{{'A', 'Z'}, {'a', 'z'}, {'0', '9'}, {'_', '_'}}, nil
		case 's':
			return [][2]rune{{' ', ' '}, {'\t', '\t'}, {'\n', '\n'}, {'\r', '\r'}, {'\f', '\f'}}, nil
		case 'D', 'W', 'S':
			return nil, fmt.Errorf("negated shorthand escapes are not supported inside character classes")
		default:
			return [][2]rune{{char, char}}, nil
		}
	}
	char := parser.peek()
	parser.cursor++
	return [][2]rune{{char, char}}, nil
}

func (parser *patternParser) parseEscape() (patternAtom, error) {
	parser.cursor++
	if parser.peek() == 0 {
		return patternAtom{}, fmt.Errorf("dangling escape")
	}
	char := parser.peek()
	parser.cursor++
	switch char {
	case 'd':
		return patternAtom{kind: patternClass, ranges: [][2]rune{{'0', '9'}}}, nil
	case 'D':
		return patternAtom{kind: patternClass, ranges: [][2]rune{{'0', '9'}}, negate: true}, nil
	case 'w':
		return wordPatternAtom(false), nil
	case 'W':
		return wordPatternAtom(true), nil
	case 's':
		return spacePatternAtom(false), nil
	case 'S':
		return spacePatternAtom(true), nil
	default:
		return patternAtom{kind: patternLiteral, lit: char}, nil
	}
}

func (parser *patternParser) parseQuantifier() (int, int, bool, error) {
	switch parser.peek() {
	case '*':
		parser.cursor++
		return 0, -1, true, nil
	case '+':
		parser.cursor++
		return 1, -1, true, nil
	case '?':
		parser.cursor++
		return 0, 1, true, nil
	case '{':
		return parser.parseCountRange()
	default:
		return 0, 0, false, nil
	}
}

func (parser *patternParser) parseCountRange() (int, int, bool, error) {
	parser.cursor++
	start := parser.cursor
	for parser.cursor < len(parser.source) && parser.peek() != '}' {
		parser.cursor++
	}
	if parser.peek() != '}' {
		return 0, 0, false, fmt.Errorf("unterminated quantifier")
	}
	body := string(parser.source[start:parser.cursor])
	parser.cursor++
	min, max, err := parsePatternCountRange(body)
	return min, max, true, err
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

func (parser patternParser) peek() rune {
	if parser.cursor >= len(parser.source) {
		return 0
	}
	return parser.source[parser.cursor]
}

func (parser patternParser) peekNext() rune {
	if parser.cursor+1 >= len(parser.source) {
		return 0
	}
	return parser.source[parser.cursor+1]
}

func (node patternNode) match(value []rune, offset int) []int {
	switch node.kind {
	case patternChoice:
		var ends []int
		for _, child := range node.children {
			ends = appendUniqueInts(ends, child.match(value, offset)...)
		}
		return ends
	case patternRepeat:
		return node.matchRepeat(value, offset)
	case patternAtomNode:
		if offset < len(value) && node.atom.matches(value[offset]) {
			return []int{offset + 1}
		}
		return nil
	default:
		positions := []int{offset}
		for _, child := range node.children {
			var next []int
			for _, position := range positions {
				next = appendUniqueInts(next, child.match(value, position)...)
			}
			if len(next) == 0 {
				return nil
			}
			positions = next
		}
		return positions
	}
}

func (node patternNode) matchRepeat(value []rune, offset int) []int {
	child := node.children[0]
	positions := []int{offset}
	matchedByCount := map[int][]int{0: positions}
	limit := node.max
	if limit < 0 || limit > len(value)-offset {
		limit = len(value) - offset
	}
	for count := 1; count <= limit; count++ {
		var next []int
		for _, position := range positions {
			for _, end := range child.match(value, position) {
				if end != position {
					next = appendUniqueInts(next, end)
				}
			}
		}
		if len(next) == 0 {
			break
		}
		matchedByCount[count] = next
		positions = next
	}
	var ends []int
	for count, positions := range matchedByCount {
		if count >= node.min {
			ends = appendUniqueInts(ends, positions...)
		}
	}
	slices.Sort(ends)
	slices.Reverse(ends)
	return ends
}

func appendUniqueInts(values []int, next ...int) []int {
	for _, value := range next {
		if !slices.Contains(values, value) {
			values = append(values, value)
		}
	}
	return values
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

func wordPatternAtom(negate bool) patternAtom {
	return patternAtom{kind: patternClass, negate: negate, ranges: [][2]rune{{'A', 'Z'}, {'a', 'z'}, {'0', '9'}, {'_', '_'}}}
}

func spacePatternAtom(negate bool) patternAtom {
	return patternAtom{kind: patternClass, negate: negate, ranges: [][2]rune{{' ', ' '}, {'\t', '\t'}, {'\n', '\n'}, {'\r', '\r'}, {'\f', '\f'}}}
}
