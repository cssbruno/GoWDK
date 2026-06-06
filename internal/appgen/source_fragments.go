package appgen

import (
	"go/ast"
	"go/token"
	"sort"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func fragmentFuncDecl(fragments []FragmentEndpoint, rateLimit bool) *ast.FuncDecl {
	if len(fragments) == 0 {
		return funcDecl("fragment", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
	}
	var clauses []ast.Stmt
	for _, fragment := range sortedFragmentEndpoints(fragments) {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{fragmentCaseExpr(fragment)},
			Body: fragmentCaseStmts(fragment, rateLimit),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{returnBool(false)}})
	return funcDecl("fragment", actionParams(), boolResults(), []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(sel("path", "Clean"), &ast.BinaryExpr{
			X:  stringLit("/"),
			Op: token.ADD,
			Y:  selExpr(selExpr(id("request"), "URL"), "Path"),
		})),
		&ast.SwitchStmt{Body: &ast.BlockStmt{List: clauses}},
	})
}

func fragmentCaseExpr(fragment FragmentEndpoint) ast.Expr {
	return &ast.BinaryExpr{
		X: &ast.BinaryExpr{
			X:  selExpr(id("request"), "Method"),
			Op: token.EQL,
			Y:  stringLit(fragment.Method),
		},
		Op: token.LAND,
		Y: &ast.BinaryExpr{
			X:  id("requestPath"),
			Op: token.EQL,
			Y:  stringLit(cleanRoutePath(fragment.Route)),
		},
	}
}

func fragmentCaseStmts(fragment FragmentEndpoint, rateLimit bool) []ast.Stmt {
	stmts := endpointContextStmts("fragment", fragment.PageID, fragment.FragmentName, fragment.Method, fragment.Route, "")
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(fragment.Guards)...)
	if fragment.Binding.Status == manifest.BackendBindingUnsupportedSignature {
		stmts = append(stmts, backendNotImplementedStmts(fragment.Binding, "fragment")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	if fragment.Binding.Status == manifest.BackendBindingBound {
		stmts = append(stmts,
			define([]ast.Expr{id("result"), id("err")}, call(sel(fragment.BackendAlias, fragment.Binding.FunctionName), id("ctx"))),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(
					writeNoStoreErrorExprStmt(call(sel("gowdkresponse", "HandlerStatus"), id("err"), sel("http", "StatusInternalServerError")), call(selExpr(id("err"), "Error"))),
					returnBool(true),
				),
			},
			writeNoStoreHTTPStmt(id("result")),
			returnBool(true),
		)
		return stmts
	}
	stmts = append(stmts,
		define([]ast.Expr{id("fragment")}, call(sel("gowdkresponse", "FragmentFor"), stringLit(fragment.Target), stringLit(fragment.HTML))),
		writeNoStoreHTTPStmt(id("fragment")),
		returnBool(true),
	)
	return stmts
}

func sortedFragmentEndpoints(fragments []FragmentEndpoint) []FragmentEndpoint {
	sorted := append([]FragmentEndpoint(nil), fragments...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			if sorted[i].Method == sorted[j].Method {
				return sorted[i].FragmentName < sorted[j].FragmentName
			}
			return sorted[i].Method < sorted[j].Method
		}
		return sorted[i].Route < sorted[j].Route
	})
	return sorted
}
