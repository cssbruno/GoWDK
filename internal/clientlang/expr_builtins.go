package clientlang

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

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
	case "lower", "upper":
		if err := checkStringBuiltinArgs(expr, symbols, functions, fields, 1); err != nil {
			return TypeUnknown, true, err
		}
		return TypeString, true, nil
	case "contains":
		if err := checkStringBuiltinArgs(expr, symbols, functions, fields, 2); err != nil {
			return TypeUnknown, true, err
		}
		return TypeBool, true, nil
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

func checkStringBuiltinArgs(expr CallExpr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool, count int) error {
	if len(expr.Args) != count {
		return fmt.Errorf("built-in %s expects %d argument%s, got %d", expr.Name, count, plural(count), len(expr.Args))
	}
	for index, arg := range expr.Args {
		actual, err := checkExpr(arg, symbols, functions, fields)
		if err != nil {
			return err
		}
		if actual != TypeUnknown && actual != TypeString {
			return fmt.Errorf("built-in %s argument %d expects string, got %s", expr.Name, index+1, actual)
		}
	}
	return nil
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
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
	case "lower":
		return evalCaseBuiltin(expr, values, strings.ToLower)
	case "upper":
		return evalCaseBuiltin(expr, values, strings.ToUpper)
	case "contains":
		return evalContainsBuiltin(expr, values)
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

func evalCaseBuiltin(expr CallExpr, values map[string]string, fn func(string) string) (any, bool, error) {
	if len(expr.Args) != 1 {
		return nil, true, fmt.Errorf("built-in %s expects 1 argument, got %d", expr.Name, len(expr.Args))
	}
	value, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return nil, true, err
	}
	typed, ok := value.(string)
	if !ok {
		return nil, true, fmt.Errorf("built-in %s expects string", expr.Name)
	}
	return fn(typed), true, nil
}

func evalContainsBuiltin(expr CallExpr, values map[string]string) (any, bool, error) {
	if len(expr.Args) != 2 {
		return nil, true, fmt.Errorf("built-in contains expects 2 arguments, got %d", len(expr.Args))
	}
	haystack, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return nil, true, err
	}
	needle, err := evalExpr(expr.Args[1], values)
	if err != nil {
		return nil, true, err
	}
	haystackString, ok := haystack.(string)
	if !ok {
		return nil, true, fmt.Errorf("built-in contains argument 1 expects string")
	}
	needleString, ok := needle.(string)
	if !ok {
		return nil, true, fmt.Errorf("built-in contains argument 2 expects string")
	}
	return strings.Contains(haystackString, needleString), true, nil
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
