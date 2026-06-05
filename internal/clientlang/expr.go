package clientlang

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// ValueType is the small client expression type universe.
type ValueType string

const (
	TypeUnknown ValueType = ""
	TypeString  ValueType = "string"
	TypeInt     ValueType = "int"
	TypeFloat   ValueType = "float"
	TypeBool    ValueType = "bool"
	TypeNil     ValueType = "nil"
	TypeObject  ValueType = "object"
	TypeArray   ValueType = "array"
)

// Expr describes a parsed client expression.
type Expr interface {
	exprNode()
}

// ExprFunction describes a return-valued helper callable from expressions.
type ExprFunction struct {
	Params []ValueType
	Return ValueType
}

// LiteralExpr is a scalar literal.
type LiteralExpr struct {
	Type  ValueType
	Value string
}

func (LiteralExpr) exprNode() {}

// IdentExpr reads a state, prop, param, or local name.
type IdentExpr struct {
	Name string
}

func (IdentExpr) exprNode() {}

// MemberExpr reads a field from an object expression.
type MemberExpr struct {
	X    Expr
	Name string
}

func (MemberExpr) exprNode() {}

// IndexExpr reads an item from an array expression.
type IndexExpr struct {
	X     Expr
	Index Expr
}

func (IndexExpr) exprNode() {}

// CallExpr invokes a component-local helper function.
type CallExpr struct {
	Name string
	Args []Expr
}

func (CallExpr) exprNode() {}

// UnaryExpr applies a unary operator.
type UnaryExpr struct {
	Op string
	X  Expr
}

func (UnaryExpr) exprNode() {}

// BinaryExpr applies a binary operator.
type BinaryExpr struct {
	Op          string
	Left, Right Expr
}

func (BinaryExpr) exprNode() {}

// ConditionalExpr chooses between two expressions from a bool condition.
type ConditionalExpr struct {
	Cond Expr
	Then Expr
	Else Expr
}

func (ConditionalExpr) exprNode() {}

// ParseExpr parses the supported client expression subset.
func ParseExpr(source string) (Expr, error) {
	parser := exprParser{lexer: newExprLexer(source)}
	expr, err := parser.parseConditional()
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != tokenEOF {
		return nil, fmt.Errorf("unexpected token %q", parser.peek().value)
	}
	return expr, nil
}

// CheckExpr parses and type-checks a client expression against symbols.
func CheckExpr(source string, symbols map[string]ValueType) (ValueType, []string, error) {
	return CheckExprWithFunctions(source, symbols, nil)
}

// CheckExprWithFunctions parses and type-checks a client expression against
// value symbols and return-valued helper functions.
func CheckExprWithFunctions(source string, symbols map[string]ValueType, functions map[string]ExprFunction) (ValueType, []string, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return TypeUnknown, nil, err
	}
	fields := map[string]bool{}
	typ, err := checkExpr(expr, symbols, functions, fields)
	if err != nil {
		return TypeUnknown, nil, err
	}
	return typ, sortedStringKeys(fields), nil
}

