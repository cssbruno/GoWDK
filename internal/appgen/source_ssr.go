package appgen

import (
	"go/ast"
	"go/token"
	"sort"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func ssrHandlerSource(routes []SSRRoute) string {
	sorted := sortedSSRRoutes(routes)
	return printActionDecls([]ast.Decl{
		ssrExactDecl(sorted, false),
		ssrDynamicDecl(sorted, false),
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

func ssrExactDecl(routes []SSRRoute, rateLimit bool) *ast.FuncDecl {
	clauses := []ast.Stmt{}
	for _, route := range routes {
		if len(ssrRoutePatternParams(route.Route)) > 0 {
			continue
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{stringLit(route.Route)},
			Body: ssrRouteBodyStmts(route, false, rateLimit),
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

func ssrDynamicDecl(routes []SSRRoute, rateLimit bool) *ast.FuncDecl {
	body := []ast.Stmt{}
	for _, route := range routes {
		if len(ssrRoutePatternParams(route.Route)) == 0 {
			continue
		}
		body = append(body, ssrDynamicIfStmt(route, rateLimit))
	}
	body = append(body, returnBool(false))
	return funcDecl("ssrDynamic", actionParams(), boolResults(), body)
}

func ssrDynamicIfStmt(route SSRRoute, rateLimit bool) ast.Stmt {
	names := []ast.Expr{id("params"), id("ok")}
	body := ssrRouteBodyStmts(route, true, rateLimit)
	return &ast.IfStmt{
		Init: define(names, call(sel("gowdkroute", "Match"), stringLit(route.Route), selExpr(selExpr(id("request"), "URL"), "Path"))),
		Cond: id("ok"),
		Body: block(body...),
	}
}

func ssrRouteBodyStmts(route SSRRoute, includeParams bool, rateLimit bool) []ast.Stmt {
	body := ssrRouteContextStmts(route, includeParams)
	body = append(body, rateLimitStmts(rateLimit)...)
	body = append(body, guardStmts(route.Guards)...)
	body = append(body, define([]ast.Expr{id("html")}, stringLit(route.HTML)))
	for _, replacement := range route.Replacements {
		body = append(body, assign([]ast.Expr{id("html")}, call(
			sel("strings", "ReplaceAll"),
			id("html"),
			stringLit(replacement.Placeholder),
			call(sel("gowdkhtml", "Escape"), &ast.IndexExpr{X: id("params"), Index: stringLit(replacement.Param)}),
		)))
	}
	body = append(body, ssrLoadStmts(route)...)
	body = append(body, ssrWriteHTMLStmts(id("html"), route.Cache)...)
	return body
}

func ssrLoadStmts(route SSRRoute) []ast.Stmt {
	if !route.HasLoad {
		return nil
	}
	if route.LoadBinding.Status != manifest.BackendBindingBound {
		stmts := backendNotImplementedStmts(route.LoadBinding, "SSR load")
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("loadContext")}, call(sel("gowdkssr", "NewLoadContext"), id("request"), id("nil"))),
	}
	loadCall := call(sel(route.LoadBackendAlias, route.LoadBinding.FunctionName), id("loadContext"))
	switch route.LoadBinding.Signature {
	case manifest.BackendSignatureLoadError:
		stmts = append(stmts,
			define([]ast.Expr{id("loadData"), id("err")}, loadCall),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(ssrLoadErrorStmts()...),
			},
		)
	default:
		stmts = append(stmts, define([]ast.Expr{id("loadData")}, loadCall))
	}
	for index, replacement := range route.LoadReplacements {
		valueName := id("loadValue" + intIdentSuffix(index))
		okName := id("loadOK" + intIdentSuffix(index))
		stmts = append(stmts,
			define([]ast.Expr{valueName, okName}, call(sel("gowdkssr", "LoadPath"), id("loadData"), stringLit(replacement.Path))),
			&ast.IfStmt{
				Cond: &ast.UnaryExpr{Op: token.NOT, X: okName},
				Body: block(
					exprStmt(call(sel("gowdkruntime", "WriteErrorPage"), id("response"), id("request"), sel("http", "StatusInternalServerError"), stringLit("missing load field "+replacement.Path))),
					returnBool(true),
				),
			},
			assign([]ast.Expr{id("html")}, call(
				sel("strings", "ReplaceAll"),
				id("html"),
				stringLit(replacement.Placeholder),
				call(sel("gowdkhtml", "Escape"), call(sel("fmt", "Sprint"), valueName)),
			)),
		)
	}
	return stmts
}

func ssrLoadErrorStmts() []ast.Stmt {
	return []ast.Stmt{
		&ast.IfStmt{
			Init: define([]ast.Expr{id("redirectURL"), id("redirectStatus"), id("ok")}, call(sel("gowdkssr", "RedirectTarget"), id("err"))),
			Cond: id("ok"),
			Body: block(
				writeNoStoreHTTPStmt(&ast.CompositeLit{
					Type: sel("gowdkresponse", "Response"),
					Elts: []ast.Expr{
						keyValue("Kind", sel("gowdkresponse", "Redirect")),
						keyValue("Status", id("redirectStatus")),
						keyValue("URL", id("redirectURL")),
					},
				}),
				returnBool(true),
			),
		},
		exprStmt(call(sel("gowdkruntime", "WriteErrorPage"), id("response"), id("request"), sel("http", "StatusInternalServerError"), call(selExpr(id("err"), "Error")))),
		returnBool(true),
	}
}

func ssrRouteContextStmts(route SSRRoute, includeParams bool) []ast.Stmt {
	metadata := []ast.Expr{
		keyValue("Kind", stringLit("ssr")),
		keyValue("PageID", stringLit(route.PageID)),
		keyValue("Method", stringLit("GET")),
		keyValue("Path", stringLit(route.Route)),
		keyValue("Render", stringLit(ssrRouteRender(route))),
	}
	if route.Cache != "" {
		metadata = append(metadata, keyValue("Cache", stringLit(route.Cache)))
	}
	if route.ErrorPage != "" {
		metadata = append(metadata, keyValue("ErrorPage", stringLit(route.ErrorPage)))
	}
	dynamicParams := route.DynamicParams
	if len(dynamicParams) == 0 {
		dynamicParams = ssrRoutePatternParams(route.Route)
	}
	if len(dynamicParams) > 0 {
		metadata = append(metadata, keyValue("DynamicParams", stringSliceExpr(dynamicParams)))
	}
	if len(route.RouteParams) > 0 {
		metadata = append(metadata, keyValue("RouteParams", routeParamMetadataExpr(route.RouteParams)))
	}
	if len(route.Guards) > 0 {
		metadata = append(metadata, keyValue("Guards", stringSliceExpr(route.Guards)))
	}
	if route.HasLoad {
		metadata = append(metadata, keyValue("HasLoad", id("true")))
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("ctx")}, call(
			sel("gowdkruntime", "WithRoute"),
			call(selExpr(id("request"), "Context")),
			&ast.CompositeLit{
				Type: sel("gowdkruntime", "RouteMetadata"),
				Elts: metadata,
			},
		)),
	}
	if includeParams {
		stmts = append(stmts, assign([]ast.Expr{id("ctx")}, call(sel("gowdkruntime", "WithParams"), id("ctx"), id("params"))))
		stmts = append(stmts, typedRouteParamStmts(route.RouteParams)...)
	}
	stmts = append(stmts, assign([]ast.Expr{id("request")}, call(selExpr(id("request"), "WithContext"), id("ctx"))))
	return stmts
}

func typedRouteParamStmts(params []manifest.RouteParam) []ast.Stmt {
	if len(params) == 0 {
		return nil
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("typedParams")}, &ast.CompositeLit{Type: &ast.MapType{Key: id("string"), Value: id("any")}}),
	}
	for index, param := range params {
		paramType := param.Type
		if paramType == "" {
			paramType = "string"
		}
		stmts = append(stmts, typedRouteParamDecodeStmts(index, param.Name, paramType)...)
	}
	stmts = append(stmts, assign([]ast.Expr{id("ctx")}, call(sel("gowdkruntime", "WithTypedParams"), id("ctx"), id("typedParams"))))
	return stmts
}

