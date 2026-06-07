package appgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func contractHandlerDecls(exposures []BackendContractExposure) []ast.Decl {
	routable := routableContractExposures(exposures)
	decls := make([]ast.Decl, 0, len(routable))
	for _, exposure := range routable {
		if contractExposureExecutable(exposure) {
			decls = append(decls, executableContractHandlerDecl(exposure))
			continue
		}
		decls = append(decls, fallbackContractHandlerDecl(exposure))
	}
	return decls
}

func executableContractHandlerDecl(exposure BackendContractExposure) *ast.FuncDecl {
	return funcDecl(contractHandlerName(exposure), []*ast.Field{
		{Names: []*ast.Ident{id("contractRegistry")}, Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}},
	}, []*ast.Field{{Type: sel("gowdkruntime", "BackendHandler")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{&ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{List: actionParams()},
				Results: &ast.FieldList{List: boolResults()},
			},
			Body: block(executableContractHandlerStmts(exposure)...),
		}}},
	})
}

func executableContractHandlerStmts(exposure BackendContractExposure) []ast.Stmt {
	stmts := endpointContextStmts(
		string(exposure.Endpoint.Kind),
		exposure.Endpoint.PageID,
		exposure.Contract,
		exposure.Endpoint.Method,
		exposure.Endpoint.Path,
		"",
	)
	stmts = append(stmts,
		&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("input")},
			Type:  sel(exposure.ImportAlias, exposure.Type),
		}}}},
	)
	execute := "ExecuteCommandForRole"
	if exposure.Endpoint.Kind == BackendEndpointQuery {
		execute = "ExecuteQueryForRole"
	}
	stmts = append(stmts,
		define([]ast.Expr{id("result"), id("err")}, call(&ast.IndexListExpr{
			X: sel("gowdkcontracts", execute),
			Indices: []ast.Expr{
				sel(exposure.ImportAlias, exposure.Type),
				sel(exposure.ImportAlias, exposure.Result),
			},
		}, id("ctx"), id("contractRegistry"), sel("gowdkcontracts", "RoleWeb"), id("input"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreErrorExprStmt(call(sel("gowdkresponse", "HandlerStatus"), id("err"), sel("http", "StatusInternalServerError")), call(selExpr(id("err"), "Error"))),
				returnBool(true),
			),
		},
		define([]ast.Expr{id("httpResult"), id("err")}, call(sel("gowdkresponse", "JSONValue"), sel("http", "StatusOK"), id("result"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreErrorExprStmt(sel("http", "StatusInternalServerError"), call(selExpr(id("err"), "Error"))),
				returnBool(true),
			),
		},
		writeNoStoreHTTPStmt(id("httpResult")),
		returnBool(true),
	)
	return stmts
}

func fallbackContractHandlerDecl(exposure BackendContractExposure) *ast.FuncDecl {
	return funcDecl(contractHandlerName(exposure), actionParams(), boolResults(), []ast.Stmt{
		writeNoStoreErrorStmt(sel("http", "StatusNotImplemented"), contractFallbackMessage(exposure)),
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

func contractExposureExecutable(exposure BackendContractExposure) bool {
	return exposure.Status == gwdkir.ContractBindingBound &&
		strings.TrimSpace(exposure.ImportAlias) != "" &&
		strings.TrimSpace(exposure.ImportPath) != "" &&
		strings.TrimSpace(exposure.Type) != "" &&
		strings.TrimSpace(exposure.Result) != "" &&
		strings.TrimSpace(exposure.Register) != ""
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
		exportedIdentifier(exposure.OwnerID) +
		exportedIdentifier(exposure.Type) +
		exportedIdentifier(exposure.Endpoint.Method) +
		exportedIdentifier(exposure.Endpoint.Path)
}
