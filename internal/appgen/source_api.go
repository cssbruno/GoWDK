package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func apiHandlerSource(apis []APIEndpoint) (source string, err error) {
	defer recoverGeneratedIdentifierError(&err)

	adapter := backendAdapterIR(Options{APIs: apis})
	decls := []ast.Decl{apiFuncDecl(adapter.APIs, false, false)}
	decls = append(decls, apiDecoderDecls(adapter.APIs)...)
	return printActionDecls(decls)
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
	stmts = append(stmts, apiInputDecodeStmts(api)...)
	stmts = append(stmts, boundAPIResultStmts(api)...)
	stmts = append(stmts, returnBool(true))
	return stmts
}

func apiBodyLimitStmt() ast.Stmt {
	return assign([]ast.Expr{selExpr(id("request"), "Body")}, call(sel("http", "MaxBytesReader"), id("response"), selExpr(id("request"), "Body"), id("maxAPIBodyBytes")))
}

func apiInputDecodeStmts(api BackendAPIAdapter) []ast.Stmt {
	switch api.Binding.Signature {
	case source.BackendSignatureAPIInput, source.BackendSignatureAPIInputPtr:
		return []ast.Stmt{
			define([]ast.Expr{id("input"), id("err")}, call(sel(apiDecoderName(api)), id("request"))),
			ifErrReturnInvalidJSONForm(),
		}
	default:
		return nil
	}
}

func boundAPIResultStmts(api BackendAPIAdapter) []ast.Stmt {
	args := []ast.Expr{id("ctx")}
	switch api.Binding.Signature {
	case source.BackendSignatureAPI:
		args = append(args, id("request"))
	case source.BackendSignatureAPIInput:
		args = append(args, id("input"))
	case source.BackendSignatureAPIInputPtr:
		args = append(args, &ast.UnaryExpr{Op: token.AND, X: id("input")})
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("result"), id("err")}, call(sel(api.BackendAlias, api.Binding.FunctionName), args...)),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				apiHandlerErrorStmt(api),
				returnBool(true),
			),
		},
	}
	if api.Binding.Signature == source.BackendSignatureAPI {
		return append(stmts, writeNoStoreHTTPStmt(id("result")))
	}
	return append(stmts,
		define([]ast.Expr{id("status")}, call(sel("gowdkapi", "ResultStatus"), id("result"), sel("http", "StatusOK"))),
		define([]ast.Expr{id("httpResult"), id("err")}, call(sel("gowdkresponse", "JSONValue"), id("status"), id("result"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
		writeNoStoreHTTPStmt(id("httpResult")),
	)
}

func apiHandlerErrorStmt(api BackendAPIAdapter) ast.Stmt {
	if api.Binding.Signature == source.BackendSignatureAPI {
		return writeNoStoreHandlerErrorExprStmt(id("err"), sel("http", "StatusInternalServerError"))
	}
	return writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError"))
}

func apiDecoderDecls(apis []BackendAPIAdapter) []ast.Decl {
	decls := make([]ast.Decl, 0, len(apis))
	for _, api := range apis {
		if !apiUsesTypedInput(api) {
			continue
		}
		if apiUsesQueryInput(api) {
			decls = append(decls, queryAPIDecoderDecl(api))
			continue
		}
		decls = append(decls, jsonAPIDecoderDecl(api))
	}
	return decls
}

func queryAPIDecoderDecl(api BackendAPIAdapter) *ast.FuncDecl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: sel(api.BackendAlias, api.Binding.InputType)}),
		define([]ast.Expr{id("values")}, call(sel("gowdkform", "FromURLValues"), call(selExpr(selExpr(id("request"), "URL"), "Query")))),
	}
	for index, field := range api.Binding.InputFields {
		stmts = append(stmts, boundActionFieldDecodeStmts(index, field)...)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("input"), id("nil")}})
	return funcDecl(apiDecoderName(api), []*ast.Field{
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}, []*ast.Field{{Type: sel(api.BackendAlias, api.Binding.InputType)}, {Type: id("error")}}, stmts)
}