func checkExpr(expr Expr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool) (ValueType, error) {
	switch typed := expr.(type) {
	case LiteralExpr:
		return typed.Type, nil
	case IdentExpr:
		typ, ok := symbols[typed.Name]
		if !ok {
			return TypeUnknown, fmt.Errorf("unknown client value %q", typed.Name)
		}
		fields[typed.Name] = true
		return typ, nil
	case MemberExpr:
		base, err := checkExpr(typed.X, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		path := exprPath(typed)
		if path != "" {
			if typ, ok := symbols[path]; ok {
				return typ, nil
			}
		}
		if base == TypeUnknown {
			return TypeUnknown, nil
		}
		if base != TypeObject && base != TypeArray {
			return TypeUnknown, fmt.Errorf("cannot read field %q from %s expression", typed.Name, base)
		}
		if path != "" {
			return TypeUnknown, fmt.Errorf("unknown client value %q", path)
		}
		return TypeUnknown, fmt.Errorf("unknown client field %q", typed.Name)
	case IndexExpr:
		base, err := checkExpr(typed.X, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		index, err := checkExpr(typed.Index, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		if index != TypeUnknown && index != TypeInt {
			return TypeUnknown, fmt.Errorf("index expression requires int, got %s", index)
		}
		path := exprPath(typed)
		if path != "" {
			if typ, ok := symbols[path]; ok {
				return typ, nil
			}
		}
		if base == TypeUnknown {
			return TypeUnknown, nil
		}
		if base != TypeArray {
			return TypeUnknown, fmt.Errorf("cannot index %s expression", base)
		}
		if path != "" {
			return TypeUnknown, fmt.Errorf("unknown client value %q", path)
		}
		return TypeUnknown, nil
	case CallExpr:
		if typ, ok, err := checkBuiltinCall(typed, symbols, functions, fields); ok || err != nil {
			return typ, err
		}
		function, ok := functions[typed.Name]
		if !ok {
			return TypeUnknown, fmt.Errorf("unknown client helper function %q", typed.Name)
		}
		if len(typed.Args) != len(function.Params) {
			return TypeUnknown, fmt.Errorf("client helper function %s expects %d arguments, got %d", typed.Name, len(function.Params), len(typed.Args))
		}
		for index, arg := range typed.Args {
			actual, err := checkExpr(arg, symbols, functions, fields)
			if err != nil {
				return TypeUnknown, err
			}
			expected := function.Params[index]
			if expected != TypeUnknown && actual != TypeUnknown && expected != actual && !compatibleNumericType(actual, expected) {
				return TypeUnknown, fmt.Errorf("client helper function %s argument %d expects %s, got %s", typed.Name, index+1, expected, actual)
			}
		}
		return function.Return, nil
	case UnaryExpr:
		typ, err := checkExpr(typed.X, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		switch typed.Op {
		case "!":
			if typ == TypeUnknown {
				return TypeUnknown, nil
			}
			if typ != TypeBool {
				return TypeUnknown, fmt.Errorf("operator ! requires bool, got %s", typ)
			}
			return TypeBool, nil
		case "-":
			if typ == TypeUnknown {
				return TypeUnknown, nil
			}
			if !isNumericType(typ) {
				return TypeUnknown, fmt.Errorf("operator - requires number, got %s", typ)
			}
			return typ, nil
		default:
			return TypeUnknown, fmt.Errorf("unsupported unary operator %q", typed.Op)
		}
	case BinaryExpr:
		left, err := checkExpr(typed.Left, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		right, err := checkExpr(typed.Right, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		return checkBinaryExpr(typed.Op, left, right)
	case ConditionalExpr:
		cond, err := checkExpr(typed.Cond, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		if cond != TypeUnknown && cond != TypeBool {
			return TypeUnknown, fmt.Errorf("if expression condition requires bool, got %s", cond)
		}
		thenType, err := checkExpr(typed.Then, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		elseType, err := checkExpr(typed.Else, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		return checkConditionalBranches(thenType, elseType)
	default:
		return TypeUnknown, fmt.Errorf("unknown expression node")
	}
}

func checkConditionalBranches(thenType, elseType ValueType) (ValueType, error) {
	if thenType == TypeUnknown || elseType == TypeUnknown {
		return TypeUnknown, nil
	}
	if thenType == elseType {
		return thenType, nil
	}
	if compatibleNumericType(thenType, elseType) {
		if thenType == TypeFloat || elseType == TypeFloat {
			return TypeFloat, nil
		}
		return TypeInt, nil
	}
	if thenType == TypeNil {
		return elseType, nil
	}
	if elseType == TypeNil {
		return thenType, nil
	}
	return TypeUnknown, fmt.Errorf("if expression branches must have matching types, got %s and %s", thenType, elseType)
}

func checkBinaryExpr(op string, left, right ValueType) (ValueType, error) {
	if left == TypeUnknown || right == TypeUnknown {
		return TypeUnknown, nil
	}
	switch op {
	case "+", "-", "*", "/", "%":
		if op == "+" && left == TypeString && right == TypeString {
			return TypeString, nil
		}
		if !isNumericType(left) || !isNumericType(right) {
			return TypeUnknown, fmt.Errorf("operator %s requires numbers", op)
		}
		if left == TypeFloat || right == TypeFloat || op == "/" {
			return TypeFloat, nil
		}
		return TypeInt, nil
	case "==", "!=":
		if left != right && left != TypeNil && right != TypeNil {
			return TypeUnknown, fmt.Errorf("operator %s requires comparable matching types", op)
		}
		return TypeBool, nil
	case "<", "<=", ">", ">=":
		if isNumericType(left) && isNumericType(right) {
			return TypeBool, nil
		}
		if left == TypeString && right == TypeString {
			return TypeBool, nil
		}
		return TypeUnknown, fmt.Errorf("operator %s requires numbers or strings", op)
	case "&&", "||":
		if left != TypeBool || right != TypeBool {
			return TypeUnknown, fmt.Errorf("operator %s requires bools", op)
		}
		return TypeBool, nil
	default:
		return TypeUnknown, fmt.Errorf("unsupported binary operator %q", op)
	}
}

// ExprFields returns identifier references from a syntactically valid
// expression. It does not require type information.
func ExprFields(source string) ([]string, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return nil, err
	}
	fields := map[string]bool{}
	collectExprFields(expr, fields)
	return sortedStringKeys(fields), nil
}

// ExprCalls returns helper function call names from a syntactically valid
// expression. It does not require type information.
func ExprCalls(source string) ([]string, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return nil, err
	}
	calls := map[string]bool{}
	collectExprCalls(expr, calls)
	return sortedStringKeys(calls), nil
}

// EvalBool evaluates a supported expression as a bool against scalar values.
func EvalBool(source string, values map[string]string) (bool, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return false, err
	}
	value, err := evalExpr(expr, values)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q does not evaluate to bool", source)
	}
	return result, nil
}

// EvalValue evaluates a supported expression and returns its typed value.
func EvalValue(source string, values map[string]string) (any, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return nil, err
	}
	return evalExpr(expr, values)
}

// EvalScalar evaluates a supported expression and returns the browser-visible
// scalar string representation.
func EvalScalar(source string, values map[string]string) (string, error) {
	value, err := EvalValue(source, values)
	if err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case json.Number:
		return typed.String(), nil
	default:
		return fmt.Sprint(typed), nil
	}
}

func evalExpr(expr Expr, values map[string]string) (any, error) {
	switch typed := expr.(type) {
	case LiteralExpr:
		return evalLiteral(typed)
	case IdentExpr:
		raw, ok := values[typed.Name]
		if !ok {
			return nil, fmt.Errorf("unknown client value %q", typed.Name)
		}
		return parseRuntimeScalar(raw), nil
	case MemberExpr:
		value, err := evalExpr(typed.X, values)
		if err != nil {
			return nil, err
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot read field %q", typed.Name)
		}
		field, ok := object[typed.Name]
		if !ok {
			return nil, fmt.Errorf("unknown client field %q", typed.Name)
		}
		return field, nil
	case IndexExpr:
		value, err := evalExpr(typed.X, values)
		if err != nil {
			return nil, err
		}
		index, err := evalExpr(typed.Index, values)
		if err != nil {
			return nil, err
		}
		number, ok := numericFloat(index)
		if !ok {
			return nil, fmt.Errorf("index expression requires int")
		}
		position := int(number)
		if float64(position) != number {
			return nil, fmt.Errorf("index expression requires int")
		}
		items, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("cannot index expression")
		}
		if position < 0 || position >= len(items) {
			return nil, fmt.Errorf("index %d out of range", position)
		}
		return items[position], nil
	case CallExpr:
		if value, ok, err := evalBuiltinCall(typed, values); ok || err != nil {
			return value, err
		}
		return nil, fmt.Errorf("cannot evaluate helper function %q without a runtime helper scope", typed.Name)
	case UnaryExpr:
		value, err := evalExpr(typed.X, values)
		if err != nil {
			return nil, err
		}
		switch typed.Op {
		case "!":
			boolValue, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("operator ! requires bool")
			}
			return !boolValue, nil
		case "-":
			number, ok := numericFloat(value)
			if !ok {
				return nil, fmt.Errorf("operator - requires number")
			}
			return -number, nil
		default:
			return nil, fmt.Errorf("unsupported unary operator %q", typed.Op)
		}
	case BinaryExpr:
		left, err := evalExpr(typed.Left, values)
		if err != nil {
			return nil, err
		}
		right, err := evalExpr(typed.Right, values)
		if err != nil {
			return nil, err
		}
		return evalBinary(typed.Op, left, right)
	case ConditionalExpr:
		cond, err := evalExpr(typed.Cond, values)
		if err != nil {
			return nil, err
		}
		boolValue, ok := cond.(bool)
		if !ok {
			return nil, fmt.Errorf("if expression condition requires bool")
		}
		if boolValue {
			return evalExpr(typed.Then, values)
		}
		return evalExpr(typed.Else, values)
	default:
		return nil, fmt.Errorf("unknown expression node")
	}
}

func evalLiteral(expr LiteralExpr) (any, error) {
	switch expr.Type {
	case TypeString:
		return strconv.Unquote(expr.Value)
	case TypeInt:
		return strconv.Atoi(expr.Value)
	case TypeFloat:
		return strconv.ParseFloat(expr.Value, 64)
	case TypeBool:
		return expr.Value == "true", nil
	case TypeNil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown literal type %q", expr.Type)
	}
}

func evalBinary(op string, left, right any) (any, error) {
	switch op {
	case "+":
		if leftString, ok := left.(string); ok {
			rightString, ok := right.(string)
			if !ok {
				return nil, fmt.Errorf("operator + requires matching types")
			}
			return leftString + rightString, nil
		}
		leftNumber, leftOK := numericFloat(left)
		rightNumber, rightOK := numericFloat(right)
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("operator + requires numbers")
		}
		return leftNumber + rightNumber, nil
	case "-", "*", "/", "%":
		leftNumber, leftOK := numericFloat(left)
		rightNumber, rightOK := numericFloat(right)
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("operator %s requires numbers", op)
		}
		switch op {
		case "-":
			return leftNumber - rightNumber, nil
		case "*":
			return leftNumber * rightNumber, nil
		case "/":
			return leftNumber / rightNumber, nil
		default:
			return float64(int(leftNumber) % int(rightNumber)), nil
		}
	case "==":
		return reflect.DeepEqual(left, right), nil
	case "!=":
		return !reflect.DeepEqual(left, right), nil
	case "<", "<=", ">", ">=":
		if leftString, ok := left.(string); ok {
			rightString, ok := right.(string)
			if !ok {
				return nil, fmt.Errorf("operator %s requires matching types", op)
			}
			switch op {
			case "<":
				return leftString < rightString, nil
			case "<=":
				return leftString <= rightString, nil
			case ">":
				return leftString > rightString, nil
			default:
				return leftString >= rightString, nil
			}
		}
		leftNumber, leftOK := numericFloat(left)
		rightNumber, rightOK := numericFloat(right)
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("operator %s requires numbers or strings", op)
		}
		switch op {
		case "<":
			return leftNumber < rightNumber, nil
		case "<=":
			return leftNumber <= rightNumber, nil
		case ">":
			return leftNumber > rightNumber, nil
		default:
			return leftNumber >= rightNumber, nil
		}
	case "&&", "||":
		leftBool, leftOK := left.(bool)
		rightBool, rightOK := right.(bool)
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("operator %s requires bools", op)
		}
		if op == "&&" {
			return leftBool && rightBool, nil
		}
		return leftBool || rightBool, nil
	default:
		return nil, fmt.Errorf("unsupported binary operator %q", op)
	}
}

