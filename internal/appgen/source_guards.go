package appgen

import (
	"go/ast"
	"go/token"
)

func generatedUsesGuards(options Options) bool {
	if endpointsUseGuards(options.Actions, options.APIs) {
		return true
	}
	for _, route := range options.SSR {
		if len(route.Guards) > 0 {
			return true
		}
	}
	return false
}

func endpointsUseGuards(actions []ActionEndpoint, apis []APIEndpoint) bool {
	for _, action := range actions {
		if len(action.Guards) > 0 {
			return true
		}
	}
	for _, api := range apis {
		if len(api.Guards) > 0 {
			return true
		}
	}
	return false
}

func guardDecls(options Options) []ast.Decl {
	if !generatedUsesGuards(options) {
		return nil
	}
	return []ast.Decl{
		guardRegistryVarDecl(),
		registerGuardsDecl(),
		runGuardsDecl(),
	}
}

func guardRegistryVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("guardRegistry")},
		Type:  sel("gowdkssr", "GuardRegistry"),
	}}}
}

func registerGuardsDecl() ast.Decl {
	return funcDecl("RegisterGuards", []*ast.Field{
		{Names: []*ast.Ident{id("registry")}, Type: sel("gowdkssr", "GuardRegistry")},
	}, nil, []ast.Stmt{
		assign([]ast.Expr{id("guardRegistry")}, id("registry")),
	})
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
		define([]ast.Expr{id("loadContext")}, call(sel("gowdkssr", "NewLoadContext"), id("request"), id("nil"))),
		&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(sel("gowdkssr", "RunGuards"), id("loadContext"), id("guards"), id("guardRegistry"))),
			Cond: notNil("err"),
			Body: block(
				writeNoStoreErrorExprStmt(sel("http", "StatusForbidden"), call(selExpr(id("err"), "Error"))),
				returnBool(false),
			),
		},
		returnBool(true),
	})
}

func guardStmts(guards []string) []ast.Stmt {
	if len(guards) == 0 {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: call(sel("runGuards"), id("response"), id("request"), stringSliceExpr(guards))},
		Body: block(returnBool(true)),
	}}
}
