package buildgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
)

type buildCallRef struct {
	Alias    string
	Function string
}

type buildValueKind int

const (
	buildValueString buildValueKind = iota
	buildValueNumber
	buildValueBool
	buildValueNil
	buildValueList
	buildValueObject
)

// buildValue is one evaluated build-time value. Scalars carry their canonical
// string form in text (used directly as interpolation data); list and object
// values carry their items/fields and serialize text to canonical JSON so they
// flow through the map[string]string build-data contract unchanged.
type buildValue struct {
	kind    buildValueKind
	text    string
	number  float64
	boolean bool
	items   []buildValue
	fields  map[string]buildValue
	order   []string
}

func buildStringValue(value string) buildValue {
	return buildValue{kind: buildValueString, text: value}
}

func buildNumberValue(value float64) buildValue {
	return buildValue{kind: buildValueNumber, text: strconv.FormatFloat(value, 'f', -1, 64), number: value}
}

func buildBoolValue(value bool) buildValue {
	return buildValue{kind: buildValueBool, text: strconv.FormatBool(value), boolean: value}
}

func buildNilValue() buildValue {
	return buildValue{kind: buildValueNil}
}

func buildListValue(items []buildValue) buildValue {
	value := buildValue{kind: buildValueList, items: items}
	value.text = value.jsonText()
	return value
}

func buildObjectValue(order []string, fields map[string]buildValue) buildValue {
	value := buildValue{kind: buildValueObject, order: order, fields: fields}
	value.text = value.jsonText()
	return value
}

// jsonText renders a build value as canonical, deterministic JSON. Object keys
// follow the declared field order and list items follow their declared order, so
// re-running the build over the same inputs always produces byte-identical data.
func (v buildValue) jsonText() string {
	switch v.kind {
	case buildValueString:
		return strconv.Quote(v.text)
	case buildValueNumber:
		return canonicalJSONNumber(v.text, v.number)
	case buildValueBool:
		return v.text
	case buildValueNil:
		return "null"
	case buildValueList:
		parts := make([]string, len(v.items))
		for index, item := range v.items {
			parts[index] = item.jsonText()
		}
		return "[" + strings.Join(parts, ",") + "]"
	case buildValueObject:
		parts := make([]string, 0, len(v.order))
		for _, name := range v.order {
			parts = append(parts, strconv.Quote(name)+":"+v.fields[name].jsonText())
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		return "null"
	}
}

// canonicalJSONNumber returns a JSON-valid spelling of a number value. A scalar's
// text is the author's literal form, which can be valid Go but invalid JSON (for
// example the octal-style `01`). When the literal is already canonical JSON it is
// preserved verbatim so wide integer literals keep full precision; otherwise it
// is reformatted from the parsed float.
func canonicalJSONNumber(text string, number float64) string {
	if isCanonicalJSONNumber(text) {
		return text
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// isCanonicalJSONNumber reports whether text is a JSON number literal: an
// optional minus, an integer part with no redundant leading zero, and optional
// fraction and exponent.
func isCanonicalJSONNumber(text string) bool {
	if text == "" {
		return false
	}
	index := 0
	if text[index] == '-' {
		index++
	}
	intStart := index
	for index < len(text) && text[index] >= '0' && text[index] <= '9' {
		index++
	}
	intLen := index - intStart
	if intLen == 0 {
		return false
	}
	if intLen > 1 && text[intStart] == '0' {
		return false
	}
	if index < len(text) && text[index] == '.' {
		index++
		fracStart := index
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			index++
		}
		if index == fracStart {
			return false
		}
	}
	if index < len(text) && (text[index] == 'e' || text[index] == 'E') {
		index++
		if index < len(text) && (text[index] == '+' || text[index] == '-') {
			index++
		}
		expStart := index
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			index++
		}
		if index == expStart {
			return false
		}
	}
	return index == len(text)
}

func buildValueStrings(data map[string]buildValue) map[string]string {
	out := make(map[string]string, len(data))
	for key, value := range data {
		out[key] = value.text
	}
	return out
}

func parseBuildDataCallLine(line string) (buildCallRef, bool, error) {
	expr, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return buildCallRef{}, false, nil
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") {
		return buildCallRef{}, false, nil
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return buildCallRef{}, false, fmt.Errorf("parse build call: %w", err)
	}
	call, ok := parsed.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return buildCallRef{}, false, nil
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return buildCallRef{Function: fun.Name}, true, nil
	case *ast.SelectorExpr:
		alias, ok := fun.X.(*ast.Ident)
		if !ok {
			return buildCallRef{}, false, fmt.Errorf("build data call receiver must be an import alias")
		}
		return buildCallRef{Alias: alias.Name, Function: fun.Sel.Name}, true, nil
	default:
		return buildCallRef{}, false, nil
	}
}

