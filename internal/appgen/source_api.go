package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func apiHandlerSource(apis []APIEndpoint) string {
	return printActionDecls([]ast.Decl{apiFuncDecl(sortedAPIEndpoints(apis))})
}

func apiFuncDecl(apis []APIEndpoint) *ast.FuncDecl {
	if len(apis) == 0 {
		return funcDecl("api", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
	}
	var clauses []ast.Stmt
	for _, api := range apis {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{apiCaseExpr(api)},
			Body: apiCaseStmts(api),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{returnBool(false)}})
	return funcDecl("api", actionParams(), boolResults(), []ast.Stmt{
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

func apiCaseExpr(api APIEndpoint) ast.Expr {
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

func apiCaseStmts(api APIEndpoint) []ast.Stmt {
	if api.Binding.Status != manifest.BackendBindingBound {
		return append(backendNotImplementedStmts(api.Binding, "API"), returnBool(true))
	}
	return []ast.Stmt{
		endpointContextStmt("api", api.PageID, api.APIName, api.Method, api.Route),
		define([]ast.Expr{id("result"), id("err")}, call(sel(api.BackendAlias, api.Binding.FunctionName), id("ctx"), id("request"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreErrorExprStmt(call(sel("gowdkresponse", "HandlerStatus"), id("err"), sel("http", "StatusInternalServerError")), call(selExpr(id("err"), "Error"))),
				returnBool(true),
			),
		},
		writeNoStoreHTTPStmt(id("result")),
		returnBool(true),
	}
}
