package appgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func contractHandlerDecls(exposures []BackendContractExposure, csrf bool, rateLimit bool) []ast.Decl {
	routable := routableContractExposures(exposures)
	decls := make([]ast.Decl, 0, len(routable))
	for _, exposure := range routable {
		if contractExposureExecutable(exposure) {
			decls = append(decls, executableContractHandlerDecl(exposure, csrf, rateLimit))
			continue
		}
		decls = append(decls, fallbackContractHandlerDecl(exposure))
	}
	return decls
}

func executableContractHandlerDecl(exposure BackendContractExposure, csrf bool, rateLimit bool) *ast.FuncDecl {
	return funcDecl(contractHandlerName(exposure), []*ast.Field{
		{Names: []*ast.Ident{id("contractRegistry")}, Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}},
	}, []*ast.Field{{Type: sel("gowdkruntime", "BackendHandler")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{&ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{List: actionParams()},
				Results: &ast.FieldList{List: boolResults()},
			},
			Body: block(executableContractHandlerStmts(exposure, csrf, rateLimit)...),
		}}},
	})
}

func executableContractHandlerStmts(exposure BackendContractExposure, csrf bool, rateLimit bool) []ast.Stmt {
	stmts := endpointContextStmts(
		string(exposure.Endpoint.Kind),
		exposure.Endpoint.PageID,
		exposure.Contract,
		exposure.Endpoint.Method,
		exposure.Endpoint.Path,
		"",
	)
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(exposure.Guards)...)
	stmts = append(stmts, contractInputStmts(exposure, csrf)...)
	if exposure.Endpoint.Kind == BackendEndpointCommand {
		stmts = append(stmts, executableCommandContractStmts(exposure)...)
	} else {
		stmts = append(stmts, executableQueryContractStmts(exposure)...)
	}
	stmts = append(stmts,
		define([]ast.Expr{id("httpResult"), id("err")}, call(sel("gowdkresponse", "JSONValue"), sel("http", "StatusOK"), id("result"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
		writeNoStoreHTTPStmt(id("httpResult")),
		returnBool(true),
	)
	return stmts
}

func executableCommandContractStmts(exposure BackendContractExposure) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id("result"), id("events"), id("err")}, call(&ast.IndexListExpr{
			X: sel("gowdkcontracts", "CaptureCommandEventsForRole"),
			Indices: []ast.Expr{
				sel(exposure.ImportAlias, exposure.Type),
				sel(exposure.ImportAlias, exposure.Result),
			},
		}, id("ctx"), id("contractRegistry"), sel("gowdkcontracts", "RoleWeb"), id("input"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
		&ast.IfStmt{
			Init: define([]ast.Expr{id("dispatchErr")}, call(sel("gowdkcontracts", "DispatchCommandEvents"), id("ctx"), call(id("currentContractEventSink")), id("contractRegistry"), sel("gowdkcontracts", "RoleWeb"), id("events"))),
			Cond: notNil("dispatchErr"),
			Body: block(
				writeNoStoreHandlerJSONErrorExprStmt(id("dispatchErr"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
	}
}

func executableQueryContractStmts(exposure BackendContractExposure) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id("result"), id("err")}, call(&ast.IndexListExpr{
			X: sel("gowdkcontracts", "ExecuteQueryForRole"),
			Indices: []ast.Expr{
				sel(exposure.ImportAlias, exposure.Type),
				sel(exposure.ImportAlias, exposure.Result),
			},
		}, id("ctx"), id("contractRegistry"), sel("gowdkcontracts", "RoleWeb"), id("input"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
	}
}

func contractInputStmts(exposure BackendContractExposure, csrf bool) []ast.Stmt {
	switch exposure.Endpoint.Kind {
	case BackendEndpointCommand:
		stmts := actionParseFormStmts(csrf)
		if len(exposure.InputFields) == 0 {
			stmts = append(stmts, define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: sel(exposure.ImportAlias, exposure.Type)}))
			return stmts
		}
		stmts = append(stmts, contractDecodeInputStmts(exposure, call(sel("gowdkform", "FromURLValues"), selExpr(id("request"), "PostForm")))...)
		return stmts
	case BackendEndpointQuery:
		if len(exposure.InputFields) == 0 {
			return []ast.Stmt{
				define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: sel(exposure.ImportAlias, exposure.Type)}),
			}
		}
		return contractDecodeInputStmts(exposure, call(sel("gowdkform", "FromURLValues"), call(selExpr(selExpr(id("request"), "URL"), "Query"))))
	default:
		return []ast.Stmt{
			define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: sel(exposure.ImportAlias, exposure.Type)}),
		}
	}
}

func contractDecodeInputStmts(exposure BackendContractExposure, values ast.Expr) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id("values")}, values),
		define([]ast.Expr{id("input"), id("err")}, call(sel(contractDecoderName(exposure)), id("values"))),
		ifErrReturnInvalidJSONForm(),
	}
}

