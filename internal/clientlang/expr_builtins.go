package clientlang

import (
	"encoding/json"
	"fmt"
	"math"
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
	case "fixed", "percent":
		if err := checkNumberDigitsBuiltinArgs(expr, symbols, functions, fields); err != nil {
			return TypeUnknown, true, err
		}
		return TypeString, true, nil
	case "round":
		if err := checkNumberDigitsBuiltinArgs(expr, symbols, functions, fields); err != nil {
			return TypeUnknown, true, err
		}
		return TypeFloat, true, nil
	case "formatTime":
		if err := checkFormatTimeBuiltinArgs(expr, symbols, functions, fields); err != nil {
			return TypeUnknown, true, err
		}
		return TypeString, true, nil
	default:
		return TypeUnknown, false, nil
	}
}

// checkNumberDigitsBuiltinArgs validates the (number, digits) signature shared by
// fixed, round, and percent. The digit count's value range is enforced at
// evaluation time because it is not known at check time.
func checkNumberDigitsBuiltinArgs(expr CallExpr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool) error {
	if len(expr.Args) != 2 {
		return fmt.Errorf("built-in %s expects 2 arguments, got %d", expr.Name, len(expr.Args))
	}
	value, err := checkExpr(expr.Args[0], symbols, functions, fields)
	if err != nil {
		return err
	}
	if value != TypeUnknown && !isNumericType(value) {
		return fmt.Errorf("built-in %s argument 1 expects a number, got %s", expr.Name, value)
	}
	digits, err := checkExpr(expr.Args[1], symbols, functions, fields)
	if err != nil {
		return err
	}
	if digits != TypeUnknown && digits != TypeInt {
		return fmt.Errorf("built-in %s argument 2 expects an int, got %s", expr.Name, digits)
	}
	return nil
}

func checkFormatTimeBuiltinArgs(expr CallExpr, symbols map[string]ValueType, functions map[string]ExprFunction, fields map[string]bool) error {
	if len(expr.Args) != 2 {
		return fmt.Errorf("built-in formatTime expects 2 arguments, got %d", len(expr.Args))
	}
	timestamp, err := checkExpr(expr.Args[0], symbols, functions, fields)
	if err != nil {
		return err
	}
	if timestamp != TypeUnknown && !isNumericType(timestamp) {
		return fmt.Errorf("built-in formatTime argument 1 expects a unix timestamp, got %s", timestamp)
	}
	layout, err := checkExpr(expr.Args[1], symbols, functions, fields)
	if err != nil {
		return err
	}
	if layout != TypeUnknown && layout != TypeString {
		return fmt.Errorf("built-in formatTime argument 2 expects a string layout, got %s", layout)
	}
	return nil
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
	case "fixed":
		number, digits, err := evalNumberDigitsArgs(expr, values)
		if err != nil {
			return nil, true, err
		}
		formatted, err := formatFixed(number, digits)
		return formatted, true, err
	case "round":
		number, digits, err := evalNumberDigitsArgs(expr, values)
		if err != nil {
			return nil, true, err
		}
		rounded, err := roundTo(number, digits)
		return rounded, true, err
	case "percent":
		number, digits, err := evalNumberDigitsArgs(expr, values)
		if err != nil {
			return nil, true, err
		}
		formatted, err := formatPercent(number, digits)
		return formatted, true, err
	case "formatTime":
		return evalFormatTimeBuiltin(expr, values)
	default:
		return nil, false, nil
	}
}

func evalNumberDigitsArgs(expr CallExpr, values map[string]string) (float64, int, error) {
	if len(expr.Args) != 2 {
		return 0, 0, fmt.Errorf("built-in %s expects 2 arguments, got %d", expr.Name, len(expr.Args))
	}
	first, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return 0, 0, err
	}
	number, ok := numericFloat(first)
	if !ok {
		return 0, 0, fmt.Errorf("built-in %s argument 1 expects a number", expr.Name)
	}
	second, err := evalExpr(expr.Args[1], values)
	if err != nil {
		return 0, 0, err
	}
	digits, ok := numericInt(second)
	if !ok {
		return 0, 0, fmt.Errorf("built-in %s argument 2 expects an int", expr.Name)
	}
	return number, digits, nil
}

func evalFormatTimeBuiltin(expr CallExpr, values map[string]string) (any, bool, error) {
	if len(expr.Args) != 2 {
		return nil, true, fmt.Errorf("built-in formatTime expects 2 arguments, got %d", len(expr.Args))
	}
	first, err := evalExpr(expr.Args[0], values)
	if err != nil {
		return nil, true, err
	}
	timestamp, ok := numericFloat(first)
	if !ok {
		return nil, true, fmt.Errorf("built-in formatTime argument 1 expects a unix timestamp")
	}
	second, err := evalExpr(expr.Args[1], values)
	if err != nil {
		return nil, true, err
	}
	layout, ok := second.(string)
	if !ok {
		return nil, true, fmt.Errorf("built-in formatTime argument 2 expects a string layout")
	}
	formatted, err := formatUnixTime(timestamp, layout)
	return formatted, true, err
}

// numericInt coerces a numeric runtime value to an int, rejecting fractional
// numbers so digit counts and timestamps stay whole.
func numericInt(value any) (int, bool) {
	number, ok := numericFloat(value)
	if !ok || number != math.Trunc(number) {
		return 0, false
	}
	return int(number), true
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
