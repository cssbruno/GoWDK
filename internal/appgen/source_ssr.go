package appgen

import (
	"go/ast"
	"sort"
)

func ssrHandlerSource(routes []SSRRoute) string {
	sorted := sortedSSRRoutes(routes)
	return printActionDecls([]ast.Decl{
		ssrExactDecl(sorted),
		ssrDynamicDecl(sorted),
	})
}

func sortedSSRRoutes(routes []SSRRoute) []SSRRoute {
	sorted := append([]SSRRoute(nil), routes...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].PageID < sorted[j].PageID
		}
		return sorted[i].Route < sorted[j].Route
	})
	return sorted
}

func ssrExactDecl(routes []SSRRoute) *ast.FuncDecl {
	clauses := []ast.Stmt{}
	for _, route := range routes {
		if len(ssrRoutePatternParams(route.Route)) > 0 {
			continue
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{stringLit(route.Route)},
			Body: append(ssrRouteContextStmts(route, false), ssrWriteHTMLStmts(stringLit(route.HTML))...),
		})
	}
	return funcDecl("ssrExact", actionParams(), boolResults(), []ast.Stmt{
		&ast.SwitchStmt{
			Tag:  selExpr(selExpr(id("request"), "URL"), "Path"),
			Body: &ast.BlockStmt{List: clauses},
		},
		returnBool(false),
	})
}

func ssrDynamicDecl(routes []SSRRoute) *ast.FuncDecl {
	body := []ast.Stmt{}
	for _, route := range routes {
		if len(ssrRoutePatternParams(route.Route)) == 0 {
			continue
		}
		body = append(body, ssrDynamicIfStmt(route))
	}
	body = append(body, returnBool(false))
	return funcDecl("ssrDynamic", actionParams(), boolResults(), body)
}

func ssrDynamicIfStmt(route SSRRoute) ast.Stmt {
	names := []ast.Expr{id("params"), id("ok")}
	body := ssrRouteContextStmts(route, true)
	body = append(body, define([]ast.Expr{id("html")}, stringLit(route.HTML)))
	for _, replacement := range route.Replacements {
		body = append(body, assign([]ast.Expr{id("html")}, call(
			sel("strings", "ReplaceAll"),
			id("html"),
			stringLit(replacement.Placeholder),
			call(sel("gowdkhtml", "Escape"), &ast.IndexExpr{X: id("params"), Index: stringLit(replacement.Param)}),
		)))
	}
	body = append(body, ssrWriteHTMLStmts(id("html"))...)
	return &ast.IfStmt{
		Init: define(names, call(sel("gowdkroute", "Match"), stringLit(route.Route), selExpr(selExpr(id("request"), "URL"), "Path"))),
		Cond: id("ok"),
		Body: block(body...),
	}
}

func ssrRouteContextStmts(route SSRRoute, includeParams bool) []ast.Stmt {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("ctx")}, call(
			sel("gowdkruntime", "WithRoute"),
			call(selExpr(id("request"), "Context")),
			&ast.CompositeLit{
				Type: sel("gowdkruntime", "RouteMetadata"),
				Elts: []ast.Expr{
					keyValue("Kind", stringLit("ssr")),
					keyValue("PageID", stringLit(route.PageID)),
					keyValue("Method", stringLit("GET")),
					keyValue("Path", stringLit(route.Route)),
				},
			},
		)),
	}
	if includeParams {
		stmts = append(stmts, assign([]ast.Expr{id("ctx")}, call(sel("gowdkruntime", "WithParams"), id("ctx"), id("params"))))
	}
	stmts = append(stmts, assign([]ast.Expr{id("request")}, call(selExpr(id("request"), "WithContext"), id("ctx"))))
	return stmts
}

func ssrWriteHTMLStmts(html ast.Expr) []ast.Stmt {
	return []ast.Stmt{
		assign([]ast.Expr{id("_")}, call(sel("gowdkresponse", "WriteNoStoreHTML"), id("response"), id("request"), html)),
		returnBool(true),
	}
}

func ssrUsesDynamicRoutes(routes []SSRRoute) bool {
	for _, route := range routes {
		if len(ssrRoutePatternParams(route.Route)) > 0 {
			return true
		}
	}
	return false
}

func ssrUsesReplacements(routes []SSRRoute) bool {
	for _, route := range routes {
		if len(route.Replacements) > 0 {
			return true
		}
	}
	return false
}
