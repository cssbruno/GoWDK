package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk"
)

func generatedUsesRateLimit(options Options) bool {
	return options.Config.HasFeature(gowdk.FeatureRateLimit) && (len(options.Actions) > 0 || len(options.APIs) > 0 || len(options.Fragments) > 0 || len(options.SSR) > 0)
}

func rateLimitDecls(options Options) []ast.Decl {
	if !generatedUsesRateLimit(options) {
		return nil
	}
	return []ast.Decl{
		rateLimiterVarDecl(),
		registerRateLimiterDecl(),
		runRateLimitDecl(),
	}
}

func rateLimiterVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("rateLimiter")},
		Type:  &ast.StarExpr{X: sel("gowdkratelimit", "Limiter")},
	}}}
}

func registerRateLimiterDecl() ast.Decl {
	return funcDecl("RegisterRateLimiter", []*ast.Field{
		{Names: []*ast.Ident{id("limiter")}, Type: &ast.StarExpr{X: sel("gowdkratelimit", "Limiter")}},
	}, nil, []ast.Stmt{
		assign([]ast.Expr{id("rateLimiter")}, id("limiter")),
	})
}

func runRateLimitDecl() ast.Decl {
	return funcDecl("runRateLimit", actionParams(), boolResults(), []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("rateLimiter"), Op: token.EQL, Y: id("nil")},
			Body: block(returnBool(false)),
		},
		define([]ast.Expr{id("result"), id("err")}, call(selExpr(id("rateLimiter"), "AllowRequest"), id("request"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				exprStmt(call(sel("gowdkratelimit", "DefaultErrorHandler"), id("response"), id("request"), id("err"))),
				returnBool(true),
			),
		},
		exprStmt(call(sel("gowdkratelimit", "WriteHeaders"), id("response"), id("result"))),
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: selExpr(id("result"), "Allowed")},
			Body: block(
				exprStmt(call(sel("gowdkratelimit", "DefaultLimitHandler"), id("response"), id("request"), id("result"))),
				returnBool(true),
			),
		},
		returnBool(false),
	})
}

func rateLimitStmts(enabled bool) []ast.Stmt {
	if !enabled {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Cond: call(id("runRateLimit"), id("response"), id("request")),
		Body: block(returnBool(true)),
	}}
}