func parseRuntimeScalar(value string) any {
	switch value {
	case "true":
		return true
	case "false":
		return false
	case "":
		return ""
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err == nil {
			return decoded
		}
	}
	if strings.Contains(value, ".") {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return value
}

func numericFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func checkBuiltinCall(expr CallExpr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool) (ValueType, bool, error) {
	switch expr.Name {
	case "len":
		if len(expr.Args) != 1 {
			return TypeUnknown, true, fmt.Errorf("built-in len expects 1 argument, got %d", len(expr.Args))
		}
		actual, err := checkExpr(expr.Args[0], symbols, functions, fields)
		if err != nil {
			return TypeUnknown, true, err
		}
		if actual == TypeUnknown || actual == TypeString || actual == TypeArray {
			return TypeInt, true, nil
		}
		return TypeUnknown, true, fmt.Errorf("built-in len expects string or array, got %s", actual)
	case "string":
		return checkConversionBuiltin(expr, symbols, functions, fields, TypeString)
	case "int":
		return checkConversionBuiltin(expr, symbols, functions, fields, TypeInt)
	case "float":
		return checkConversionBuiltin(expr, symbols, functions, fields, TypeFloat)
	default:
		return TypeUnknown, false, nil
	}
}

func checkConversionBuiltin(expr CallExpr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool, out ValueType) (ValueType, bool, error) {
	if len(expr.Args) != 1 {
		return TypeUnknown, true, fmt.Errorf("built-in %s expects 1 argument, got %d", expr.Name, len(expr.Args))
	}
	actual, err := checkExpr(expr.Args[0], symbols, functions, fields)
	if err != nil {
		return TypeUnknown, true, err
	}
	if actual == TypeUnknown {
		return out, true, nil
	}
	switch expr.Name {
	case "string":
		if actual == TypeArray || actual == TypeObject {
			return TypeUnknown, true, fmt.Errorf("built-in string expects scalar, got %s", actual)
		}
	case "int", "float":
		if actual != TypeString && !isNumericType(actual) {
			return TypeUnknown, true, fmt.Errorf("built-in %s expects string or number, got %s", expr.Name, actual)
		}
	}
	return out, true, nil
}

func evalBuiltinCall(expr CallExpr, values map[string]string) (any, bool, error) {
	switch expr.Name {
	case "len":
		if len(expr.Args) != 1 {
			return nil, true, fmt.Errorf("built-in len expects 1 argument, got %d", len(expr.Args))
		}
		value, err := evalExpr(expr.Args[0], values)
		if err != nil {
			return nil, true, err
		}
		switch typed := value.(type) {
		case string:
			return len(typed), true, nil
		case []any:
			return len(typed), true, nil
		default:
			return nil, true, fmt.Errorf("built-in len expects string or array")
		}
	case "string":
		return evalStringBuiltin(expr, values)
	case "int":
		return evalNumericBuiltin(expr, values, TypeInt)
	case "float":
		return evalNumericBuiltin(expr, values, TypeFloat)
	default:
		return nil, false, nil
	}
}

func evalStringBuiltin(expr CallExpr, values map[string]string) (any, bool, error) {
	if len(expr.Args) != 1 {
		return nil, true, fmt.Errorf("built-in string expects 1 argument, got %d", len(expr.Args))
	}
	value, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return nil, true, err
	}
	switch typed := value.(type) {
	case nil:
		return "", true, nil
	case string:
		return typed, true, nil
	case bool:
		return strconv.FormatBool(typed), true, nil
	case int:
		return strconv.Itoa(typed), true, nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true, nil
	case json.Number:
		return typed.String(), true, nil
	default:
		return nil, true, fmt.Errorf("built-in string expects scalar")
	}
}

