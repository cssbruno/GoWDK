package buildgen

import (
	"fmt"
	"go/parser"
	"strings"
)

// Build-time evaluation is pure and deterministic, so the only way an
// expression can fail to terminate is unbounded iteration or runaway nesting.
// These limits bound both: the element budget caps how many list elements a
// single build {} block may produce (across seq, comprehensions, and literals),
// and the depth limit caps how deeply value expressions may nest. Both surface a
// diagnostic rather than hanging the build.
const (
	buildIterationBudget = 50000
	buildValueMaxDepth   = 64
)

// buildEnv is the evaluation environment for one build {} block. data holds the
// build fields resolved so far (later fields may reference earlier ones); scope
// holds comprehension loop variables; budget is the shared, decreasing element
// budget; depth guards against runaway nesting. A build {} block cannot form a
// reference cycle because fields resolve in source order and loop variables are
// lexically scoped, so no cycle diagnostic is required — only the budget/depth
// limits below.
type buildEnv struct {
	routeParams map[string]string
	data        map[string]buildValue
	scope       map[string]buildValue
	budget      *int
	depth       int
}

func newBuildEnv(routeParams map[string]string, data map[string]buildValue) *buildEnv {
	budget := buildIterationBudget
	return &buildEnv{routeParams: routeParams, data: data, budget: &budget}
}

// child returns a nested environment with loop-variable bindings isolated from
// the parent, so sibling comprehension iterations never see each other's vars.
func (env *buildEnv) child() *buildEnv {
	scope := make(map[string]buildValue, len(env.scope)+2)
	for key, value := range env.scope {
		scope[key] = value
	}
	return &buildEnv{routeParams: env.routeParams, data: env.data, scope: scope, budget: env.budget, depth: env.depth + 1}
}

// deeper returns a nested environment that shares the scope; used for nested
// values (list elements, object fields, builtin arguments) that introduce no new
// loop variable.
func (env *buildEnv) deeper() *buildEnv {
	return &buildEnv{routeParams: env.routeParams, data: env.data, scope: env.scope, budget: env.budget, depth: env.depth + 1}
}

func (env *buildEnv) consume(count int) error {
	if env.budget == nil {
		return nil
	}
	*env.budget -= count
	if *env.budget < 0 {
		return fmt.Errorf("build-time iteration exceeded the limit of %d elements", buildIterationBudget)
	}
	return nil
}

// interpolationValues exposes the string forms of fields and loop variables for
// "{name}" interpolation inside string literals.
func (env *buildEnv) interpolationValues() map[string]string {
	out := make(map[string]string, len(env.data)+len(env.scope))
	for key, value := range env.data {
		out[key] = value.text
	}
	for key, value := range env.scope {
		out[key] = value.text
	}
	return out
}

// buildEvalExprString evaluates one build value expression from its source form.
// Comprehensions and list/object literals are not valid Go expressions, so they
// are recognized here before falling back to the Go expression parser for the
// existing scalar/expression subset.
func buildEvalExprString(exprStr string, env *buildEnv) (buildValue, error) {
	source := strings.TrimSpace(exprStr)
	if source == "" {
		return buildValue{}, fmt.Errorf("value must not be empty")
	}
	if env.depth > buildValueMaxDepth {
		return buildValue{}, fmt.Errorf("build expression nested too deeply (limit %d)", buildValueMaxDepth)
	}
	if strings.HasPrefix(source, "[") && strings.HasSuffix(source, "]") {
		inner := source[1 : len(source)-1]
		if indexTopLevelWord(inner, "for") >= 0 {
			return buildEvalComprehension(inner, env)
		}
		return buildEvalListLiteral(inner, env)
	}
	if strings.HasPrefix(source, "{") && strings.HasSuffix(source, "}") {
		return buildEvalObjectLiteral(source[1:len(source)-1], env)
	}
	expr, err := parser.ParseExpr(source)
	if err != nil {
		return buildValue{}, fmt.Errorf("invalid build expression %q: %w", source, err)
	}
	return buildEvalAST(expr, env)
}

