package appgen

import (
	"fmt"
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
		if secret.Required || secret.MinBytes > 0 {
			return true
		}
	}
	return false
}

func generatedEnvFileLoadRequired(options Options) bool {
	return envConfigDeclared(options.Config.Env) || csrfEnabled(options) || generatedUsesAuthAddon(options)
}

func envConfigDeclared(config gowdk.EnvConfig) bool {
	return len(config.Vars) > 0 || len(config.Secrets) > 0
}

func envDefaultsRequired(config gowdk.EnvConfig) bool {
	for _, variable := range config.Vars {
		if variable.Default != "" {
			return true
		}
	}
	return false
}

func loadEnvFileDecl(options Options) []ast.Decl {
	if !generatedEnvFileLoadRequired(options) {
		return nil
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("explicit")}, call(sel("strings", "TrimSpace"), call(sel("os", "Getenv"), stringLit("GOWDK_ENV_FILE")))),
		define([]ast.Expr{id("path"), id("_"), id("err")}, call(sel("gowdkenvfile", "LookupPath"), stringLit(""), id("explicit"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("err")}}),
		},
		assign([]ast.Expr{id("_"), id("err")}, call(sel("gowdkenvfile", "LoadIntoEnv"), id("path"), &ast.BinaryExpr{X: id("explicit"), Op: token.NEQ, Y: stringLit("")})),
		&ast.ReturnStmt{Results: []ast.Expr{id("err")}},
	}
	return []ast.Decl{funcDecl("loadEnvFile", nil, []*ast.Field{{Type: id("error")}}, stmts)}
}

func loadEnvFileStmt(options Options) []ast.Stmt {
	if !generatedEnvFileLoadRequired(options) {
		return nil
	}
	return []ast.Stmt{&ast.IfStmt{
		Init: define([]ast.Expr{id("err")}, call(id("loadEnvFile"))),
		Cond: notNil("err"),
		Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
	}}
}

func applyEnvDefaultsDecl(config gowdk.EnvConfig) []ast.Decl {
	if !envDefaultsRequired(config) {
		return nil
	}
	var stmts []ast.Stmt
	for _, variable := range config.Vars {
		if variable.Default == "" {
			continue
		}
		stmts = append(stmts, &ast.IfStmt{
			Init: define([]ast.Expr{id("value")}, call(sel("os", "Getenv"), stringLit(variable.Name))),
			Cond: &ast.BinaryExpr{
				X:  call(sel("strings", "TrimSpace"), id("value")),
				Op: token.EQL,
				Y:  stringLit(""),
			},
			Body: block(exprStmt(call(sel("os", "Setenv"), stringLit(variable.Name), stringLit(variable.Default)))),
		})
	}
	return []ast.Decl{funcDecl("applyEnvDefaults", nil, nil, stmts)}
}

func applyEnvDefaultsStmt(config gowdk.EnvConfig) []ast.Stmt {
	if !envDefaultsRequired(config) {
		return nil
	}
	return []ast.Stmt{exprStmt(call(id("applyEnvDefaults")))}
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
		if stmt := appendSecretEnvStmt(secret); stmt != nil {
			stmts = append(stmts, stmt)
		}
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

// appendSecretEnvStmt validates one secret env var. A required secret with no
// minimum keeps the existing empty-only check. A secret with MinBytes also
// rejects a present-but-too-short value, so a weak signing key fails the env
// contract at startup instead of deferring to the first request.
func appendSecretEnvStmt(secret gowdk.SecretEnv) ast.Stmt {
	if secret.MinBytes <= 0 {
		if secret.Required {
			return appendMissingEnvStmt(secret.Name)
		}
		return nil
	}
	appendMissing := func(message string) ast.Stmt {
		return assign([]ast.Expr{id("missing")}, call(id("append"), id("missing"), stringLit(message)))
	}
	getenv := define([]ast.Expr{id("value")}, call(sel("strings", "TrimSpace"), call(sel("os", "Getenv"), stringLit(secret.Name))))
	tooShort := &ast.BinaryExpr{X: call(id("len"), id("value")), Op: token.LSS, Y: intLit(secret.MinBytes)}
	shortStmt := block(appendMissing(fmt.Sprintf("%s must be at least %d bytes", secret.Name, secret.MinBytes)))
	if !secret.Required {
		// Optional secret: only enforce the minimum when a value is present.
		return &ast.IfStmt{
			Init: getenv,
			Cond: &ast.BinaryExpr{
				X:  &ast.BinaryExpr{X: id("value"), Op: token.NEQ, Y: stringLit("")},
				Op: token.LAND,
				Y:  tooShort,
			},
			Body: shortStmt,
		}
	}
	return &ast.IfStmt{
		Init: getenv,
		Cond: &ast.BinaryExpr{X: id("value"), Op: token.EQL, Y: stringLit("")},
		Body: block(appendMissing(secret.Name + " is required but is not set")),
		Else: &ast.IfStmt{Cond: tooShort, Body: shortStmt},
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