func jsonAPIDecoderDecl(api BackendAPIAdapter) *ast.FuncDecl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: sel(api.BackendAlias, api.Binding.InputType)}),
		define([]ast.Expr{id("decoder"), id("err")}, call(sel("gowdkapi", "NewJSONFieldDecoder"), id("request"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("err")}}),
		},
		&ast.ForStmt{
			Cond: call(selExpr(id("decoder"), "More")),
			Body: block(
				define([]ast.Expr{id("field"), id("err")}, call(selExpr(id("decoder"), "Field"))),
				&ast.IfStmt{
					Cond: notNil("err"),
					Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("err")}}),
				},
				apiJSONFieldSwitch(api),
			),
		},
		&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(selExpr(id("decoder"), "Finish"))),
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("err")}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("nil")}},
	}
	return funcDecl(apiDecoderName(api), []*ast.Field{
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}, []*ast.Field{{Type: sel(api.BackendAlias, api.Binding.InputType)}, {Type: id("error")}}, stmts)
}

func apiJSONFieldSwitch(api BackendAPIAdapter) ast.Stmt {
	clauses := make([]ast.Stmt, 0, len(api.Binding.InputFields)+1)
	for index, field := range api.Binding.InputFields {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{stringLit(field.FormName)},
			Body: apiJSONFieldDecodeStmts(index, field),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{id("input"), call(selExpr(id("decoder"), "UnknownField"), id("field"))}},
	}})
	return &ast.SwitchStmt{Tag: id("field"), Body: &ast.BlockStmt{List: clauses}}
}

func apiJSONFieldDecodeStmts(index int, field source.BackendInputField) []ast.Stmt {
	fieldType := source.MustBackendInputFieldType(field.Type)
	value := id(fmtFieldValueName(index))
	var decode ast.Expr
	var assignment ast.Expr = value
	switch fieldType.Kind {
	case source.BackendInputFieldKindString:
		decode = call(selExpr(id("decoder"), "String"), stringLit(field.FormName))
	case source.BackendInputFieldKindBool:
		decode = call(selExpr(id("decoder"), "Bool"), stringLit(field.FormName))
	case source.BackendInputFieldKindSignedInt:
		decode = call(selExpr(id("decoder"), "Int"), stringLit(field.FormName), intLit(fieldType.BitSize))
		assignment = convertIfNeeded(field.Type, value)
	case source.BackendInputFieldKindUnsignedInt:
		decode = call(selExpr(id("decoder"), "Uint"), stringLit(field.FormName), intLit(fieldType.BitSize))
		assignment = convertIfNeeded(field.Type, value)
	case source.BackendInputFieldKindStringSlice:
		decode = call(selExpr(id("decoder"), "Strings"), stringLit(field.FormName))
	default:
		panic("unsupported typed API input field type " + field.Type)
	}
	return []ast.Stmt{
		define([]ast.Expr{value, id("err")}, decode),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("err")}}),
		},
		assign([]ast.Expr{selExpr(id("input"), field.FieldName)}, assignment),
	}
}

func apiDecoderName(api BackendAPIAdapter) string {
	return "decode" + source.ExportedIdentifier(api.PageID, "API") + source.ExportedIdentifier(api.APIName, "API") + "Input"
}

func apiUsesTypedInput(api BackendAPIAdapter) bool {
	return api.Binding.Status == source.BackendBindingBound &&
		(api.Binding.Signature == source.BackendSignatureAPIInput || api.Binding.Signature == source.BackendSignatureAPIInputPtr)
}

func apiUsesQueryInput(api BackendAPIAdapter) bool {
	return api.Method == "GET" || api.Method == "HEAD"
}

func apisUseTypedJSONInput(apis []BackendAPIAdapter) bool {
	for _, api := range apis {
		if apiUsesTypedInput(api) && !apiUsesQueryInput(api) {
			return true
		}
	}
	return false
}

func apisUseTypedResult(apis []BackendAPIAdapter) bool {
	for _, api := range apis {
		switch api.Binding.Signature {
		case source.BackendSignatureAPI0, source.BackendSignatureAPIInput, source.BackendSignatureAPIInputPtr:
			return true
		}
	}
	return false
}

func apisUseTypedQueryInput(apis []BackendAPIAdapter) bool {
	for _, api := range apis {
		if apiUsesTypedInput(api) && apiUsesQueryInput(api) {
			return true
		}
	}
	return false
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