func evalNumericBuiltin(expr CallExpr, values map[string]string, out ValueType) (any, bool, error) {
	if len(expr.Args) != 1 {
		return nil, true, fmt.Errorf("built-in %s expects 1 argument, got %d", expr.Name, len(expr.Args))
	}
	value, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return nil, true, err
	}
	var number float64
	if typed, ok := value.(string); ok {
		number, err = strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return nil, true, fmt.Errorf("built-in %s cannot parse %q", expr.Name, typed)
		}
	} else {
		var ok bool
		number, ok = numericFloat(value)
		if !ok {
			return nil, true, fmt.Errorf("built-in %s expects string or number", expr.Name)
		}
	}
	if out == TypeInt {
		return int(number), true, nil
	}
	return number, true, nil
}

func collectExprFields(expr Expr, fields map[string]bool) {
	switch typed := expr.(type) {
	case IdentExpr:
		fields[typed.Name] = true
	case MemberExpr:
		collectExprFields(typed.X, fields)
	case IndexExpr:
		collectExprFields(typed.X, fields)
		collectExprFields(typed.Index, fields)
	case CallExpr:
		for _, arg := range typed.Args {
			collectExprFields(arg, fields)
		}
	case UnaryExpr:
		collectExprFields(typed.X, fields)
	case BinaryExpr:
		collectExprFields(typed.Left, fields)
		collectExprFields(typed.Right, fields)
	case ConditionalExpr:
		collectExprFields(typed.Cond, fields)
		collectExprFields(typed.Then, fields)
		collectExprFields(typed.Else, fields)
	}
}