// buildFieldValueFromString evaluates one build field value expression in env.
// The string form (rather than a pre-parsed Go AST) is the entry point because
// build-time iteration adds comprehension and list/object literal syntax that is
// not valid Go and must be recognized before the Go expression parser runs.
func buildFieldValueFromString(name string, exprStr string, env *buildEnv) (string, buildValue, error) {
	if !isLiteralName(name) {
		return "", buildValue{}, fmt.Errorf("invalid build field name")
	}
	value, err := buildEvalExprString(exprStr, env)
	if err != nil {
		return "", buildValue{}, fmt.Errorf("build field %s: %w", name, err)
	}
	// A build field must carry meaningful data: reject a value that resolves to an
	// empty or whitespace-only string. Empty strings remain valid inside
	// expressions (for example a list element or a join separator).
	if value.kind == buildValueString && strings.TrimSpace(value.text) == "" {
		return "", buildValue{}, fmt.Errorf("build field %s: value must not be empty", name)
	}
	return name, value, nil
}

func buildEvalAST(expr ast.Expr, env *buildEnv) (buildValue, error) {
	if env.depth > buildValueMaxDepth {
		return buildValue{}, fmt.Errorf("build expression nested too deeply (limit %d)", buildValueMaxDepth)
	}
	switch typed := expr.(type) {
	case *ast.BasicLit:
		switch typed.Kind {
		case token.STRING:
			value, err := strconv.Unquote(typed.Value)
			if err != nil {
				return buildValue{}, err
			}
			interpolated, err := interpolateBuildValue(value, env.routeParams, env.interpolationValues())
			if err != nil {
				return buildValue{}, err
			}
			return buildStringValue(interpolated), nil
		case token.INT, token.FLOAT:
			number, err := strconv.ParseFloat(strings.ReplaceAll(typed.Value, "_", ""), 64)
			if err != nil {
				return buildValue{}, fmt.Errorf("invalid numeric literal %q", typed.Value)
			}
			value := buildNumberValue(number)
			if typed.Kind == token.INT {
				value.text = strings.ReplaceAll(typed.Value, "_", "")
			}
			return value, nil
		default:
			return buildValue{}, fmt.Errorf("unsupported scalar literal")
		}
	case *ast.Ident:
		switch typed.Name {
		case "true":
			return buildBoolValue(true), nil
		case "false":
			return buildBoolValue(false), nil
		case "nil", "null":
			return buildNilValue(), nil
		default:
			if value, ok := env.scope[typed.Name]; ok {
				return value, nil
			}
			value, ok := env.data[typed.Name]
			if !ok {
				return buildValue{}, fmt.Errorf("unknown build field reference %q", typed.Name)
			}
			return value, nil
		}
	case *ast.SelectorExpr:
		return buildSelectorValue(typed, env.deeper())
	case *ast.IndexExpr:
		return buildIndexValue(typed, env.deeper())
	case *ast.CallExpr:
		return buildCallValue(typed, env.deeper())
	case *ast.ParenExpr:
		return buildEvalAST(typed.X, env.deeper())
	case *ast.UnaryExpr:
		value, err := buildEvalAST(typed.X, env.deeper())
		if err != nil {
			return buildValue{}, err
		}
		return buildUnaryValue(typed.Op, value)
	case *ast.BinaryExpr:
		left, err := buildEvalAST(typed.X, env.deeper())
		if err != nil {
			return buildValue{}, err
		}
		right, err := buildEvalAST(typed.Y, env.deeper())
		if err != nil {
			return buildValue{}, err
		}
		return buildBinaryValue(typed.Op, left, right)
	default:
		return buildValue{}, fmt.Errorf("value must be a string, number, boolean, nil, expression, list, object, comprehension, builtin, param(), field(), or earlier field reference")
	}
}