func contractDecoderDecls(exposures []BackendContractExposure) []ast.Decl {
	seen := map[string]bool{}
	var decls []ast.Decl
	for _, exposure := range executableContractExposures(exposures) {
		if len(exposure.InputFields) == 0 {
			continue
		}
		name := contractDecoderName(exposure)
		if seen[name] {
			continue
		}
		seen[name] = true
		decls = append(decls, contractDecoderDecl(exposure))
	}
	return decls
}

func contractDecoderDecl(exposure BackendContractExposure) *ast.FuncDecl {
	inputType := sel(exposure.ImportAlias, exposure.Type)
	stmts := []ast.Stmt{
		define([]ast.Expr{id("decoded"), id("err")}, call(sel("gowdkform", "DecodeExpected"), id("values"), contractFormSchemaExpr(exposure.InputFields))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{
				&ast.CompositeLit{Type: inputType},
				id("err"),
			}}),
		},
		assign([]ast.Expr{id("values")}, id("decoded")),
		define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: inputType}),
	}
	for index, field := range exposure.InputFields {
		stmts = append(stmts, boundActionFieldDecodeStmts(index, field)...)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("input"), id("nil")}})
	return funcDecl(contractDecoderName(exposure), []*ast.Field{
		{Names: []*ast.Ident{id("values")}, Type: sel("gowdkform", "Values")},
	}, []*ast.Field{{Type: inputType}, {Type: id("error")}}, stmts)
}

func contractFormSchemaExpr(fields []source.BackendInputField) ast.Expr {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.FormName)
	}
	return formSchemaExpr(names)
}

func fallbackContractHandlerDecl(exposure BackendContractExposure) *ast.FuncDecl {
	return funcDecl(contractHandlerName(exposure), actionParams(), boolResults(), []ast.Stmt{
		writeNoStoreJSONErrorStmt(sel("http", "StatusNotImplemented"), contractFallbackMessage(exposure)),
		returnBool(true),
	})
}

func contractFallbackMessage(exposure BackendContractExposure) string {
	message := strings.TrimSpace(exposure.Message)
	if message != "" {
		return message
	}
	if exposure.Status == gwdkir.ContractBindingBound {
		return "GOWDK " + string(exposure.Endpoint.Kind) + " contract cannot be executed by the generated adapter"
	}
	return "GOWDK " + string(exposure.Endpoint.Kind) + " contract is not implemented"
}

func routableContractExposures(exposures []BackendContractExposure) []BackendContractExposure {
	out := make([]BackendContractExposure, 0, len(exposures))
	seen := map[string]bool{}
	for _, exposure := range exposures {
		if strings.TrimSpace(exposure.Endpoint.Method) == "" || strings.TrimSpace(exposure.Endpoint.Path) == "" {
			continue
		}
		key := string(exposure.Endpoint.Kind) + "\x00" + exposure.Endpoint.Method + "\x00" + exposure.Endpoint.Path + "\x00" + exposure.Contract + "\x00" + exposure.OwnerID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, exposure)
	}
	return out
}

func executableContractExposures(exposures []BackendContractExposure) []BackendContractExposure {
	routable := routableContractExposures(exposures)
	out := make([]BackendContractExposure, 0, len(routable))
	for _, exposure := range routable {
		if contractExposureExecutable(exposure) {
			out = append(out, exposure)
		}
	}
	return out
}

func executableCommandContractExposures(exposures []BackendContractExposure) []BackendContractExposure {
	executable := executableContractExposures(exposures)
	out := make([]BackendContractExposure, 0, len(executable))
	for _, exposure := range executable {
		if exposure.Endpoint.Kind == BackendEndpointCommand {
			out = append(out, exposure)
		}
	}
	return out
}

func contractExposureExecutable(exposure BackendContractExposure) bool {
	return exposure.Status == gwdkir.ContractBindingBound &&
		strings.TrimSpace(exposure.ImportAlias) != "" &&
		strings.TrimSpace(exposure.ImportPath) != "" &&
		strings.TrimSpace(exposure.Type) != "" &&
		strings.TrimSpace(exposure.Result) != "" &&
		strings.TrimSpace(exposure.Register) != ""
}

func contractExposuresUseForm(exposures []BackendContractExposure) bool {
	for _, exposure := range exposures {
		if len(exposure.InputFields) > 0 {
			return true
		}
	}
	return false
}

func contractExposuresParseForm(exposures []BackendContractExposure) bool {
	for _, exposure := range exposures {
		if exposure.Endpoint.Kind == BackendEndpointCommand {
			return true
		}
	}
	return false
}

func contractEventSinkDecls(exposures []BackendContractExposure) []ast.Decl {
	if len(executableCommandContractExposures(exposures)) == 0 {
		return nil
	}
	return []ast.Decl{
		contractEventSinkMutexVarDecl(),
		contractEventSinkVarDecl(),
		registerContractEventSinkDecl(),
		currentContractEventSinkDecl(),
	}
}

