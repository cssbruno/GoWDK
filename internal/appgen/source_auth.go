package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk"
)

const authRequiredGuard = "auth.required"

func generatedUsesAuthAddon(options Options) bool {
	return options.Config.HasFeature(gowdk.FeatureAuth)
}

func authSetupDecls(options Options) []ast.Decl {
	if !generatedUsesAuthAddon(options) {
		return nil
	}
	return []ast.Decl{
		authSessionsVarDecl(),
		configureAuthDecl(options),
	}
}

func authSessionsVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("authSessions")},
		Type:  &ast.StarExpr{X: sel("gowdkauthaddon", "Sessions")},
	}}}
}

func configureAuthDecl(options Options) ast.Decl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("sessions"), id("err")}, call(sel("gowdkauthaddon", "Configure"), authOptionsExpr(authSessionOptions(options.Config)))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("err")}}),
		},
		assign([]ast.Expr{id("authSessions")}, id("sessions")),
	}
	if generatedUsesGuards(options) {
		stmts = append(stmts,
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: id("authProvider"), Op: token.EQL, Y: id("nil")},
				Body: block(exprStmt(call(sel("RegisterAuthProvider"), call(selExpr(id("sessions"), "Provider"))))),
			},
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: id("guardRegistry"), Op: token.EQL, Y: id("nil")},
				Body: block(assign([]ast.Expr{id("guardRegistry")}, &ast.CompositeLit{Type: sel("gowdkguard", "Registry")})),
			},
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{
					X:  &ast.IndexExpr{X: id("guardRegistry"), Index: stringLit(authRequiredGuard)},
					Op: token.EQL,
					Y:  id("nil"),
				},
				Body: block(assign([]ast.Expr{
					&ast.IndexExpr{X: id("guardRegistry"), Index: stringLit(authRequiredGuard)},
				}, call(sel("gowdkauthaddon", "RequireAuthenticated"), id("authProvider")))),
			},
		)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("nil")}})
	return funcDecl("configureAuth", nil, []*ast.Field{{Type: id("error")}}, stmts)
}

func authSetupStmts(options Options) []ast.Stmt {
	if !generatedUsesAuthAddon(options) {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Init: define([]ast.Expr{id("err")}, call(id("configureAuth"))),
		Cond: notNil("err"),
		Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
	}}
}

func authSessionOptions(config gowdk.Config) gowdk.AuthSessionOptions {
	for _, addon := range config.Addons {
		provider, ok := addon.(gowdk.AuthSessionProvider)
		if ok {
			return provider.AuthSessionOptions()
		}
	}
	return gowdk.AuthSessionOptions{}
}

func authOptionsExpr(options gowdk.AuthSessionOptions) ast.Expr {
	elts := []ast.Expr{}
	if options.SecretEnv == "" {
		elts = append(elts, keyValue("SecretEnv", sel("gowdkauthaddon", "DefaultSessionSecretEnv")))
	} else {
		elts = append(elts, keyValue("SecretEnv", stringLit(options.SecretEnv)))
	}
	if options.CookieName != "" {
		elts = append(elts, keyValue("CookieName", stringLit(options.CookieName)))
	}
	if options.TTL > 0 {
		elts = append(elts, keyValue("TTL", int64Lit(int64(options.TTL))))
	}
	if options.Insecure {
		elts = append(elts, keyValue("Insecure", id("true")))
	}
	return &ast.CompositeLit{
		Type: sel("gowdkauthaddon", "Options"),
		Elts: elts,
	}
}
