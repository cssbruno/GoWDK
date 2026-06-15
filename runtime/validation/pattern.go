package validation

import (
	"fmt"
	"strconv"
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
	return compiled.match([]rune(value)), nil
}

func compilePattern(pattern string) (*compiledPattern, error) {
	source := []rune(pattern)
	if len(source) > 0 && source[0] == '^' {
		source = source[1:]
	}
	if len(source) > 0 && source[len(source)-1] == '$' && !patternRuneEscaped(source, len(source)-1) {
		source = source[:len(source)-1]
	}
	parser := patternParser{source: source}
	root, err := parser.parseExpression(0)
	if err != nil {
		return nil, err
	}
	if parser.index != len(source) {
		return nil, fmt.Errorf("unexpected pattern token %q", source[parser.index])
	}
	return &compiledPattern{root: root}, nil
}

type compiledPattern struct {
	root patternNode
}

func (pattern *compiledPattern) match(value []rune) bool {
	for _, position := range pattern.root.match(value, 0) {
		if position == len(value) {
			return true
		}
	}
	return false
}

type patternNode interface {
	match(value []rune, position int) []int
}

type emptyNode struct{}

func (emptyNode) match(_ []rune, position int) []int {
	return []int{position}
}

type literalNode rune

func (node literalNode) match(value []rune, position int) []int {
	if position < len(value) && value[position] == rune(node) {
		return []int{position + 1}
	}
	return nil
}

type anyNode struct{}

func (anyNode) match(value []rune, position int) []int {
	if position < len(value) {
		return []int{position + 1}
	}
	return nil
}

type shorthandNode rune

func (node shorthandNode) match(value []rune, position int) []int {
	if position >= len(value) {
		return nil
	}
	matched := matchShorthand(rune(node), value[position])
	if matched {
		return []int{position + 1}
	}
	return nil
}

type sequenceNode []patternNode

func (node sequenceNode) match(value []rune, position int) []int {
	positions := []int{position}
	for _, child := range node {
		var next []int
		for _, current := range positions {
			next = append(next, child.match(value, current)...)
		}
		positions = uniquePositions(next)
		if len(positions) == 0 {
			return nil
		}
	}
	return positions
}

type alternationNode []patternNode

func (node alternationNode) match(value []rune, position int) []int {
	var positions []int
	for _, child := range node {
		positions = append(positions, child.match(value, position)...)
	}
	return uniquePositions(positions)
}

type repeatNode struct {
	child patternNode
	min   int
	max   int
}

func (node repeatNode) match(value []rune, position int) []int {
	limit := node.max
	if limit < 0 || limit > len(value)-position+node.min {
		limit = len(value) - position + node.min
	}
	positionsByCount := [][]int{{position}}
	var accepted []int
	for count := 0; count <= limit; count++ {
		current := positionsByCount[count]
		if count >= node.min {
			accepted = append(accepted, current...)
		}
		if count == limit {
			break
		}
		var next []int
		for _, currentPosition := range current {
			for _, nextPosition := range node.child.match(value, currentPosition) {
				if nextPosition != currentPosition {
					next = append(next, nextPosition)
				}
			}
		}
		next = uniquePositions(next)
		if len(next) == 0 {
			break
		}
		positionsByCount = append(positionsByCount, next)
	}
	return uniquePositions(accepted)
}

type classNode struct {
	negated bool
	parts   []classPart
}

func (node classNode) match(value []rune, position int) []int {
	if position >= len(value) {
		return nil
	}
	matched := false
	for _, part := range node.parts {
		if part.match(value[position]) {
			matched = true
			break
		}
	}
	if node.negated {
		matched = !matched
	}
	if matched {
		return []int{position + 1}
	}
	return nil
}

type classPart interface {
	match(r rune) bool
}

type literalClassPart rune

func (part literalClassPart) match(r rune) bool {
	return r == rune(part)
}

type rangeClassPart struct {
	first rune
	last  rune
}

func (part rangeClassPart) match(r rune) bool {
	return r >= part.first && r <= part.last
}

