package appgen

import (
	"go/ast"
	"go/token"
	"sort"
)

func newBackendRouterDecl(adapter BackendAdapterIR) *ast.FuncDecl {
	routes := []ast.Expr{}
	for _, registration := range adapter.Registrations {
		routes = append(routes, backendRouteExpr(backendRegistrationMethodExpr(registration), registration.Kind, registration.Path, id(registration.Handler)))
	}
	for _, exposure := range routableContractExposures(adapter.ContractExposures) {
		method := contractExposureMethodExpr(exposure)
		routes = append(routes, backendRouteExpr(method, exposure.Endpoint.Kind, exposure.Endpoint.Path, contractRouteHandlerExpr(exposure)))
	}
	stmts := []ast.Stmt{}
	if len(executableContractExposures(adapter.ContractExposures)) > 0 {
		stmts = append(stmts,
			define([]ast.Expr{id("contractRegistry")}, call(id("NewContractRegistry"))),
		)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkruntime", "NewBackendRouter"), routes...)}})
	return funcDecl("newBackendRouter", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("gowdkruntime", "BackendRouter")}},
		{Type: id("error")},
	}, stmts)
}

func backendRegistrationMethodExpr(registration BackendEndpointRegistration) ast.Expr {
	switch {
	case registration.Kind == BackendEndpointAction && registration.Method == "POST":
		return sel("http", "MethodPost")
	case registration.Kind == BackendEndpointFragment && registration.Method == "GET":
		return sel("http", "MethodGet")
	default:
		return stringLit(registration.Method)
	}
}

func contractRouteHandlerExpr(exposure BackendContractExposure) ast.Expr {
	handler := sel(contractHandlerName(exposure))
	if contractExposureExecutable(exposure) {
		return call(handler, id("contractRegistry"))
	}
	return handler
}

func contractExposureMethodExpr(exposure BackendContractExposure) ast.Expr {
	switch {
	case exposure.Endpoint.Kind == BackendEndpointCommand && exposure.Endpoint.Method == "POST":
		return sel("http", "MethodPost")
	case exposure.Endpoint.Kind == BackendEndpointQuery && exposure.Endpoint.Method == "GET":
		return sel("http", "MethodGet")
	default:
		return stringLit(exposure.Endpoint.Method)
	}
}

func backendRouteExpr(method ast.Expr, kind BackendEndpointKind, route string, handler ast.Expr) ast.Expr {
	return &ast.CompositeLit{
		Type: sel("gowdkruntime", "BackendRoute"),
		Elts: []ast.Expr{
			keyValue("Method", method),
			keyValue("Path", stringLit(route)),
			keyValue("Kind", stringLit(string(kind))),
			keyValue("Handler", handler),
		},
	}
}

func emptyBackendHandlerDecl() *ast.FuncDecl {
	return funcDecl("backend", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
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

func backendProxySource(options Options) (string, error) {
	if !options.ProxyBackend || !hasBackendRoutes(options) {
		return "", nil
	}
	return printActionDecls([]ast.Decl{
		backendProxyDecl(false),
		isBackendRouteDecl(backendAdapterIR(options)),
	})
}

func backendProxyDecl(rateLimit bool) *ast.FuncDecl {
	stmts := []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: call(id("isBackendRoute"), selExpr(id("request"), "Method"), selExpr(selExpr(id("request"), "URL"), "Path"))},
			Body: block(returnBool(false)),
		},
	}
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts,
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
	)
	return funcDecl("backendProxy", actionParams(), boolResults(), stmts)
}

func isBackendRouteDecl(adapter BackendAdapterIR) *ast.FuncDecl {
	clauses := []ast.Stmt{}
	for _, registration := range adapter.Registrations {
		if registration.Dynamic {
			continue
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{backendRouteCond(backendRegistrationMethodExpr(registration), registration.Path)},
			Body: []ast.Stmt{returnBool(true)},
		})
	}
	for _, exposure := range routableContractExposures(adapter.ContractExposures) {
		if exposure.Endpoint.Dynamic {
			continue
		}
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{backendRouteCond(contractExposureMethodExpr(exposure), exposure.Endpoint.Path)},
			Body: []ast.Stmt{returnBool(true)},
		})
	}
	body := []ast.Stmt{}
	if adapter.HasDynamicRoutes() {
		body = append(body, define([]ast.Expr{id("rawRequestPath")}, id("requestPath")))
	}
	body = append(body,
		assign([]ast.Expr{id("requestPath")}, call(sel("path", "Clean"), &ast.BinaryExpr{X: stringLit("/"), Op: token.ADD, Y: id("requestPath")})),
		&ast.SwitchStmt{Body: &ast.BlockStmt{List: clauses}},
	)
	for _, registration := range adapter.Registrations {
		if !registration.Dynamic {
			continue
		}
		body = append(body, backendDynamicRouteIfStmt(backendRegistrationMethodExpr(registration), registration.Path))
	}
	for _, exposure := range routableContractExposures(adapter.ContractExposures) {
		if !exposure.Endpoint.Dynamic {
			continue
		}
		body = append(body, backendDynamicRouteIfStmt(contractExposureMethodExpr(exposure), exposure.Endpoint.Path))
	}
	body = append(body, returnBool(false))
	return funcDecl("isBackendRoute", []*ast.Field{
		{Names: []*ast.Ident{id("method")}, Type: id("string")},
		{Names: []*ast.Ident{id("requestPath")}, Type: id("string")},
	}, boolResults(), body)
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

func backendDynamicRouteIfStmt(method ast.Expr, route string) ast.Stmt {
	return &ast.IfStmt{
		Init: define([]ast.Expr{id("_"), id("ok")}, call(sel("gowdkroute", "Match"), stringLit(route), id("rawRequestPath"))),
		Cond: &ast.BinaryExpr{
			X: &ast.BinaryExpr{
				X:  id("method"),
				Op: token.EQL,
				Y:  method,
			},
			Op: token.LAND,
			Y:  id("ok"),
		},
		Body: block(returnBool(true)),
	}
}
