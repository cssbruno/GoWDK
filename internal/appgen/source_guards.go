package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/runtime/auth"
)

func generatedUsesGuards(options Options) bool {
	if adapterUsesRuntimeGuards(backendAdapterIR(options)) {
		return true
	}
	for _, route := range options.SSR {
		if len(runtimeGuardNames(route.Guards)) > 0 {
			return true
		}
	}
	return false
}

func adapterUsesRuntimeGuards(adapter BackendAdapterIR) bool {
	return len(runtimeGuardNames(adapter.GuardNames())) > 0
}

func guardDecls(options Options) []ast.Decl {
	if !generatedUsesGuards(options) {
		return nil
	}
	decls := []ast.Decl{
		guardRegistryVarDecl(),
		authProviderVarDecl(),
		registerGuardsDecl(),
		registerAuthProviderDecl(),
		runGuardsDecl(),
	}
	if initDecl := requiredGuardBackingInitDecl(options); initDecl != nil {
		decls = append(decls, initDecl)
	}
	return decls
}

func guardRegistryVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("guardRegistry")},
		Type:  sel("gowdkguard", "Registry"),
	}}}
}

func authProviderVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("authProvider")},
		Type:  sel("gowdkauth", "Provider"),
	}}}
}

func registerGuardsDecl() ast.Decl {
	return funcDecl("RegisterGuards", []*ast.Field{
		{Names: []*ast.Ident{id("registry")}, Type: sel("gowdkguard", "Registry")},
	}, nil, []ast.Stmt{
		assign([]ast.Expr{id("guardRegistry")}, id("registry")),
	})
}

func registerAuthProviderDecl() ast.Decl {
	return funcDecl("RegisterAuthProvider", []*ast.Field{
		{Names: []*ast.Ident{id("provider")}, Type: sel("gowdkauth", "Provider")},
	}, nil, []ast.Stmt{
		assign([]ast.Expr{id("authProvider")}, id("provider")),
	})
}

func requiredGuardBackingInitDecl(options Options) ast.Decl {
	var stmts []ast.Stmt
	if generatedUsesCustomGuards(options) {
		stmts = append(stmts, exprStmt(call(sel("RegisterGuards"), call(id("GOWDKGuardRegistry")))))
	}
	if generatedUsesNativeRBACGuards(options) {
		stmts = append(stmts, exprStmt(call(sel("RegisterAuthProvider"), call(id("GOWDKAuthProvider")))))
	}
	if len(stmts) == 0 {
		return nil
	}
	return funcDecl("init", nil, nil, stmts)
}

func runGuardsDecl() ast.Decl {
	return funcDecl("runGuards", []*ast.Field{
		{Names: []*ast.Ident{id("response")}, Type: sel("http", "ResponseWriter")},
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
		{Names: []*ast.Ident{id("guards")}, Type: &ast.ArrayType{Elt: id("string")}},
	}, boolResults(), []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: call(id("len"), id("guards")), Op: token.EQL, Y: intLit(0)},
			Body: block(returnBool(true)),
		},
		define([]ast.Expr{id("guardContext")}, call(sel("gowdkguard", "NewContext"), id("request"), id("nil"))),
		&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(sel("gowdkguard", "RunGuardsWithAuth"), id("guardContext"), id("guards"), id("guardRegistry"), id("authProvider"))),
			Cond: notNil("err"),
			Body: block(
				exprStmt(call(sel("gowdkguard", "WriteNoStoreFailure"), id("response"), id("err"))),
				returnBool(false),
			),
		},
		returnBool(true),
	})
}

func guardStmts(guards []string) []ast.Stmt {
	guards = runtimeGuardNames(guards)
	if len(guards) == 0 {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: call(sel("runGuards"), id("response"), id("request"), stringSliceExpr(guards))},
		Body: block(returnBool(true)),
	}}
}

// endpointDeniedByOmission reports whether an endpoint that declares the given
// guards must be denied at request time because it declares no guard at all. An
// endpoint that declares `guard public` (or any runtime guard) is not denied by
// omission.
func endpointDeniedByOmission(guards []string) bool {
	return len(guards) == 0
}

// denyByOmissionStmts emits a fail-closed 403 for an endpoint that declares no
// guard. It returns before any context, body parsing, or handler statements,
// matching the SSR route lane (ssrRouteBodyStmts) and the DefaultDeny posture
// reported in gowdk-security.json.
func denyByOmissionStmts() []ast.Stmt {
	return []ast.Stmt{
		writeNoStoreErrorStmt(sel("http", "StatusForbidden"), "403 forbidden"),
		returnBool(true),
	}
}

func generatedUsesCustomGuards(options Options) bool {
	for _, name := range generatedGuardNames(options) {
		if auth.IsPublicGuard(name) {
			continue
		}
		if !auth.IsNativeGuard(name) {
			return true
		}
	}
	return false
}

func generatedUsesNativeRBACGuards(options Options) bool {
	for _, name := range generatedGuardNames(options) {
		if auth.IsPublicGuard(name) {
			continue
		}
		if auth.IsNativeGuard(name) {
			return true
		}
	}
	return false
}

func generatedGuardNames(options Options) []string {
	guards := backendAdapterIR(options).GuardNames()
	for _, route := range options.SSR {
		guards = append(guards, route.Guards...)
	}
	return guards
}

func runtimeGuardNames(guards []string) []string {
	var filtered []string
	for _, guard := range guards {
		if auth.IsPublicGuard(guard) {
			continue
		}
		filtered = append(filtered, guard)
	}
	return filtered
}