// buildEvalComprehension evaluates "TEMPLATE for VAR[, INDEX] in SOURCE [if COND]"
// (the text already stripped of its enclosing brackets). The source must be a
// list; the template and optional filter are evaluated once per element with VAR
// (and the optional zero-based INDEX) bound in a child scope.
func buildEvalComprehension(inner string, env *buildEnv) (buildValue, error) {
	forIndex := indexTopLevelWord(inner, "for")
	if forIndex < 0 {
		return buildValue{}, fmt.Errorf("comprehension must use [expr for v in source]")
	}
	template := strings.TrimSpace(inner[:forIndex])
	rest := strings.TrimSpace(inner[forIndex+len("for"):])
	inIndex := indexTopLevelWord(rest, "in")
	if inIndex < 0 {
		return buildValue{}, fmt.Errorf("comprehension must use [expr for v in source]")
	}
	itemVar, indexVar, err := parseComprehensionVars(strings.TrimSpace(rest[:inIndex]))
	if err != nil {
		return buildValue{}, err
	}
	remainder := strings.TrimSpace(rest[inIndex+len("in"):])
	sourceStr := remainder
	condStr := ""
	if ifIndex := indexTopLevelWord(remainder, "if"); ifIndex >= 0 {
		sourceStr = strings.TrimSpace(remainder[:ifIndex])
		condStr = strings.TrimSpace(remainder[ifIndex+len("if"):])
	}
	if template == "" {
		return buildValue{}, fmt.Errorf("comprehension is missing its element expression")
	}
	if sourceStr == "" {
		return buildValue{}, fmt.Errorf("comprehension is missing its source expression")
	}
	source, err := buildEvalExprString(sourceStr, env.deeper())
	if err != nil {
		return buildValue{}, err
	}
	if source.kind != buildValueList {
		return buildValue{}, fmt.Errorf("comprehension source must be a list")
	}
	items := make([]buildValue, 0, len(source.items))
	for index, element := range source.items {
		if err := env.consume(1); err != nil {
			return buildValue{}, err
		}
		scope := env.child()
		scope.scope[itemVar] = element
		if indexVar != "" {
			scope.scope[indexVar] = buildNumberValue(float64(index))
		}
		if condStr != "" {
			keep, err := buildEvalExprString(condStr, scope)
			if err != nil {
				return buildValue{}, err
			}
			if keep.kind != buildValueBool {
				return buildValue{}, fmt.Errorf("comprehension filter must be a boolean")
			}
			if !keep.boolean {
				continue
			}
		}
		mapped, err := buildEvalExprString(template, scope)
		if err != nil {
			return buildValue{}, err
		}
		items = append(items, mapped)
	}
	return buildListValue(items), nil
}

func parseComprehensionVars(spec string) (itemVar string, indexVar string, err error) {
	parts := splitTopLevel(spec, ',')
	switch len(parts) {
	case 1:
		itemVar = strings.TrimSpace(parts[0])
	case 2:
		itemVar = strings.TrimSpace(parts[0])
		indexVar = strings.TrimSpace(parts[1])
	default:
		return "", "", fmt.Errorf("comprehension binds at most an item and index variable")
	}
	if !isLiteralName(itemVar) {
		return "", "", fmt.Errorf("invalid comprehension variable %q", itemVar)
	}
	if indexVar != "" {
		if !isLiteralName(indexVar) {
			return "", "", fmt.Errorf("invalid comprehension index variable %q", indexVar)
		}
		if indexVar == itemVar {
			return "", "", fmt.Errorf("comprehension item and index variables must differ")
		}
	}
	return itemVar, indexVar, nil
}

func buildEvalListLiteral(inner string, env *buildEnv) (buildValue, error) {
	elements, err := splitLiteralElements(inner)
	if err != nil {
		return buildValue{}, fmt.Errorf("list literal: %w", err)
	}
	items := make([]buildValue, 0, len(elements))
	for _, element := range elements {
		if err := env.consume(1); err != nil {
			return buildValue{}, err
		}
		value, err := buildEvalExprString(element, env.deeper())
		if err != nil {
			return buildValue{}, err
		}
		items = append(items, value)
	}
	return buildListValue(items), nil
}

