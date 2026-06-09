package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk"
)

func envRuntimeValidationRequired(config gowdk.EnvConfig) bool {
	for _, variable := range config.Vars {
		if variable.Required && variable.Default == "" {
			return true
		}
	}
	for _, secret := range config.Secrets {
		if secret.Required {
			return true
		}
	}
	return false
}

func validateEnvContractDecl(config gowdk.EnvConfig) []ast.Decl {
	if !envRuntimeValidationRequired(config) {
		return nil
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("missing")}, &ast.CompositeLit{Type: &ast.ArrayType{Elt: id("string")}}),
	}
	for _, variable := range config.Vars {
		if !variable.Required || variable.Default != "" {
			continue
		}
		stmts = append(stmts, appendMissingEnvStmt(variable.Name))
	}
	for _, secret := range config.Secrets {
		if !secret.Required {
			continue
		}
		stmts = append(stmts, appendMissingEnvStmt(secret.Name))
	}
	stmts = append(stmts,
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: call(id("len"), id("missing")), Op: token.GTR, Y: intLit(0)},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{
				call(sel("errors", "New"), call(sel("strings", "Join"), id("missing"), stringLit("\n"))),
			}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("nil")}},
	)
	return []ast.Decl{funcDecl("validateEnvContract", nil, []*ast.Field{{Type: id("error")}}, stmts)}
}

func appendMissingEnvStmt(name string) ast.Stmt {
	return &ast.IfStmt{
		Init: define([]ast.Expr{id("value")}, call(sel("os", "Getenv"), stringLit(name))),
		Cond: &ast.BinaryExpr{
			X:  call(sel("strings", "TrimSpace"), id("value")),
			Op: token.EQL,
			Y:  stringLit(""),
		},
		Body: block(assign([]ast.Expr{id("missing")}, call(id("append"), id("missing"), stringLit(name+" is required but is not set")))),
	}
}

func validateEnvContractStmt(config gowdk.EnvConfig) []ast.Stmt {
	if !envRuntimeValidationRequired(config) {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Init: define([]ast.Expr{id("err")}, call(id("validateEnvContract"))),
		Cond: notNil("err"),
		Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
	}}
}
