package clientlang

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

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
			if int(rightNumber) == 0 {
				return nil, fmt.Errorf("operator %% requires a non-zero divisor")
			}
			return float64(int(leftNumber) % int(rightNumber)), nil
		}
	case "==":
		return valuesEqual(left, right), nil
	case "!=":
		return !valuesEqual(left, right), nil
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

// valuesEqual compares operands of == and != with numeric normalization so
// values decoded from JSON (json.Number) compare equal to int and float64
// literals, matching the strict numeric equality of the browser evaluator.
func valuesEqual(left, right any) bool {
	leftNumber, leftOK := numericFloat(left)
	rightNumber, rightOK := numericFloat(right)
	if leftOK && rightOK {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
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
