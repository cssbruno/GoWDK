package appgen

import (
	"go/ast"
	"go/token"
	"sort"
)

func backendHandlerSource(actions []ActionEndpoint, apis []APIEndpoint) string {
	if len(actions) == 0 && len(apis) == 0 {
		return printActionDecls([]ast.Decl{emptyBackendHandlerDecl()})
	}
	return printActionDecls([]ast.Decl{backendHandlerDecl()})
}

func emptyBackendHandlerDecl() *ast.FuncDecl {
	return funcDecl("backend", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
}

func backendHandlerDecl() *ast.FuncDecl {
	return funcDecl("backend", actionParams(), boolResults(), []ast.Stmt{
		&ast.IfStmt{
			Cond: call(id("api"), id("response"), id("request")),
			Body: block(returnBool(true)),
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.BinaryExpr{
					X:  selExpr(id("request"), "Method"),
					Op: token.EQL,
					Y:  sel("http", "MethodPost"),
				},
				Op: token.LAND,
				Y:  call(id("action"), id("response"), id("request")),
			},
			Body: block(returnBool(true)),
		},
		returnBool(false),
	})
}

func sortedAPIEndpoints(apis []APIEndpoint) []APIEndpoint {
	sorted := append([]APIEndpoint(nil), apis...)
	sortAPIEndpoints(sorted)
	return sorted
}

func sortAPIEndpoints(apis []APIEndpoint) {
	sort.Slice(apis, func(i, j int) bool {
		if apis[i].Route == apis[j].Route {
			if apis[i].Method == apis[j].Method {
				return apis[i].APIName < apis[j].APIName
			}
			return apis[i].Method < apis[j].Method
		}
		return apis[i].Route < apis[j].Route
	})
}

func backendProxySource(options Options) string {
	if !options.ProxyBackend || !hasBackendRoutes(options) {
		return ""
	}
	return printActionDecls([]ast.Decl{
		backendProxyDecl(),
		isBackendRouteDecl(options),
	})
}

func backendProxyDecl() *ast.FuncDecl {
	return funcDecl("backendProxy", actionParams(), boolResults(), []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: call(id("isBackendRoute"), selExpr(id("request"), "Method"), selExpr(selExpr(id("request"), "URL"), "Path"))},
			Body: block(returnBool(false)),
		},
		define([]ast.Expr{id("origin")}, call(sel("strings", "TrimSpace"), call(sel("os", "Getenv"), stringLit("GOWDK_BACKEND_ORIGIN")))),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("origin"), Op: token.EQL, Y: stringLit("")},
			Body: block(
				writeNoStoreErrorStmt(sel("http", "StatusBadGateway"), "GOWDK_BACKEND_ORIGIN is required for split backend routes"),
				returnBool(true),
			),
		},
		define([]ast.Expr{id("target"), id("err")}, call(sel("neturl", "Parse"), id("origin"))),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  notNil("err"),
				Op: token.LOR,
				Y: &ast.BinaryExpr{
					X:  &ast.BinaryExpr{X: selExpr(id("target"), "Scheme"), Op: token.EQL, Y: stringLit("")},
					Op: token.LOR,
					Y:  &ast.BinaryExpr{X: selExpr(id("target"), "Host"), Op: token.EQL, Y: stringLit("")},
				},
			},
			Body: block(
				writeNoStoreErrorStmt(sel("http", "StatusBadGateway"), "GOWDK_BACKEND_ORIGIN is invalid"),
				returnBool(true),
			),
		},
		define([]ast.Expr{id("proxy")}, call(sel("httputil", "NewSingleHostReverseProxy"), id("target"))),
		exprStmt(call(selExpr(id("proxy"), "ServeHTTP"), id("response"), id("request"))),
		returnBool(true),
	})
}

func isBackendRouteDecl(options Options) *ast.FuncDecl {
	clauses := []ast.Stmt{}
	for _, action := range sortedActionEndpoints(options.Actions) {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{backendRouteCond(sel("http", "MethodPost"), action.Route)},
			Body: []ast.Stmt{returnBool(true)},
		})
	}
	for _, api := range sortedAPIEndpoints(options.APIs) {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{backendRouteCond(stringLit(api.Method), api.Route)},
			Body: []ast.Stmt{returnBool(true)},
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{returnBool(false)}})
	return funcDecl("isBackendRoute", []*ast.Field{
		{Names: []*ast.Ident{id("method")}, Type: id("string")},
		{Names: []*ast.Ident{id("requestPath")}, Type: id("string")},
	}, boolResults(), []ast.Stmt{
		assign([]ast.Expr{id("requestPath")}, call(sel("path", "Clean"), &ast.BinaryExpr{X: stringLit("/"), Op: token.ADD, Y: id("requestPath")})),
		&ast.SwitchStmt{Body: &ast.BlockStmt{List: clauses}},
	})
}

func backendRouteCond(method ast.Expr, route string) ast.Expr {
	return &ast.BinaryExpr{
		X: &ast.BinaryExpr{
			X:  id("method"),
			Op: token.EQL,
			Y:  method,
		},
		Op: token.LAND,
		Y: &ast.BinaryExpr{
			X:  id("requestPath"),
			Op: token.EQL,
			Y:  stringLit(cleanRoutePath(route)),
		},
	}
}