func buildSelectorValue(selector *ast.SelectorExpr, env *buildEnv) (buildValue, error) {
	receiver, err := buildEvalAST(selector.X, env)
	if err != nil {
		return buildValue{}, err
	}
	if receiver.kind != buildValueObject {
		return buildValue{}, fmt.Errorf("cannot read field %q from a non-object value", selector.Sel.Name)
	}
	value, ok := receiver.fields[selector.Sel.Name]
	if !ok {
		return buildValue{}, fmt.Errorf("unknown object field %q", selector.Sel.Name)
	}
	return value, nil
}

func buildIndexValue(index *ast.IndexExpr, env *buildEnv) (buildValue, error) {
	receiver, err := buildEvalAST(index.X, env)
	if err != nil {
		return buildValue{}, err
	}
	if receiver.kind != buildValueList {
		return buildValue{}, fmt.Errorf("cannot index a non-list value")
	}
	position, err := buildEvalAST(index.Index, env)
	if err != nil {
		return buildValue{}, err
	}
	offset, err := buildIntFromValue(position, "list index")
	if err != nil {
		return buildValue{}, err
	}
	if offset < 0 || offset >= len(receiver.items) {
		return buildValue{}, fmt.Errorf("list index %d out of range (length %d)", offset, len(receiver.items))
	}
	return receiver.items[offset], nil
}

func buildCallValue(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	name, ok := call.Fun.(*ast.Ident)
	if !ok {
		return buildValue{}, fmt.Errorf("unsupported build value call")
	}
	switch name.Name {
	case "param", "field":
		return buildLookupCall(name.Name, call, env)
	case "seq":
		return buildSeqCall(call, env)
	case "count":
		return buildCountCall(call, env)
	case "sum":
		return buildSumCall(call, env)
	case "join":
		return buildJoinCall(call, env)
	case "first", "last":
		return buildEndCall(name.Name, call, env)
	case "take":
		return buildTakeCall(call, env)
	case "reverse":
		return buildReverseCall(call, env)
	default:
		return buildValue{}, fmt.Errorf("unsupported build value call %s", name.Name)
	}
}

func buildLookupCall(name string, call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	if len(call.Args) != 1 {
		return buildValue{}, fmt.Errorf("%s expects 1 argument", name)
	}
	arg, ok := call.Args[0].(*ast.BasicLit)
	if !ok || arg.Kind != token.STRING {
		return buildValue{}, fmt.Errorf("%s argument must be a string literal", name)
	}
	key, err := strconv.Unquote(arg.Value)
	if err != nil {
		return buildValue{}, err
	}
	switch name {
	case "param":
		value, ok := env.routeParams[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown route param %q", key)
		}
		return buildStringValue(value), nil
	default: // field
		value, ok := env.data[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown build field %q", key)
		}
		return value, nil
	}
}

func buildSeqCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	if len(call.Args) < 1 || len(call.Args) > 2 {
		return buildValue{}, fmt.Errorf("seq expects 1 or 2 arguments")
	}
	bounds := make([]int, len(call.Args))
	for index, arg := range call.Args {
		value, err := buildEvalAST(arg, env)
		if err != nil {
			return buildValue{}, err
		}
		bounds[index], err = buildIntFromValue(value, "seq argument")
		if err != nil {
			return buildValue{}, err
		}
	}
	start, end := 0, bounds[0]
	if len(bounds) == 2 {
		start, end = bounds[0], bounds[1]
	}
	if end < start {
		return buildValue{}, fmt.Errorf("seq end %d must be >= start %d", end, start)
	}
	// Bound the operands so end-start cannot overflow int before the budget check;
	// a valid seq produces at most the per-block budget anyway.
	if start < -buildSeqBound || start > buildSeqBound || end < -buildSeqBound || end > buildSeqBound {
		return buildValue{}, fmt.Errorf("seq bounds must be within [-%d, %d]", buildSeqBound, buildSeqBound)
	}
	if err := env.consume(end - start); err != nil {
		return buildValue{}, err
	}
	items := make([]buildValue, 0, end-start)
	for value := start; value < end; value++ {
		items = append(items, buildNumberValue(float64(value)))
	}
	return buildListValue(items), nil
}

func buildCountCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	list, err := buildSingleListArg("count", call, env)
	if err != nil {
		return buildValue{}, err
	}
	return buildNumberValue(float64(len(list.items))), nil
}

func buildSumCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	list, err := buildSingleListArg("sum", call, env)
	if err != nil {
		return buildValue{}, err
	}
	total := 0.0
	for _, item := range list.items {
		if item.kind != buildValueNumber {
			return buildValue{}, fmt.Errorf("sum requires a list of numbers")
		}
		total += item.number
	}
	return buildNumberValue(total), nil
}

func buildJoinCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	if len(call.Args) != 2 {
		return buildValue{}, fmt.Errorf("join expects 2 arguments")
	}
	list, err := buildEvalAST(call.Args[0], env)
	if err != nil {
		return buildValue{}, err
	}
	if list.kind != buildValueList {
		return buildValue{}, fmt.Errorf("join requires a list as its first argument")
	}
	separator, err := buildEvalAST(call.Args[1], env)
	if err != nil {
		return buildValue{}, err
	}
	if separator.kind != buildValueString {
		return buildValue{}, fmt.Errorf("join requires a string separator")
	}
	parts := make([]string, len(list.items))
	for index, item := range list.items {
		if item.kind == buildValueList || item.kind == buildValueObject {
			return buildValue{}, fmt.Errorf("join requires a list of scalars")
		}
		parts[index] = item.text
	}
	return buildStringValue(strings.Join(parts, separator.text)), nil
}

func buildEndCall(name string, call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	list, err := buildSingleListArg(name, call, env)
	if err != nil {
		return buildValue{}, err
	}
	if len(list.items) == 0 {
		return buildNilValue(), nil
	}
	if name == "first" {
		return list.items[0], nil
	}
	return list.items[len(list.items)-1], nil
}

func buildTakeCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	if len(call.Args) != 2 {
		return buildValue{}, fmt.Errorf("take expects 2 arguments")
	}
	list, err := buildEvalAST(call.Args[0], env)
	if err != nil {
		return buildValue{}, err
	}
	if list.kind != buildValueList {
		return buildValue{}, fmt.Errorf("take requires a list as its first argument")
	}
	count, err := buildEvalAST(call.Args[1], env)
	if err != nil {
		return buildValue{}, err
	}
	limit, err := buildIntFromValue(count, "take count")
	if err != nil {
		return buildValue{}, err
	}
	if limit < 0 {
		return buildValue{}, fmt.Errorf("take count must not be negative")
	}
	if limit > len(list.items) {
		limit = len(list.items)
	}
	if err := env.consume(limit); err != nil {
		return buildValue{}, err
	}
	return buildListValue(append([]buildValue(nil), list.items[:limit]...)), nil
}

func buildReverseCall(call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	list, err := buildSingleListArg("reverse", call, env)
	if err != nil {
		return buildValue{}, err
	}
	if err := env.consume(len(list.items)); err != nil {
		return buildValue{}, err
	}
	reversed := make([]buildValue, len(list.items))
	for index, item := range list.items {
		reversed[len(list.items)-1-index] = item
	}
	return buildListValue(reversed), nil
}

func buildSingleListArg(name string, call *ast.CallExpr, env *buildEnv) (buildValue, error) {
	if len(call.Args) != 1 {
		return buildValue{}, fmt.Errorf("%s expects 1 argument", name)
	}
	value, err := buildEvalAST(call.Args[0], env)
	if err != nil {
		return buildValue{}, err
	}
	if value.kind != buildValueList {
		return buildValue{}, fmt.Errorf("%s requires a list argument", name)
	}
	return value, nil
}

func buildIntFromValue(value buildValue, label string) (int, error) {
	if value.kind != buildValueNumber {
		return 0, fmt.Errorf("%s must be a number", label)
	}
	if value.number != math.Trunc(value.number) {
		return 0, fmt.Errorf("%s must be an integer", label)
	}
	return int(value.number), nil
}