func contractRegistryDecls(exposures []BackendContractExposure) []ast.Decl {
	if len(executableContractExposures(exposures)) == 0 {
		return nil
	}
	return []ast.Decl{
		newContractRegistryDecl(exposures),
		runContractEventWorkerDecl(),
		runContractEventWorkerWithSeenStoreDecl(),
	}
}

func newContractRegistryDecl(exposures []BackendContractExposure) ast.Decl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("contractRegistry")}, call(sel("gowdkcontracts", "NewRegistry"))),
	}
	for _, registration := range contractRegisterCalls(exposures) {
		stmts = append(stmts, exprStmt(call(sel(registration.Alias, registration.Function), id("contractRegistry"))))
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("contractRegistry")}})
	return funcDecl("NewContractRegistry", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}},
	}, stmts)
}

func runContractEventWorkerDecl() ast.Decl {
	return funcDecl("RunContractEventWorker", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{
			call(sel("gowdkcontracts", "RunEventWorker"), id("ctx"), call(id("NewContractRegistry")), id("source")),
		}},
	})
}

func runContractEventWorkerWithSeenStoreDecl() ast.Decl {
	return funcDecl("RunContractEventWorkerWithSeenStore", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
		{Names: []*ast.Ident{id("seen")}, Type: sel("gowdkcontracts", "SeenStore")},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{
			call(sel("gowdkcontracts", "RunEventWorkerWithSeenStore"), id("ctx"), call(id("NewContractRegistry")), id("source"), id("seen")),
		}},
	})
}

func contractEventSinkMutexVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("contractEventSinkMu")},
		Type:  sel("sync", "RWMutex"),
	}}}
}

func contractEventSinkVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("contractEventSink")},
		Type:  sel("gowdkcontracts", "CommandEventSink"),
	}}}
}

func registerContractEventSinkDecl() ast.Decl {
	return funcDecl("RegisterContractEventSink", []*ast.Field{
		{Names: []*ast.Ident{id("sink")}, Type: sel("gowdkcontracts", "CommandEventSink")},
	}, nil, []ast.Stmt{
		exprStmt(call(selExpr(id("contractEventSinkMu"), "Lock"))),
		&ast.DeferStmt{Call: call(selExpr(id("contractEventSinkMu"), "Unlock"))},
		assign([]ast.Expr{id("contractEventSink")}, id("sink")),
	})
}

func currentContractEventSinkDecl() ast.Decl {
	return funcDecl("currentContractEventSink", nil, []*ast.Field{
		{Type: sel("gowdkcontracts", "CommandEventSink")},
	}, []ast.Stmt{
		exprStmt(call(selExpr(id("contractEventSinkMu"), "RLock"))),
		&ast.DeferStmt{Call: call(selExpr(id("contractEventSinkMu"), "RUnlock"))},
		&ast.ReturnStmt{Results: []ast.Expr{id("contractEventSink")}},
	})
}

type contractRegisterCall struct {
	Alias    string
	Function string
}

func contractRegisterCalls(exposures []BackendContractExposure) []contractRegisterCall {
	seen := map[string]bool{}
	var calls []contractRegisterCall
	for _, exposure := range executableContractExposures(exposures) {
		key := exposure.ImportAlias + "." + exposure.Register
		if seen[key] {
			continue
		}
		seen[key] = true
		calls = append(calls, contractRegisterCall{Alias: exposure.ImportAlias, Function: exposure.Register})
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Alias != calls[j].Alias {
			return calls[i].Alias < calls[j].Alias
		}
		return calls[i].Function < calls[j].Function
	})
	return calls
}

func backendContractImports(exposures []BackendContractExposure) map[string]string {
	imports := map[string]string{}
	for _, exposure := range executableContractExposures(exposures) {
		imports[exposure.ImportPath] = exposure.ImportAlias
	}
	return imports
}

func hasRoutableContractReferences(options Options) bool {
	if options.IR == nil {
		return false
	}
	return len(routableContractExposures(backendAdapterIR(options).ContractExposures)) > 0
}

func contractHandlerName(exposure BackendContractExposure) string {
	return string(exposure.Endpoint.Kind) +
		source.ExportedIdentifier(exposure.OwnerID, "Contract") +
		source.ExportedIdentifier(exposure.Type, "Contract") +
		source.ExportedIdentifier(exposure.Endpoint.Method, "Method") +
		source.ExportedIdentifier(exposure.Endpoint.Path, "Path")
}

func contractDecoderName(exposure BackendContractExposure) string {
	return "decodeContract" + source.ExportedIdentifier(exposure.ImportAlias, "Contract") + source.ExportedIdentifier(exposure.Type, "Contract") + "Input"
}
