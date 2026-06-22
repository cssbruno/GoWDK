package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func apiHandlerSource(apis []APIEndpoint) (source string, err error) {
	defer recoverGeneratedIdentifierError(&err)

	return printActionDecls([]ast.Decl{apiFuncDecl(backendAdapterIR(Options{APIs: apis}).APIs, false, false)})
}

func apiFuncDecl(apis []BackendAPIAdapter, csrf bool, rateLimit bool) *ast.FuncDecl {
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
			Body: apiCaseStmts(api, csrf && gwdkir.HTTPMethodRequiresCSRF(api.Method), rateLimit),
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

func apiCaseStmts(api BackendAPIAdapter, csrf bool, rateLimit bool) []ast.Stmt {
	if endpointDeniedByOmission(api.Guards) {
		return denyByOmissionStmts()
	}
	stmts := endpointContextStmts("api", api.PageID, api.APIName, api.Method, api.Route, api.ErrorPage)
	if api.ErrorPage != "" {
		stmts = append(stmts, endpointPanicBoundaryStmt())
	}
	stmts = append(stmts, apiBodyLimitStmt())
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(api.Guards)...)
	if api.Binding.Status != source.BackendBindingBound {
		stmts = append(stmts, backendNotImplementedStmts(api.Binding, "API")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	stmts = append(stmts, apiCSRFStmts(csrf)...)
	stmts = append(stmts,
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

func apiBodyLimitStmt() ast.Stmt {
	return assign([]ast.Expr{selExpr(id("request"), "Body")}, call(sel("http", "MaxBytesReader"), id("response"), selExpr(id("request"), "Body"), id("maxAPIBodyBytes")))
}

func apiCSRFStmts(csrf bool) []ast.Stmt {
	if !csrf {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Cond: notNil("csrfValidator"),
		Body: block(&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(selExpr(id("csrfValidator"), "Validate"), id("request"))),
			Cond: notNil("err"),
			Body: block(
				writeNoStoreJSONErrorStmt(sel("http", "StatusForbidden"), "invalid csrf token"),
				returnBool(true),
			),
		}),
	}}
}

func apisUseErrorPages(apis []BackendAPIAdapter) bool {
	for _, api := range apis {
		if api.ErrorPage != "" {
			return true
		}
	}
	return false
}