func buildUnaryValue(op token.Token, value buildValue) (buildValue, error) {
	switch op {
	case token.ADD:
		if value.kind != buildValueNumber {
			return buildValue{}, fmt.Errorf("unary + requires a number")
		}
		return value, nil
	case token.SUB:
		if value.kind != buildValueNumber {
			return buildValue{}, fmt.Errorf("unary - requires a number")
		}
		return buildNumberValue(-value.number), nil
	case token.NOT:
		if value.kind != buildValueBool {
			return buildValue{}, fmt.Errorf("unary ! requires a boolean")
		}
		return buildBoolValue(!value.boolean), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported unary operator %s", op)
	}
}

func buildBinaryValue(op token.Token, left, right buildValue) (buildValue, error) {
	switch op {
	case token.ADD:
		if left.kind == buildValueString || right.kind == buildValueString {
			return buildStringValue(left.text + right.text), nil
		}
		return buildNumericBinaryValue(op, left, right)
	case token.SUB, token.MUL, token.QUO, token.REM:
		return buildNumericBinaryValue(op, left, right)
	case token.EQL, token.NEQ:
		equal, err := buildValuesEqual(left, right)
		if err != nil {
			return buildValue{}, err
		}
		if op == token.NEQ {
			equal = !equal
		}
		return buildBoolValue(equal), nil
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		return buildOrderedComparisonValue(op, left, right)
	case token.LAND, token.LOR:
		if left.kind != buildValueBool || right.kind != buildValueBool {
			return buildValue{}, fmt.Errorf("logical operator %s requires booleans", op)
		}
		if op == token.LAND {
			return buildBoolValue(left.boolean && right.boolean), nil
		}
		return buildBoolValue(left.boolean || right.boolean), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported binary operator %s", op)
	}
}

func buildNumericBinaryValue(op token.Token, left, right buildValue) (buildValue, error) {
	if left.kind != buildValueNumber || right.kind != buildValueNumber {
		return buildValue{}, fmt.Errorf("operator %s requires numbers", op)
	}
	switch op {
	case token.ADD:
		return buildNumberValue(left.number + right.number), nil
	case token.SUB:
		return buildNumberValue(left.number - right.number), nil
	case token.MUL:
		return buildNumberValue(left.number * right.number), nil
	case token.QUO:
		if right.number == 0 {
			return buildValue{}, fmt.Errorf("division by zero")
		}
		return buildNumberValue(left.number / right.number), nil
	case token.REM:
		if right.number == 0 {
			return buildValue{}, fmt.Errorf("division by zero")
		}
		return buildNumberValue(math.Mod(left.number, right.number)), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported numeric operator %s", op)
	}
}

func buildValuesEqual(left, right buildValue) (bool, error) {
	if left.kind != right.kind {
		return false, nil
	}
	switch left.kind {
	case buildValueString, buildValueNil:
		return left.text == right.text, nil
	case buildValueNumber:
		return left.number == right.number, nil
	case buildValueBool:
		return left.boolean == right.boolean, nil
	case buildValueList, buildValueObject:
		return left.text == right.text, nil
	default:
		return false, fmt.Errorf("unsupported equality operands")
	}
}

func buildOrderedComparisonValue(op token.Token, left, right buildValue) (buildValue, error) {
	if left.kind != right.kind {
		return buildValue{}, fmt.Errorf("operator %s requires matching operand types", op)
	}
	var result bool
	switch left.kind {
	case buildValueNumber:
		result = compareOrdered(op, left.number, right.number)
	case buildValueString:
		result = compareOrdered(op, left.text, right.text)
	default:
		return buildValue{}, fmt.Errorf("operator %s requires strings or numbers", op)
	}
	return buildBoolValue(result), nil
}

func compareOrdered[T ~float64 | ~string](op token.Token, left, right T) bool {
	switch op {
	case token.LSS:
		return left < right
	case token.LEQ:
		return left <= right
	case token.GTR:
		return left > right
	case token.GEQ:
		return left >= right
	default:
		return false
	}
}

func significantBuildLines(body string) []string {
	var lines []string
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
