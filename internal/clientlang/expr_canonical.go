package clientlang

import (
	"strconv"
	"strings"
)

// CanonicalExpr returns a deterministic representation of the supported
// expression subset. It is intended for compiler fingerprints, not for source
// rewriting.
func CanonicalExpr(source string) (string, error) {
	expr, err := ParseExpr(source)
	if err != nil {
		return "", err
	}
	return canonicalExpr(expr), nil
}

func canonicalExpr(expr Expr) string {
	switch typed := expr.(type) {
	case LiteralExpr:
		if typed.Type == TypeString {
			value, err := strconv.Unquote(typed.Value)
			if err == nil {
				return strconv.Quote(value)
			}
		}
		return typed.Value
	case IdentExpr:
		return typed.Name
	case MemberExpr:
		return canonicalExpr(typed.X) + "." + typed.Name
	case IndexExpr:
		return canonicalExpr(typed.X) + "[" + canonicalExpr(typed.Index) + "]"
	case CallExpr:
		args := make([]string, 0, len(typed.Args))
		for _, arg := range typed.Args {
			args = append(args, canonicalExpr(arg))
		}
		return typed.Name + "(" + strings.Join(args, ",") + ")"
	case UnaryExpr:
		return typed.Op + canonicalExpr(typed.X)
	case BinaryExpr:
		return "(" + canonicalExpr(typed.Left) + " " + typed.Op + " " + canonicalExpr(typed.Right) + ")"
	case ConditionalExpr:
		return "if " + canonicalExpr(typed.Cond) + " { " + canonicalExpr(typed.Then) + " } else { " + canonicalExpr(typed.Else) + " }"
	default:
		return ""
	}
}
