package appgen

import (
	"go/ast"
	"go/token"
	"sort"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func fragmentFuncDecl(fragments []FragmentEndpoint, rateLimit bool) *ast.FuncDecl {
	if len(fragments) == 0 {
		return funcDecl("fragment", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
	}
	sorted := sortedFragmentEndpoints(fragments)
	body := []ast.Stmt{}
	var clauses []ast.Stmt
	for _, fragment := range sorted {
		if fragmentRouteIsDynamic(fragment) {
			continue
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{fragmentCaseExpr(fragment)},
			Body: fragmentCaseStmts(fragment, rateLimit),
		})
	}
	if len(clauses) > 0 {
		body = append(body,
			define([]ast.Expr{id("requestPath")}, call(sel("path", "Clean"), &ast.BinaryExpr{
				X:  stringLit("/"),
				Op: token.ADD,
				Y:  selExpr(selExpr(id("request"), "URL"), "Path"),
			})),
			&ast.SwitchStmt{Body: &ast.BlockStmt{List: clauses}},
		)
	}
	for _, fragment := range sorted {
		if !fragmentRouteIsDynamic(fragment) {
			continue
		}
		body = append(body, fragmentDynamicIfStmt(fragment, rateLimit))
	}
	body = append(body, returnBool(false))
	return funcDecl("fragment", actionParams(), boolResults(), body)
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
	stmts := fragmentContextStmts(fragment, false)
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(fragment.Guards)...)
	if fragment.Binding.Status == source.BackendBindingUnsupportedSignature {
		stmts = append(stmts, backendNotImplementedStmts(fragment.Binding, "fragment")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	if fragment.Binding.Status == source.BackendBindingBound {
		stmts = append(stmts,
			define([]ast.Expr{id("result"), id("err")}, call(sel(fragment.BackendAlias, fragment.Binding.FunctionName), id("ctx"))),
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
	stmts = append(stmts,
		define([]ast.Expr{id("fragment")}, call(sel("gowdkpartial", "Fragment"), stringLit(fragment.Target), stringLit(fragment.HTML))),
		writeNoStoreHTTPStmt(id("fragment")),
		returnBool(true),
	)
	return stmts
}

func fragmentDynamicIfStmt(fragment FragmentEndpoint, rateLimit bool) ast.Stmt {
	return &ast.IfStmt{
		Init: define([]ast.Expr{id("params"), id("ok")}, call(sel("gowdkroute", "Match"), stringLit(fragment.Route), selExpr(selExpr(id("request"), "URL"), "Path"))),
		Cond: &ast.BinaryExpr{
			X: &ast.BinaryExpr{
				X:  selExpr(id("request"), "Method"),
				Op: token.EQL,
				Y:  stringLit(fragment.Method),
			},
			Op: token.LAND,
			Y:  id("ok"),
		},
		Body: block(fragmentDynamicCaseStmts(fragment, rateLimit)...),
	}
}

func fragmentDynamicCaseStmts(fragment FragmentEndpoint, rateLimit bool) []ast.Stmt {
	stmts := fragmentContextStmts(fragment, true)
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(fragment.Guards)...)
	if fragment.Binding.Status == source.BackendBindingUnsupportedSignature {
		stmts = append(stmts, backendNotImplementedStmts(fragment.Binding, "fragment")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	if fragment.Binding.Status == source.BackendBindingBound {
		stmts = append(stmts,
			define([]ast.Expr{id("result"), id("err")}, call(sel(fragment.BackendAlias, fragment.Binding.FunctionName), id("ctx"))),
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
	stmts = append(stmts,
		define([]ast.Expr{id("fragment")}, call(sel("gowdkpartial", "Fragment"), stringLit(fragment.Target), stringLit(fragment.HTML))),
		writeNoStoreHTTPStmt(id("fragment")),
		returnBool(true),
	)
	return stmts
}

func fragmentContextStmts(fragment FragmentEndpoint, includeParams bool) []ast.Stmt {
	stmts := []ast.Stmt{
		endpointContextStmt("fragment", fragment.PageID, fragment.FragmentName, fragment.Method, fragment.Route, ""),
	}
	if includeParams {
		stmts = append(stmts, assign([]ast.Expr{id("ctx")}, call(sel("gowdkruntime", "WithParams"), id("ctx"), id("params"))))
		stmts = append(stmts, typedRouteParamStmts(fragmentTypedRouteParams(fragment))...)
	}
	stmts = append(stmts, assign([]ast.Expr{id("request")}, call(selExpr(id("request"), "WithContext"), id("ctx"))))
	return stmts
}

func fragmentTypedRouteParams(fragment FragmentEndpoint) []source.RouteParam {
	if len(fragment.RouteParams) > 0 {
		return fragment.RouteParams
	}
	return gwdkir.RouteParamsFromPath(fragment.Route)
}

func fragmentsUseStaticFallback(fragments []FragmentEndpoint) bool {
	for _, fragment := range fragments {
		if fragment.Binding.Status != source.BackendBindingBound && fragment.Binding.Status != source.BackendBindingUnsupportedSignature {
			return true
		}
	}
	return false
}

func fragmentsUseExactRoutes(fragments []FragmentEndpoint) bool {
	for _, fragment := range fragments {
		if !fragmentRouteIsDynamic(fragment) {
			return true
		}
	}
	return false
}

func fragmentsUseDynamicRoutes(fragments []FragmentEndpoint) bool {
	for _, fragment := range fragments {
		if fragmentRouteIsDynamic(fragment) {
			return true
		}
	}
	return false
}

func fragmentRouteIsDynamic(fragment FragmentEndpoint) bool {
	return len(ssrRoutePatternParams(fragment.Route)) > 0
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
