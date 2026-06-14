package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/internal/source"
)

func apiHandlerSource(apis []APIEndpoint) (string, error) {
	return printActionDecls([]ast.Decl{apiFuncDecl(backendAdapterIR(Options{APIs: apis}).APIs, false)})
}

func apiFuncDecl(apis []BackendAPIAdapter, rateLimit bool) *ast.FuncDecl {
	if len(apis) == 0 {
		return funcDecl("api", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
	}
	results := boolResults()
	if apisUseErrorPages(apis) {
		results = namedBoolResults("handled")
	}
	var clauses []ast.Stmt
	for _, api := range apis {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{apiCaseExpr(api)},
			Body: apiCaseStmts(api, rateLimit),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{returnBool(false)}})
	return funcDecl("api", actionParams(), results, []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(sel("path", "Clean"), &ast.BinaryExpr{
			X:  stringLit("/"),
			Op: token.ADD,
			Y:  selExpr(selExpr(id("request"), "URL"), "Path"),
		})),
		&ast.SwitchStmt{
			Body: &ast.BlockStmt{List: clauses},
		},
	})
}

func apiCaseExpr(api BackendAPIAdapter) ast.Expr {
	return &ast.BinaryExpr{
		X: &ast.BinaryExpr{
			X:  selExpr(id("request"), "Method"),
			Op: token.EQL,
			Y:  stringLit(api.Method),
		},
		Op: token.LAND,
		Y: &ast.BinaryExpr{
			X:  id("requestPath"),
			Op: token.EQL,
			Y:  stringLit(cleanRoutePath(api.Route)),
		},
	}
}

func apiCaseStmts(api BackendAPIAdapter, rateLimit bool) []ast.Stmt {
	stmts := endpointContextStmts("api", api.PageID, api.APIName, api.Method, api.Route, api.ErrorPage)
	if api.ErrorPage != "" {
		stmts = append(stmts, endpointPanicBoundaryStmt())
	}
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(api.Guards)...)
	if api.Binding.Status != source.BackendBindingBound {
		stmts = append(stmts, backendNotImplementedStmts(api.Binding, "API")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	stmts = append(stmts,
		assign([]ast.Expr{selExpr(id("request"), "Body")}, call(sel("http", "MaxBytesReader"), id("response"), selExpr(id("request"), "Body"), id("maxAPIBodyBytes"))),
		define([]ast.Expr{id("result"), id("err")}, call(sel(api.BackendAlias, api.Binding.FunctionName), id("ctx"), id("request"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
		writeNoStoreHTTPStmt(id("result")),
		returnBool(true),
	)
	return stmts
}

func apisUseErrorPages(apis []BackendAPIAdapter) bool {
	for _, api := range apis {
		if api.ErrorPage != "" {
			return true
		}
	}
	return false
}