func typedRouteParamDecodeStmts(index int, name, paramType string) []ast.Stmt {
	decode := sel("gowdkroute", routeParamDecodeFunc(paramType))
	valueName := id("paramValue" + intIdentSuffix(index))
	okName := id("paramOK" + intIdentSuffix(index))
	errName := id("paramErr" + intIdentSuffix(index))
	stmts := []ast.Stmt{
		define([]ast.Expr{valueName, okName, errName}, call(decode, id("params"), stringLit(name))),
	}
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.BinaryExpr{X: errName, Op: token.NEQ, Y: id("nil")},
		Body: block(
			writeNoStoreErrorStmt(sel("http", "StatusBadRequest"), "invalid route parameter "+name),
			returnBool(true),
		),
	})
	stmts = append(stmts, &ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: okName},
		Body: block(
			writeNoStoreErrorStmt(sel("http", "StatusNotFound"), "missing route parameter "+name),
			returnBool(true),
		),
	})
	stmts = append(stmts, assign(
		[]ast.Expr{&ast.IndexExpr{X: id("typedParams"), Index: stringLit(name)}},
		valueName,
	))
	return stmts
}

func intIdentSuffix(value int) string {
	if value == 0 {
		return "0"
	}
	var reversed []byte
	for value > 0 {
		reversed = append(reversed, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return string(reversed)
}

func routeParamDecodeFunc(paramType string) string {
	switch paramType {
	case "int":
		return "Int"
	case "int64":
		return "Int64"
	case "uint":
		return "Uint"
	case "uint64":
		return "Uint64"
	case "bool":
		return "Bool"
	case "float64":
		return "Float64"
	default:
		return "String"
	}
}

func routeParamMetadataExpr(params []manifest.RouteParam) ast.Expr {
	elts := make([]ast.Expr, 0, len(params))
	for _, param := range params {
		paramType := param.Type
		if paramType == "" {
			paramType = "string"
		}
		elts = append(elts, &ast.CompositeLit{
			Type: sel("gowdkruntime", "RouteParamMetadata"),
			Elts: []ast.Expr{
				keyValue("Name", stringLit(param.Name)),
				keyValue("Type", stringLit(paramType)),
			},
		})
	}
	return &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: sel("gowdkruntime", "RouteParamMetadata")},
		Elts: elts,
	}
}

func ssrRouteRender(route SSRRoute) string {
	if route.Render == "" {
		return "ssr"
	}
	return string(route.Render)
}

func ssrWriteHTMLStmts(html ast.Expr, cache string) []ast.Stmt {
	if cache != "" {
		return []ast.Stmt{
			assign([]ast.Expr{id("_")}, call(sel("gowdkresponse", "WriteHTML"), id("response"), id("request"), html, stringLit(cache))),
			returnBool(true),
		}
	}
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

func ssrUsesLoad(routes []SSRRoute) bool {
	for _, route := range routes {
		if route.HasLoad {
			return true
		}
	}
	return false
}