func buildEvalObjectLiteral(inner string, env *buildEnv) (buildValue, error) {
	elements, err := splitLiteralElements(inner)
	if err != nil {
		return buildValue{}, fmt.Errorf("object literal: %w", err)
	}
	order := make([]string, 0, len(elements))
	fields := make(map[string]buildValue, len(elements))
	for _, element := range elements {
		colon := indexTopLevelByte(element, ':')
		if colon < 0 {
			return buildValue{}, fmt.Errorf("object field must use name: value")
		}
		name := strings.TrimSpace(element[:colon])
		valueStr := strings.TrimSpace(element[colon+1:])
		if !isLiteralName(name) {
			return buildValue{}, fmt.Errorf("invalid object field name %q", name)
		}
		if _, exists := fields[name]; exists {
			return buildValue{}, fmt.Errorf("duplicate object field %q", name)
		}
		if valueStr == "" {
			return buildValue{}, fmt.Errorf("object field %s: value must not be empty", name)
		}
		if err := env.consume(1); err != nil {
			return buildValue{}, err
		}
		value, err := buildEvalExprString(valueStr, env.deeper())
		if err != nil {
			return buildValue{}, err
		}
		order = append(order, name)
		fields[name] = value
	}
	return buildObjectValue(order, fields), nil
}

// splitLiteralElements splits a comma-separated list/object body at the top
// level, tolerating a single trailing comma and rejecting empty interior
// elements.
func splitLiteralElements(inner string) ([]string, error) {
	if strings.TrimSpace(inner) == "" {
		return nil, nil
	}
	parts := splitTopLevel(inner, ',')
	if last := len(parts) - 1; last >= 0 && strings.TrimSpace(parts[last]) == "" {
		parts = parts[:last]
	}
	elements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty element")
		}
		elements = append(elements, trimmed)
	}
	return elements, nil
}

// indexTopLevelByte returns the index of the first occurrence of target at
// bracket depth zero and outside a string literal, or -1.
func indexTopLevelByte(source string, target byte) int {
	scan := newExprScanner()
	for index := 0; index < len(source); index++ {
		if scan.step(source[index]) && source[index] == target {
			return index
		}
	}
	return -1
}

// splitTopLevel splits source on sep at bracket depth zero and outside string
// literals.
func splitTopLevel(source string, sep byte) []string {
	var parts []string
	scan := newExprScanner()
	start := 0
	for index := 0; index < len(source); index++ {
		if scan.step(source[index]) && source[index] == sep {
			parts = append(parts, source[start:index])
			start = index + 1
		}
	}
	return append(parts, source[start:])
}

// indexTopLevelWord returns the index of word when it appears space-delimited at
// bracket depth zero and outside a string literal, or -1. The grammar always
// places an expression before for/in/if, so a required leading space safely
// distinguishes the keyword from an identifier substring.
func indexTopLevelWord(source string, word string) int {
	scan := newExprScanner()
	needle := word + " "
	for index := 0; index < len(source); index++ {
		atTop := scan.step(source[index])
		if !atTop || source[index] != ' ' {
			continue
		}
		if strings.HasPrefix(source[index+1:], needle) {
			return index + 1
		}
	}
	return -1
}

// exprScanner tracks bracket depth and string-literal state so the top-level
// scanners ignore separators nested inside parentheses, brackets, braces, or
// string literals.
type exprScanner struct {
	depth    int
	inString bool
	escaped  bool
}

func newExprScanner() *exprScanner {
	return &exprScanner{}
}

// step consumes one byte and reports whether it sits at bracket depth zero
// outside a string literal (so the caller may treat it as a separator/keyword).
func (s *exprScanner) step(char byte) bool {
	if s.escaped {
		s.escaped = false
		return false
	}
	if s.inString {
		switch char {
		case '\\':
			s.escaped = true
		case '"':
			s.inString = false
		}
		return false
	}
	switch char {
	case '"':
		s.inString = true
		return false
	case '(', '[', '{':
		s.depth++
		return false
	case ')', ']', '}':
		if s.depth > 0 {
			s.depth--
		}
		return false
	}
	return s.depth == 0
}