func collectExprCalls(expr Expr, calls map[string]bool) {
	switch typed := expr.(type) {
	case MemberExpr:
		collectExprCalls(typed.X, calls)
	case IndexExpr:
		collectExprCalls(typed.X, calls)
		collectExprCalls(typed.Index, calls)
	case CallExpr:
		calls[typed.Name] = true
		for _, arg := range typed.Args {
			collectExprCalls(arg, calls)
		}
	case UnaryExpr:
		collectExprCalls(typed.X, calls)
	case BinaryExpr:
		collectExprCalls(typed.Left, calls)
		collectExprCalls(typed.Right, calls)
	case ConditionalExpr:
		collectExprCalls(typed.Cond, calls)
		collectExprCalls(typed.Then, calls)
		collectExprCalls(typed.Else, calls)
	}
}

func exprPath(expr Expr) string {
	switch typed := expr.(type) {
	case IdentExpr:
		return typed.Name
	case MemberExpr:
		base := exprPath(typed.X)
		if base == "" {
			return ""
		}
		return base + "." + typed.Name
	case IndexExpr:
		base := exprPath(typed.X)
		if base == "" {
			return ""
		}
		return base + "[]"
	default:
		return ""
	}
}

func isNumericType(typ ValueType) bool {
	return typ == TypeInt || typ == TypeFloat
}

