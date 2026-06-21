package clientlang

import "fmt"

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
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("unknown client value %q", typed.Name))
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
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("cannot read field %q from %s expression", typed.Name, base))
		}
		if path != "" {
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("unknown client value %q", path))
		}
		return TypeUnknown, wrapExprError(typed, fmt.Errorf("unknown client field %q", typed.Name))
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
			return TypeUnknown, wrapExprError(typed.Index, fmt.Errorf("index expression requires int, got %s", index))
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
			return TypeUnknown, wrapExprError(typed.X, fmt.Errorf("cannot index %s expression", base))
		}
		if path != "" {
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("unknown client value %q", path))
		}
		return TypeUnknown, nil
	case CallExpr:
		if typ, ok, err := checkBuiltinCall(typed, symbols, functions, fields); ok || err != nil {
			return typ, wrapExprError(typed, err)
		}
		function, ok := functions[typed.Name]
		if !ok {
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("unknown client helper function %q", typed.Name))
		}
		if len(typed.Args) != len(function.Params) {
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("client helper function %s expects %d arguments, got %d", typed.Name, len(function.Params), len(typed.Args)))
		}
		for index, arg := range typed.Args {
			actual, err := checkExpr(arg, symbols, functions, fields)
			if err != nil {
				return TypeUnknown, err
			}
			expected := function.Params[index]
			if expected != TypeUnknown && actual != TypeUnknown && expected != actual && !compatibleNumericType(actual, expected) {
				return TypeUnknown, wrapExprError(arg, fmt.Errorf("client helper function %s argument %d expects %s, got %s", typed.Name, index+1, expected, actual))
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
				return TypeUnknown, wrapExprError(typed.X, fmt.Errorf("operator ! requires bool, got %s", typ))
			}
			return TypeBool, nil
		case "-":
			if typ == TypeUnknown {
				return TypeUnknown, nil
			}
			if !isNumericType(typ) {
				return TypeUnknown, wrapExprError(typed.X, fmt.Errorf("operator - requires number, got %s", typ))
			}
			return typ, nil
		default:
			return TypeUnknown, wrapExprError(typed, fmt.Errorf("unsupported unary operator %q", typed.Op))
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
		typ, err := checkBinaryExpr(typed.Op, left, right)
		return typ, wrapExprError(typed, err)
	case ConditionalExpr:
		cond, err := checkExpr(typed.Cond, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		if cond != TypeUnknown && cond != TypeBool {
			return TypeUnknown, wrapExprError(typed.Cond, fmt.Errorf("if expression condition requires bool, got %s", cond))
		}
		thenType, err := checkExpr(typed.Then, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		elseType, err := checkExpr(typed.Else, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		typ, err := checkConditionalBranches(thenType, elseType)
		return typ, wrapExprError(typed, err)
	case SwitchExpr:
		valueType, err := checkExpr(typed.Value, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		var branchType ValueType
		branchSeen := false
		for _, item := range typed.Cases {
			matchType, err := checkExpr(item.Match, symbols, functions, fields)
			if err != nil {
				return TypeUnknown, err
			}
			if err := checkSwitchCaseType(valueType, matchType); err != nil {
				return TypeUnknown, wrapExprError(item.Match, err)
			}
			caseType, err := checkExpr(item.Value, symbols, functions, fields)
			if err != nil {
				return TypeUnknown, err
			}
			if !branchSeen {
				branchType = caseType
				branchSeen = true
			} else {
				branchType, err = mergeBranchTypes(branchType, caseType, typed.Keyword)
				if err != nil {
					return TypeUnknown, wrapExprError(item.Value, err)
				}
			}
		}
		defaultType, err := checkExpr(typed.Default, symbols, functions, fields)
		if err != nil {
			return TypeUnknown, err
		}
		typ, err := mergeBranchTypes(branchType, defaultType, typed.Keyword)
		return typ, wrapExprError(typed, err)
	default:
		return TypeUnknown, fmt.Errorf("unknown expression node")
	}
}

func checkConditionalBranches(thenType, elseType ValueType) (ValueType, error) {
	return mergeBranchTypes(thenType, elseType, "if")
}

func mergeBranchTypes(left, right ValueType, label string) (ValueType, error) {
	if left == TypeUnknown || right == TypeUnknown {
		return TypeUnknown, nil
	}
	if left == right {
		return left, nil
	}
	if compatibleNumericType(left, right) {
		if left == TypeFloat || right == TypeFloat {
			return TypeFloat, nil
		}
		return TypeInt, nil
	}
	if left == TypeNil || right == TypeNil {
		return TypeUnknown, fmt.Errorf("nil is only supported in == or != scalar comparisons")
	}
	return TypeUnknown, fmt.Errorf("%s expression branches must have matching types, got %s and %s", label, left, right)
}

func checkSwitchCaseType(valueType, matchType ValueType) error {
	if valueType == TypeUnknown || matchType == TypeUnknown {
		return nil
	}
	if valueType == TypeNil || matchType == TypeNil {
		other := valueType
		if valueType == TypeNil {
			other = matchType
		}
		if other == TypeNil || other == TypeArray || other == TypeObject {
			return fmt.Errorf("switch case supports nil only with scalar values")
		}
		return nil
	}
	if valueType == matchType || compatibleNumericType(valueType, matchType) {
		return nil
	}
	return fmt.Errorf("switch case requires comparable matching types, got %s and %s", valueType, matchType)
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
		if left == TypeNil || right == TypeNil {
			other := right
			if right == TypeNil {
				other = left
			}
			if other == TypeNil || other == TypeArray || other == TypeObject {
				return TypeUnknown, fmt.Errorf("operator %s supports nil only with scalar values", op)
			}
			return TypeBool, nil
		}
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
