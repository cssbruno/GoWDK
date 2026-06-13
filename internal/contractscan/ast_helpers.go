package contractscan

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	pathpkg "path"
	"sort"
	"strconv"

	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func contractDiagnostic(contract Contract, code string, message string) Diagnostic {
	return Diagnostic{
		Severity:       "error",
		Code:           code,
		Kind:           contract.Kind,
		Package:        contract.Package,
		Type:           contract.Type,
		TypeImportPath: contract.TypeImportPath,
		Handler:        contract.Handler,
		Source:         contract.Source,
		Line:           contract.Line,
		Column:         contract.Column,
		Message:        message,
	}
}

func contractsImportAliases(file *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, importSpec := range file.Imports {
		path, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || path != RuntimeImportPath {
			continue
		}
		switch {
		case importSpec.Name == nil:
			aliases["contracts"] = true
		case importSpec.Name.Name != "." && importSpec.Name.Name != "_":
			aliases[importSpec.Name.Name] = true
		}
	}
	return aliases
}

func goImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || importPath == "" {
			continue
		}
		alias := ""
		switch {
		case importSpec.Name == nil:
			alias = pathpkg.Base(importPath)
		case importSpec.Name.Name == "." || importSpec.Name.Name == "_":
			continue
		default:
			alias = importSpec.Name.Name
		}
		if alias != "" {
			aliases[alias] = importPath
		}
	}
	return aliases
}

func registrationSelector(expr ast.Expr) (*ast.SelectorExpr, []ast.Expr) {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return typed, nil
	case *ast.IndexExpr:
		selector, _ := typed.X.(*ast.SelectorExpr)
		return selector, []ast.Expr{typed.Index}
	case *ast.IndexListExpr:
		selector, _ := typed.X.(*ast.SelectorExpr)
		return selector, typed.Indices
	default:
		return nil, nil
	}
}

func registrationKind(name string) (runtimecontracts.Kind, runtimecontracts.EventCategory, bool) {
	switch name {
	case "RegisterQuery":
		return runtimecontracts.Query, "", true
	case "RegisterCommand":
		return runtimecontracts.Command, "", true
	case "RegisterDomainEvent":
		return runtimecontracts.Event, runtimecontracts.DomainEvent, true
	case "RegisterIntegrationEvent":
		return runtimecontracts.Event, runtimecontracts.IntegrationEvent, true
	case "RegisterPresentationEvent":
		return runtimecontracts.Event, runtimecontracts.PresentationEvent, true
	case "RegisterJob":
		return runtimecontracts.Job, "", true
	default:
		return "", "", false
	}
}

func emittedEventsByHandler(fset *token.FileSet, file *ast.File, aliases map[string]bool, imports map[string]string) map[string][]EventRef {
	out := map[string][]EventRef{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		seen := map[EventRef]bool{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, typeArgs := registrationSelector(call.Fun)
			if selector == nil {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[ident.Name] {
				return true
			}
			category, ok := emitCategory(selector.Sel.Name)
			if !ok {
				return true
			}
			eventType, eventImportPath := emittedEventType(fset, call, typeArgs, imports)
			if eventType == "" {
				return true
			}
			ref := EventRef{Category: category, Type: eventType, TypeImportPath: eventImportPath}
			if !seen[ref] {
				out[fn.Name.Name] = append(out[fn.Name.Name], ref)
				seen[ref] = true
			}
			return true
		})
	}
	for handler := range out {
		sort.Slice(out[handler], func(i, j int) bool {
			if out[handler][i].Category != out[handler][j].Category {
				return out[handler][i].Category < out[handler][j].Category
			}
			return out[handler][i].Type < out[handler][j].Type
		})
	}
	return out
}

func emitCategory(name string) (runtimecontracts.EventCategory, bool) {
	switch name {
	case "EmitDomain":
		return runtimecontracts.DomainEvent, true
	case "EmitIntegration":
		return runtimecontracts.IntegrationEvent, true
	case "EmitPresentation":
		return runtimecontracts.PresentationEvent, true
	default:
		return "", false
	}
}

func emittedEventType(fset *token.FileSet, call *ast.CallExpr, typeArgs []ast.Expr, imports map[string]string) (string, string) {
	if len(typeArgs) > 0 {
		return contractTypeName(fset, typeArgs[0], imports)
	}
	if len(call.Args) < 2 {
		return "", ""
	}
	switch arg := call.Args[1].(type) {
	case *ast.CompositeLit:
		return contractTypeName(fset, arg.Type, imports)
	case *ast.UnaryExpr:
		if literal, ok := arg.X.(*ast.CompositeLit); ok {
			return contractTypeName(fset, literal.Type, imports)
		}
	}
	return "", ""
}

func handlerName(call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	return exprString(token.NewFileSet(), call.Args[1])
}

func roleNames(call *ast.CallExpr) []string {
	if len(call.Args) <= 2 {
		return nil
	}
	roles := make([]string, 0, len(call.Args)-2)
	for _, arg := range call.Args[2:] {
		roles = append(roles, roleName(arg))
	}
	return roles
}

func roleName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		switch typed.Sel.Name {
		case "RoleWeb":
			return string(runtimecontracts.RoleWeb)
		case "RoleWorker":
			return string(runtimecontracts.RoleWorker)
		case "RoleCron":
			return string(runtimecontracts.RoleCron)
		case "RoleAPI":
			return string(runtimecontracts.RoleAPI)
		case "RoleAdmin":
			return string(runtimecontracts.RoleAdmin)
		}
	case *ast.BasicLit:
		value, err := strconv.Unquote(typed.Value)
		if err == nil {
			return value
		}
	}
	return exprString(token.NewFileSet(), expr)
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, fset, expr); err != nil {
		return fmt.Sprint(expr)
	}
	return buffer.String()
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".gowdk", ".claude", "vendor", "node_modules", "dist", "bin":
		// .claude holds tooling state and nested git worktrees
		// (.claude/worktrees/*); scanning them double-counts a sibling
		// checkout's contract registrations as duplicates of this tree's.
		return true
	default:
		return false
	}
}

func copyEventRefs(values []EventRef) []EventRef {
	if len(values) == 0 {
		return nil
	}
	copied := make([]EventRef, len(values))
	copy(copied, values)
	return copied
}
