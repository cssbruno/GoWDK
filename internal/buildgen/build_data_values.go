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
)

type buildValue struct {
	kind    buildValueKind
	text    string
	number  float64
	boolean bool
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

func parseBuildLiteralLine(line string) (*ast.CompositeLit, bool, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return nil, false, nil
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return nil, false, nil
	}
	expr, err := parser.ParseExpr("struct{}" + body)
	if err != nil {
		return nil, true, fmt.Errorf("parse build literal: %w", err)
	}
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, true, fmt.Errorf("build literal must be an object")
	}
	return literal, true, nil
}

func buildFieldValue(expr ast.Expr, routeParams map[string]string, data map[string]buildValue) (string, buildValue, error) {
	kv, ok := expr.(*ast.KeyValueExpr)
	if !ok {
		return "", buildValue{}, fmt.Errorf("build field must use name: value")
	}
	key, ok := kv.Key.(*ast.Ident)
	if !ok || !isLiteralName(key.Name) {
		return "", buildValue{}, fmt.Errorf("invalid build field name")
	}
	value, err := buildValueFromExpr(kv.Value, routeParams, data)
	if err != nil {
		return "", buildValue{}, fmt.Errorf("build field %s: %w", key.Name, err)
	}
	return key.Name, value, nil
}

func buildValueFromExpr(expr ast.Expr, routeParams map[string]string, data map[string]buildValue) (buildValue, error) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		switch typed.Kind {
		case token.STRING:
			value, err := strconv.Unquote(typed.Value)
			if err != nil {
				return buildValue{}, err
			}
			if strings.TrimSpace(value) == "" {
				return buildValue{}, fmt.Errorf("value must not be empty")
			}
			interpolated, err := interpolateBuildValue(value, routeParams, buildValueStrings(data))
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
			value, ok := data[typed.Name]
			if !ok {
				return buildValue{}, fmt.Errorf("unknown build field reference %q", typed.Name)
			}
			return value, nil
		}
	case *ast.CallExpr:
		return buildCallValue(typed, routeParams, data)
	case *ast.ParenExpr:
		return buildValueFromExpr(typed.X, routeParams, data)
	case *ast.UnaryExpr:
		value, err := buildValueFromExpr(typed.X, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		return buildUnaryValue(typed.Op, value)
	case *ast.BinaryExpr:
		left, err := buildValueFromExpr(typed.X, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		right, err := buildValueFromExpr(typed.Y, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		return buildBinaryValue(typed.Op, left, right)
	default:
		return buildValue{}, fmt.Errorf("value must be a string, number, boolean, nil, expression, param(), field(), or earlier field reference")
	}
}

func buildCallValue(call *ast.CallExpr, routeParams map[string]string, data map[string]buildValue) (buildValue, error) {
	name, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return buildValue{}, fmt.Errorf("unsupported build value call")
	}
	arg, ok := call.Args[0].(*ast.BasicLit)
	if !ok || arg.Kind != token.STRING {
		return buildValue{}, fmt.Errorf("%s argument must be a string literal", name.Name)
	}
	key, err := strconv.Unquote(arg.Value)
	if err != nil {
		return buildValue{}, err
	}
	switch name.Name {
	case "param":
		value, ok := routeParams[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown route param %q", key)
		}
		return buildStringValue(value), nil
	case "field":
		value, ok := data[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown build field %q", key)
		}
		return value, nil
	default:
		return buildValue{}, fmt.Errorf("unsupported build value call %s", name.Name)
	}
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