func compatibleNumericType(left, right ValueType) bool {
	return isNumericType(left) && isNumericType(right)
}

func sortedStringKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		item := values[i]
		j := i - 1
		for ; j >= 0 && values[j] > item; j-- {
			values[j+1] = values[j]
		}
		values[j+1] = item
	}
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenString
	tokenNumber
	tokenBool
	tokenNil
	tokenOp
	tokenLParen
	tokenRParen
	tokenDot
	tokenLBracket
	tokenRBracket
	tokenLBrace
	tokenRBrace
	tokenComma
)

type exprToken struct {
	kind  tokenKind
	value string
}

type exprLexer struct {
	source []rune
	index  int
}

func newExprLexer(source string) *exprLexer {
	return &exprLexer{source: []rune(strings.TrimSpace(source))}
}

func (lexer *exprLexer) next() (exprToken, error) {
	lexer.skipSpace()
	if lexer.index >= len(lexer.source) {
		return exprToken{kind: tokenEOF}, nil
	}
	char := lexer.source[lexer.index]
	switch {
	case isExprIdentStart(char):
		return lexer.ident(), nil
	case unicode.IsDigit(char):
		return lexer.number(), nil
	case char == '"':
		return lexer.string()
	case char == '(':
		lexer.index++
		return exprToken{kind: tokenLParen, value: "("}, nil
	case char == ')':
		lexer.index++
		return exprToken{kind: tokenRParen, value: ")"}, nil
	case char == '.':
		lexer.index++
		return exprToken{kind: tokenDot, value: "."}, nil
	case char == '[':
		lexer.index++
		return exprToken{kind: tokenLBracket, value: "["}, nil
	case char == ']':
		lexer.index++
		return exprToken{kind: tokenRBracket, value: "]"}, nil
	case char == '{':
		lexer.index++
		return exprToken{kind: tokenLBrace, value: "{"}, nil
	case char == '}':
		lexer.index++
		return exprToken{kind: tokenRBrace, value: "}"}, nil
	case char == ',':
		lexer.index++
		return exprToken{kind: tokenComma, value: ","}, nil
	default:
		return lexer.operator()
	}
}

func (lexer *exprLexer) skipSpace() {
	for lexer.index < len(lexer.source) && unicode.IsSpace(lexer.source[lexer.index]) {
		lexer.index++
	}
}

func (lexer *exprLexer) ident() exprToken {
	start := lexer.index
	for lexer.index < len(lexer.source) && isExprIdentPart(lexer.source[lexer.index]) {
		lexer.index++
	}
	value := string(lexer.source[start:lexer.index])
	switch value {
	case "true", "false":
		return exprToken{kind: tokenBool, value: value}
	case "nil":
		return exprToken{kind: tokenNil, value: value}
	default:
		return exprToken{kind: tokenIdent, value: value}
	}
}

