package appgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/source"
)

func newBackendRouterDecl(adapter BackendAdapterIR, trace bool, cors ast.Expr) *ast.FuncDecl {
	routes := []ast.Expr{}
	for _, registration := range adapter.Registrations {
		routes = append(routes, backendRouteExpr(registration, backendRegistrationMethodExpr(registration), id(registration.Handler), trace))
	}
	for _, exposure := range routableContractExposures(adapter.ContractExposures) {
		method := contractExposureMethodExpr(exposure)
		routes = append(routes, backendRouteExpr(exposure.Endpoint, method, contractRouteHandlerExpr(exposure), trace))
	}
	stmts := []ast.Stmt{}
	if len(executableContractExposures(adapter.ContractExposures)) > 0 {
		stmts = append(stmts,
			define([]ast.Expr{id("contractRegistry")}, call(id("ContractRegistry"))),
		)
	}
	if cors == nil {
		stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkruntime", "NewBackendRouter"), routes...)}})
	} else {
		stmts = append(stmts,
			define([]ast.Expr{id("backendRouter"), id("err")}, call(sel("gowdkruntime", "NewBackendRouter"), routes...)),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
			&ast.IfStmt{
				Init: define([]ast.Expr{id("err")}, call(selExpr(id("backendRouter"), "SetCORSPolicy"), cors)),
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
			&ast.ReturnStmt{Results: []ast.Expr{id("backendRouter"), id("nil")}},
		)
	}
	return funcDecl("newBackendRouter", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("gowdkruntime", "BackendRouter")}},
		{Type: id("error")},
	}, stmts)
}

func validateCORSConfig(options Options) error {
	if err := options.Config.Build.CORS.Validate(); err != nil {
		return err
	}
	for _, registration := range backendAdapterIR(options).Registrations {
		if registration.CORS == nil {
			continue
		}
		if err := registration.CORS.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func corsPolicyExpr(options Options) ast.Expr {
	if !options.Config.Build.CORS.EnabledForGeneratedAPIs() {
		return nil
	}
	if !backendAdapterIR(options).HasCORSRoutes() {
		return nil
	}
	return corsPolicyLiteral(options.Config.Build.CORS)
}

func corsPolicyLiteral(config gowdk.CORSConfig) ast.Expr {
	elts := []ast.Expr{
		keyValue("AllowedOrigins", stringSliceExpr(config.AllowedOrigins)),
	}
	if len(config.AllowedMethods) > 0 {
		elts = append(elts, keyValue("AllowedMethods", stringSliceExpr(config.AllowedMethods)))
	}
	if len(config.AllowedHeaders) > 0 {
		elts = append(elts, keyValue("AllowedHeaders", stringSliceExpr(config.AllowedHeaders)))
	}
	if len(config.ExposedHeaders) > 0 {
		elts = append(elts, keyValue("ExposedHeaders", stringSliceExpr(config.ExposedHeaders)))
	}
	if config.AllowCredentials {
		elts = append(elts, keyValue("AllowCredentials", id("true")))
	}
	if config.MaxAgeSeconds > 0 {
		elts = append(elts, keyValue("MaxAgeSeconds", intLit(config.MaxAgeSeconds)))
	}
	return &ast.CompositeLit{
		Type: sel("gowdkruntime", "CORSPolicy"),
		Elts: elts,
	}
}

func corsPolicyPtrExpr(config gowdk.CORSConfig) ast.Expr {
	return &ast.UnaryExpr{Op: token.AND, X: corsPolicyLiteral(config)}
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

func backendRouteExpr(registration BackendEndpointRegistration, method ast.Expr, handler ast.Expr, trace bool) ast.Expr {
	elts := []ast.Expr{
		keyValue("Method", method),
		keyValue("Path", stringLit(registration.Path)),
		keyValue("Kind", stringLit(string(registration.Kind))),
		keyValue("EndpointID", stringLit(backendEndpointID(registration))),
		keyValue("Handler", handler),
	}
	if trace {
		if source := traceSourceRefExpr(registration.Source, string(registration.Kind), registration.PageID, registration.Span); source != nil {
			elts = append(elts, keyValue("Source", source))
		}
	}
	if registration.CORS != nil {
		elts = append(elts, keyValue("CORS", corsPolicyPtrExpr(*registration.CORS)))
	}
	return &ast.CompositeLit{
		Type: sel("gowdkruntime", "BackendRoute"),
		Elts: elts,
	}
}

func backendEndpointID(registration BackendEndpointRegistration) string {
	switch {
	case strings.Contains(registration.Name, "."):
		return registration.Name
	case registration.PageID != "" && registration.Name != "":
		return registration.PageID + "." + registration.Name
	case registration.Name != "":
		return registration.Name
	case registration.PageID != "":
		return registration.PageID
	default:
		return string(registration.Kind) + " " + registration.Path
	}
}

func traceSourceRefExpr(sourcePath, ownerKind, ownerID string, span source.SourceSpan) ast.Expr {
	if sourcePath == "" && span.Start.Line <= 0 && ownerKind == "" && ownerID == "" {
		return nil
	}
	return &ast.CompositeLit{
		Type: sel("gowdktrace", "SourceRef"),
		Elts: []ast.Expr{
			keyValue("File", stringLit(sourcePath)),
			keyValue("Line", intLit(span.Start.Line)),
			keyValue("Column", intLit(span.Start.Column)),
			keyValue("OwnerKind", stringLit(ownerKind)),
			keyValue("OwnerID", stringLit(ownerID)),
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

func backendProxySource(options Options) (source string, err error) {
	defer recoverGeneratedIdentifierError(&err)

	if !options.ProxyBackend || !hasBackendRoutes(options) {
		return "", nil
	}
	return printActionDecls([]ast.Decl{
		backendProxyDecl(false, generatedObservabilityEnabled(options)),
		isBackendRouteDecl(backendAdapterIR(options)),
	})
}

func backendProxyDecl(rateLimit bool, trace bool) *ast.FuncDecl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("routeMethod")}, selExpr(id("request"), "Method")),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: selExpr(id("request"), "Method"), Op: token.EQL, Y: sel("http", "MethodOptions")},
			Body: block(&ast.IfStmt{
				Init: define([]ast.Expr{id("requestedMethod")}, call(sel("strings", "ToUpper"), trimHeaderCall("Access-Control-Request-Method"))),
				Cond: &ast.BinaryExpr{X: id("requestedMethod"), Op: token.NEQ, Y: stringLit("")},
				Body: block(assign([]ast.Expr{id("routeMethod")}, id("requestedMethod"))),
			}),
		},
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: call(id("isBackendRoute"), id("routeMethod"), selExpr(selExpr(id("request"), "URL"), "Path"))},
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
	)
	if trace {
		stmts = append(stmts,
			define([]ast.Expr{id("proxyDirector")}, selExpr(id("proxy"), "Director")),
			assign([]ast.Expr{selExpr(id("proxy"), "Director")}, &ast.FuncLit{
				Type: &ast.FuncType{Params: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{id("outbound")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
				}}},
				Body: block(
					exprStmt(call(id("proxyDirector"), id("outbound"))),
					exprStmt(call(sel("gowdktrace", "Inject"), selExpr(id("request"), "Context"), selExpr(id("outbound"), "Header"))),
				),
			}),
		)
	}
	stmts = append(stmts,
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
