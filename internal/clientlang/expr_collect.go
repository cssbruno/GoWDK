package clientlang

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
	case SwitchExpr:
		collectExprFields(typed.Value, fields)
		for _, item := range typed.Cases {
			collectExprFields(item.Match, fields)
			collectExprFields(item.Value, fields)
		}
		collectExprFields(typed.Default, fields)
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
	case SwitchExpr:
		collectExprCalls(typed.Value, calls)
		for _, item := range typed.Cases {
			collectExprCalls(item.Match, calls)
			collectExprCalls(item.Value, calls)
		}
		collectExprCalls(typed.Default, calls)
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