func (lexer *exprLexer) number() exprToken {
	start := lexer.index
	for lexer.index < len(lexer.source) && unicode.IsDigit(lexer.source[lexer.index]) {
		lexer.index++
	}
	if lexer.index < len(lexer.source) && lexer.source[lexer.index] == '.' {
		lexer.index++
		for lexer.index < len(lexer.source) && unicode.IsDigit(lexer.source[lexer.index]) {
			lexer.index++
		}
	}
	return exprToken{kind: tokenNumber, value: string(lexer.source[start:lexer.index])}
}

func (lexer *exprLexer) string() (exprToken, error) {
	start := lexer.index
	lexer.index++
	escaped := false
	for lexer.index < len(lexer.source) {
		char := lexer.source[lexer.index]
		lexer.index++
		if escaped {
			escaped = false
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '"':
			value := string(lexer.source[start:lexer.index])
			if _, err := strconv.Unquote(value); err != nil {
				return exprToken{}, err
			}
			return exprToken{kind: tokenString, value: value}, nil
		}
	}
	return exprToken{}, fmt.Errorf("unterminated string")
}

func (lexer *exprLexer) operator() (exprToken, error) {
	remaining := string(lexer.source[lexer.index:])
	for _, op := range []string{"==", "!=", "<=", ">=", "&&", "||", "+", "-", "*", "/", "%", "!", "<", ">"} {
		if strings.HasPrefix(remaining, op) {
			lexer.index += len([]rune(op))
			return exprToken{kind: tokenOp, value: op}, nil
		}
	}
	return exprToken{}, fmt.Errorf("unexpected character %q", lexer.source[lexer.index])
}

func isExprIdentStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isExprIdentPart(char rune) bool {
	return isExprIdentStart(char) || unicode.IsDigit(char)
}

type exprParser struct {
	lexer  *exprLexer
	buffer *exprToken
}

func (parser *exprParser) peek() exprToken {
	if parser.buffer != nil {
		return *parser.buffer
	}
	token, err := parser.lexer.next()
	if err != nil {
		token = exprToken{kind: tokenEOF, value: err.Error()}
	}
	parser.buffer = &token
	return token
}

func (parser *exprParser) consume() exprToken {
	token := parser.peek()
	parser.buffer = nil
	return token
}

func (parser *exprParser) parseOr() (Expr, error) {
	return parser.parseBinary(parser.parseAnd, "||")
}

func (parser *exprParser) parseConditional() (Expr, error) {
	token := parser.peek()
	if token.kind != tokenIdent || token.value != "if" {
		return parser.parseOr()
	}
	parser.consume()
	cond, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if token := parser.consume(); token.kind != tokenLBrace {
		return nil, parser.expected("opening { after if condition", token)
	}
	thenExpr, err := parser.parseConditional()
	if err != nil {
		return nil, err
	}
	if token := parser.consume(); token.kind != tokenRBrace {
		return nil, parser.expected("closing } after if branch", token)
	}
	token = parser.consume()
	if token.kind != tokenIdent || token.value != "else" {
		return nil, parser.expected("else after if branch", token)
	}
	if token := parser.consume(); token.kind != tokenLBrace {
		return nil, parser.expected("opening { after else", token)
	}
	elseExpr, err := parser.parseConditional()
	if err != nil {
		return nil, err
	}
	if token := parser.consume(); token.kind != tokenRBrace {
		return nil, parser.expected("closing } after else branch", token)
	}
	return ConditionalExpr{Cond: cond, Then: thenExpr, Else: elseExpr}, nil
}

func (parser *exprParser) parseAnd() (Expr, error) {
	return parser.parseBinary(parser.parseCompare, "&&")
}

func (parser *exprParser) parseCompare() (Expr, error) {
	return parser.parseBinary(parser.parseAdd, "==", "!=", "<", "<=", ">", ">=")
}

func (parser *exprParser) parseAdd() (Expr, error) {
	return parser.parseBinary(parser.parseMul, "+", "-")
}