type shorthandClassPart rune

func (part shorthandClassPart) match(r rune) bool {
	return matchShorthand(rune(part), r)
}

type patternParser struct {
	source []rune
	index  int
}

func (parser *patternParser) parseExpression(stop rune) (patternNode, error) {
	var alternatives []patternNode
	for {
		sequence, err := parser.parseSequence(stop)
		if err != nil {
			return nil, err
		}
		alternatives = append(alternatives, sequence)
		if parser.index >= len(parser.source) || parser.source[parser.index] != '|' {
			break
		}
		parser.index++
	}
	if len(alternatives) == 1 {
		return alternatives[0], nil
	}
	return alternationNode(alternatives), nil
}

func (parser *patternParser) parseSequence(stop rune) (patternNode, error) {
	var nodes []patternNode
	for parser.index < len(parser.source) {
		current := parser.source[parser.index]
		if current == '|' || (stop != 0 && current == stop) {
			break
		}
		if current == ')' && stop == 0 {
			return nil, fmt.Errorf("unexpected group terminator")
		}
		if current == '*' || current == '+' || current == '?' || current == '}' {
			return nil, fmt.Errorf("quantifier %q has no target", current)
		}
		atom, err := parser.parseAtom()
		if err != nil {
			return nil, err
		}
		atom, err = parser.parseQuantifier(atom)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, atom)
	}
	if len(nodes) == 0 {
		return emptyNode{}, nil
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return sequenceNode(nodes), nil
}

func (parser *patternParser) parseAtom() (patternNode, error) {
	if parser.index >= len(parser.source) {
		return nil, fmt.Errorf("unexpected end of pattern")
	}
	current := parser.source[parser.index]
	parser.index++
	switch current {
	case '\\':
		return parser.parseEscape()
	case '.':
		return anyNode{}, nil
	case '[':
		return parser.parseClass()
	case '(':
		return parser.parseGroup()
	case '^', '$':
		return literalNode(current), nil
	default:
		return literalNode(current), nil
	}
}

func (parser *patternParser) parseEscape() (patternNode, error) {
	if parser.index >= len(parser.source) {
		return literalNode('\\'), nil
	}
	current := parser.source[parser.index]
	parser.index++
	switch current {
	case 'd', 'D', 'w', 'W', 's', 'S':
		return shorthandNode(current), nil
	default:
		return literalNode(current), nil
	}
}

func (parser *patternParser) parseClass() (patternNode, error) {
	class := classNode{}
	if parser.index < len(parser.source) && parser.source[parser.index] == '^' {
		class.negated = true
		parser.index++
	}
	for parser.index < len(parser.source) {
		if parser.source[parser.index] == ']' && len(class.parts) > 0 {
			parser.index++
			return class, nil
		}
		first, firstLiteral, err := parser.parseClassPart()
		if err != nil {
			return nil, err
		}
		if firstLiteral && parser.index+1 < len(parser.source) && parser.source[parser.index] == '-' && parser.source[parser.index+1] != ']' {
			parser.index++
			second, secondLiteral, err := parser.parseClassPart()
			if err != nil {
				return nil, err
			}
			if !secondLiteral {
				return nil, fmt.Errorf("character class range must use literal endpoints")
			}
			firstRune := rune(first.(literalClassPart))
			secondRune := rune(second.(literalClassPart))
			if secondRune < firstRune {
				return nil, fmt.Errorf("invalid character class range")
			}
			class.parts = append(class.parts, rangeClassPart{first: firstRune, last: secondRune})
			continue
		}
		class.parts = append(class.parts, first)
	}
	return nil, fmt.Errorf("unterminated character class")
}

func (parser *patternParser) parseClassPart() (classPart, bool, error) {
	if parser.index >= len(parser.source) {
		return nil, false, fmt.Errorf("unterminated character class")
	}
	current := parser.source[parser.index]
	parser.index++
	if current != '\\' {
		return literalClassPart(current), true, nil
	}
	if parser.index >= len(parser.source) {
		return literalClassPart('\\'), true, nil
	}
	escaped := parser.source[parser.index]
	parser.index++
	switch escaped {
	case 'D', 'W', 'S':
		return nil, false, fmt.Errorf("negated shorthand escapes are not supported inside character classes")
	case 'd', 'w', 's':
		return shorthandClassPart(escaped), false, nil
	default:
		return literalClassPart(escaped), true, nil
	}
}

