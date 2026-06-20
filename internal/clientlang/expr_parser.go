package clientlang

import (
	"fmt"
	"strings"
)

type exprParser struct {
	lexer  *exprLexer
	buffer *exprToken
	err    error
}

func (parser *exprParser) peek() exprToken {
	if parser.buffer != nil {
		return *parser.buffer
	}
	token, err := parser.lexer.next()
	if err != nil {
		if parser.err == nil {
			parser.err = err
		}
		token = exprToken{kind: tokenError}
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
	start := parser.consume()
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
	end := parser.consume()
	if end.kind != tokenRBrace {
		token := end
		return nil, parser.expected("closing } after else branch", token)
	}
	return ConditionalExpr{Cond: cond, Then: thenExpr, Else: elseExpr, Span: mergeExprSpans(tokenSpan(start), tokenSpan(end))}, nil
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
		left = BinaryExpr{Op: op, Left: left, Right: right, Span: mergeExprSpans(ExprSpan(left), ExprSpan(right))}
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
		return UnaryExpr{Op: token.value, X: expr, Span: mergeExprSpans(tokenSpan(token), ExprSpan(expr))}, nil
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
			expr = MemberExpr{X: expr, Name: token.value, Span: mergeExprSpans(ExprSpan(expr), tokenSpan(token))}
		case tokenLBracket:
			parser.consume()
			index, err := parser.parseConditional()
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
			expr = IndexExpr{X: expr, Index: index, Span: mergeExprSpans(ExprSpan(expr), tokenSpan(token))}
		case tokenLParen:
			name, ok := expr.(IdentExpr)
			if !ok {
				return nil, fmt.Errorf("only helper names can be called")
			}
			args, close, err := parser.parseCallArgs()
			if err != nil {
				return nil, err
			}
			expr = CallExpr{Name: name.Name, Args: args, Span: mergeExprSpans(ExprSpan(name), tokenSpan(close))}
		default:
			return expr, nil
		}
	}
}

func (parser *exprParser) parseCallArgs() ([]Expr, exprToken, error) {
	if token := parser.consume(); token.kind != tokenLParen {
		return nil, exprToken{}, parser.expected("opening ( for helper call", token)
	}
	if parser.peek().kind == tokenRParen {
		close := parser.consume()
		return nil, close, nil
	}
	var args []Expr
	for {
		arg, err := parser.parseConditional()
		if err != nil {
			return nil, exprToken{}, err
		}
		args = append(args, arg)
		token := parser.consume()
		switch token.kind {
		case tokenComma:
			continue
		case tokenRParen:
			return args, token, nil
		default:
			if token.value != "" {
				return nil, exprToken{}, fmt.Errorf("expected , or ) in helper call, got %q", token.value)
			}
			return nil, exprToken{}, fmt.Errorf("expected , or ) in helper call")
		}
	}
}

func (parser *exprParser) parsePrimary() (Expr, error) {
	token := parser.consume()
	switch token.kind {
	case tokenIdent:
		return IdentExpr{Name: token.value, Span: tokenSpan(token)}, nil
	case tokenString:
		return LiteralExpr{Type: TypeString, Value: token.value, Span: tokenSpan(token)}, nil
	case tokenNumber:
		if strings.Contains(token.value, ".") {
			return LiteralExpr{Type: TypeFloat, Value: token.value, Span: tokenSpan(token)}, nil
		}
		return LiteralExpr{Type: TypeInt, Value: token.value, Span: tokenSpan(token)}, nil
	case tokenBool:
		return LiteralExpr{Type: TypeBool, Value: token.value, Span: tokenSpan(token)}, nil
	case tokenNil:
		return LiteralExpr{Type: TypeNil, Value: token.value, Span: tokenSpan(token)}, nil
	case tokenLParen:
		expr, err := parser.parseConditional()
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