func (parser *exprParser) parseMul() (Expr, error) {
	return parser.parseBinary(parser.parseUnary, "*", "/", "%")
}

func (parser *exprParser) parseBinary(next func() (Expr, error), ops ...string) (Expr, error) {
	left, err := next()
	if err != nil {
		return nil, err
	}
	for containsString(ops, parser.peek().value) && parser.peek().kind == tokenOp {
		op := parser.consume().value
		right, err := next()
		if err != nil {
			return nil, err
		}
		left = BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (parser *exprParser) parseUnary() (Expr, error) {
	token := parser.peek()
	if token.kind == tokenOp && (token.value == "!" || token.value == "-") {
		parser.consume()
		expr, err := parser.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryExpr{Op: token.value, X: expr}, nil
	}
	return parser.parsePostfix()
}

func (parser *exprParser) parsePostfix() (Expr, error) {
	expr, err := parser.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch parser.peek().kind {
		case tokenDot:
			parser.consume()
			token := parser.consume()
			if token.kind != tokenIdent {
				if token.value != "" {
					return nil, fmt.Errorf("expected field name after ., got %q", token.value)
				}
				return nil, fmt.Errorf("expected field name after .")
			}
			expr = MemberExpr{X: expr, Name: token.value}
		case tokenLBracket:
			parser.consume()
			index, err := parser.parseOr()
			if err != nil {
				return nil, err
			}
			token := parser.consume()
			if token.kind != tokenRBracket {
				if token.value != "" {
					return nil, fmt.Errorf("missing closing ], got %q", token.value)
				}
				return nil, fmt.Errorf("missing closing ]")
			}
			expr = IndexExpr{X: expr, Index: index}
		case tokenLParen:
			name, ok := expr.(IdentExpr)
			if !ok {
				return nil, fmt.Errorf("only helper names can be called")
			}
			args, err := parser.parseCallArgs()
			if err != nil {
				return nil, err
			}
			expr = CallExpr{Name: name.Name, Args: args}
		default:
			return expr, nil
		}
	}
}

func (parser *exprParser) parseCallArgs() ([]Expr, error) {
	if token := parser.consume(); token.kind != tokenLParen {
		return nil, parser.expected("opening ( for helper call", token)
	}
	if parser.peek().kind == tokenRParen {
		parser.consume()
		return nil, nil
	}
	var args []Expr
	for {
		arg, err := parser.parseConditional()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		token := parser.consume()
		switch token.kind {
		case tokenComma:
			continue
		case tokenRParen:
			return args, nil
		default:
			if token.value != "" {
				return nil, fmt.Errorf("expected , or ) in helper call, got %q", token.value)
			}
			return nil, fmt.Errorf("expected , or ) in helper call")
		}
	}
}

func (parser *exprParser) parsePrimary() (Expr, error) {
	token := parser.consume()
	switch token.kind {
	case tokenIdent:
		return IdentExpr{Name: token.value}, nil
	case tokenString:
		return LiteralExpr{Type: TypeString, Value: token.value}, nil
	case tokenNumber:
		if strings.Contains(token.value, ".") {
			return LiteralExpr{Type: TypeFloat, Value: token.value}, nil
		}
		return LiteralExpr{Type: TypeInt, Value: token.value}, nil
	case tokenBool:
		return LiteralExpr{Type: TypeBool, Value: token.value}, nil
	case tokenNil:
		return LiteralExpr{Type: TypeNil, Value: token.value}, nil
	case tokenLParen:
		expr, err := parser.parseOr()
		if err != nil {
			return nil, err
		}
		if token := parser.consume(); token.kind != tokenRParen {
			if token.value != "" {
				return nil, fmt.Errorf("missing closing ), got %q", token.value)
			}
			return nil, fmt.Errorf("missing closing )")
		}
		return expr, nil
	default:
		if token.value != "" {
			return nil, fmt.Errorf("unexpected token %q", token.value)
		}
		return nil, fmt.Errorf("unexpected end of expression")
	}
}

func (parser *exprParser) expected(message string, token exprToken) error {
	if token.value != "" {
		return fmt.Errorf("expected %s, got %q", message, token.value)
	}
	return fmt.Errorf("expected %s", message)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