func (parser *patternParser) parseGroup() (patternNode, error) {
	if parser.index < len(parser.source) && parser.source[parser.index] == '?' {
		parser.index++
		if parser.index >= len(parser.source) || parser.source[parser.index] != ':' {
			return nil, fmt.Errorf("unsupported group operator")
		}
		parser.index++
	}
	group, err := parser.parseExpression(')')
	if err != nil {
		return nil, err
	}
	if parser.index >= len(parser.source) || parser.source[parser.index] != ')' {
		return nil, fmt.Errorf("unterminated group")
	}
	parser.index++
	return group, nil
}

func (parser *patternParser) parseQuantifier(atom patternNode) (patternNode, error) {
	if parser.index >= len(parser.source) {
		return atom, nil
	}
	min, max, ok, err := parser.readQuantifier()
	if err != nil || !ok {
		return atom, err
	}
	if parser.index < len(parser.source) && parser.source[parser.index] == '?' {
		return nil, fmt.Errorf("quantifier %q has no target", parser.source[parser.index])
	}
	return repeatNode{child: atom, min: min, max: max}, nil
}

func (parser *patternParser) readQuantifier() (int, int, bool, error) {
	switch parser.source[parser.index] {
	case '*':
		parser.index++
		return 0, -1, true, nil
	case '+':
		parser.index++
		return 1, -1, true, nil
	case '?':
		parser.index++
		return 0, 1, true, nil
	case '{':
		return parser.readBraceQuantifier()
	default:
		return 0, 0, false, nil
	}
}

func (parser *patternParser) readBraceQuantifier() (int, int, bool, error) {
	start := parser.index
	index := parser.index + 1
	minStart := index
	for index < len(parser.source) && isASCIIDigit(parser.source[index]) {
		index++
	}
	if index == minStart {
		return 0, 0, false, nil
	}
	min, err := strconv.Atoi(string(parser.source[minStart:index]))
	if err != nil {
		return 0, 0, false, err
	}
	max := min
	if index < len(parser.source) && parser.source[index] == ',' {
		index++
		maxStart := index
		for index < len(parser.source) && isASCIIDigit(parser.source[index]) {
			index++
		}
		if index == maxStart {
			max = -1
		} else {
			max, err = strconv.Atoi(string(parser.source[maxStart:index]))
			if err != nil {
				return 0, 0, false, err
			}
		}
	}
	if index >= len(parser.source) || parser.source[index] != '}' {
		parser.index = start
		return 0, 0, false, nil
	}
	if max >= 0 && max < min {
		return 0, 0, false, fmt.Errorf("invalid repeat count")
	}
	parser.index = index + 1
	return min, max, true, nil
}

func matchShorthand(kind rune, r rune) bool {
	switch kind {
	case 'd':
		return r >= '0' && r <= '9'
	case 'D':
		return !(r >= '0' && r <= '9')
	case 'w':
		return r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
	case 'W':
		return !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	case 's':
		return r == '\t' || r == '\n' || r == '\f' || r == '\r' || r == ' '
	case 'S':
		return !(r == '\t' || r == '\n' || r == '\f' || r == '\r' || r == ' ')
	default:
		return false
	}
}

func uniquePositions(positions []int) []int {
	if len(positions) < 2 {
		return positions
	}
	seen := map[int]bool{}
	var unique []int
	for _, position := range positions {
		if seen[position] {
			continue
		}
		seen[position] = true
		unique = append(unique, position)
	}
	return unique
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func patternRuneEscaped(source []rune, index int) bool {
	backslashes := 0
	for cursor := index - 1; cursor >= 0 && source[cursor] == '\\'; cursor-- {
		backslashes++
	}
	return backslashes%2 == 1
}
