package clientlang

import (
	"errors"
	"fmt"
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

// ExprSourceSpan is a 1-based column span within the expression source. End is
// exclusive.
type ExprSourceSpan struct {
	StartColumn int
	EndColumn   int
}

// ExprValidationError wraps an expression validation failure with the source
// columns of the expression node that failed.
type ExprValidationError struct {
	Span ExprSourceSpan
	Err  error
}

func (err ExprValidationError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err ExprValidationError) Unwrap() error {
	return err.Err
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
	Span  ExprSourceSpan
}

func (LiteralExpr) exprNode() {}

// IdentExpr reads a state, prop, param, or local name.
type IdentExpr struct {
	Name string
	Span ExprSourceSpan
}

func (IdentExpr) exprNode() {}

// MemberExpr reads a field from an object expression.
type MemberExpr struct {
	X    Expr
	Name string
	Span ExprSourceSpan
}

func (MemberExpr) exprNode() {}

// IndexExpr reads an item from an array expression.
type IndexExpr struct {
	X     Expr
	Index Expr
	Span  ExprSourceSpan
}

func (IndexExpr) exprNode() {}

// CallExpr invokes a component-local helper function.
type CallExpr struct {
	Name string
	Args []Expr
	Span ExprSourceSpan
}

func (CallExpr) exprNode() {}

// UnaryExpr applies a unary operator.
type UnaryExpr struct {
	Op   string
	X    Expr
	Span ExprSourceSpan
}

func (UnaryExpr) exprNode() {}

// BinaryExpr applies a binary operator.
type BinaryExpr struct {
	Op          string
	Left, Right Expr
	Span        ExprSourceSpan
}

func (BinaryExpr) exprNode() {}

// ConditionalExpr chooses between two expressions from a bool condition.
type ConditionalExpr struct {
	Cond Expr
	Then Expr
	Else Expr
	Span ExprSourceSpan
}

func (ConditionalExpr) exprNode() {}

// ParseExpr parses the supported client expression subset.
func ParseExpr(source string) (Expr, error) {
	return ParseExprWithSpans(source)
}

// ParseExprWithSpans parses the supported client expression subset and records
// 1-based source columns on every expression node.
func ParseExprWithSpans(source string) (Expr, error) {
	parser := exprParser{lexer: newExprLexer(source)}
	expr, err := parser.parseConditional()
	if err != nil {
		if parser.err != nil {
			return nil, parser.err
		}
		return nil, err
	}
	if parser.peek().kind != tokenEOF {
		if parser.err != nil {
			return nil, parser.err
		}
		return nil, fmt.Errorf("unexpected token %q", parser.peek().value)
	}
	return expr, nil
}

// ExprSpan returns the source span recorded for expr.
func ExprSpan(expr Expr) ExprSourceSpan {
	switch typed := expr.(type) {
	case LiteralExpr:
		return typed.Span
	case IdentExpr:
		return typed.Span
	case MemberExpr:
		return typed.Span
	case IndexExpr:
		return typed.Span
	case CallExpr:
		return typed.Span
	case UnaryExpr:
		return typed.Span
	case BinaryExpr:
		return typed.Span
	case ConditionalExpr:
		return typed.Span
	default:
		return ExprSourceSpan{}
	}
}

func tokenSpan(token exprToken) ExprSourceSpan {
	return ExprSourceSpan{StartColumn: token.start + 1, EndColumn: token.end + 1}
}

func mergeExprSpans(left, right ExprSourceSpan) ExprSourceSpan {
	if left.StartColumn == 0 {
		return right
	}
	if right.StartColumn == 0 {
		return left
	}
	return ExprSourceSpan{StartColumn: left.StartColumn, EndColumn: right.EndColumn}
}

func wrapExprError(expr Expr, err error) error {
	if err == nil {
		return nil
	}
	var validation ExprValidationError
	if errors.As(err, &validation) {
		return err
	}
	return ExprValidationError{Span: ExprSpan(expr), Err: err}
}
