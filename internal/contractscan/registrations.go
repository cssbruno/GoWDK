package contractscan

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func scanContractRegistrations(fset *token.FileSet, file *ast.File, aliases map[string]bool, imports map[string]string, source string) []Contract {
	var contracts []Contract
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		register := contractRegisterFunction(fn, aliases)
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
			kind, category, ok := registrationKind(selector.Sel.Name)
			if !ok {
				return true
			}
			position := fset.Position(call.Pos())
			contract := Contract{
				Kind:          kind,
				EventCategory: category,
				Package:       file.Name.Name,
				Source:        source,
				Line:          position.Line,
				Column:        position.Column,
				Handler:       handlerName(call),
				Register:      register,
				Roles:         roleNames(call),
			}
			if len(typeArgs) > 0 {
				contract.Type, contract.TypeImportPath = contractTypeName(fset, typeArgs[0], imports)
			}
			if len(typeArgs) > 1 {
				contract.Result, contract.ResultImportPath = contractTypeName(fset, typeArgs[1], imports)
			}
			contracts = append(contracts, contract)
			return true
		})
	}
	return contracts
}

func scanInvalidationRegistrations(fset *token.FileSet, file *ast.File, aliases map[string]bool, imports map[string]string, source string) []Invalidation {
	var invalidations []Invalidation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		register := contractRegisterFunction(fn, aliases)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, typeArgs := registrationSelector(call.Fun)
			if selector == nil || selector.Sel.Name != "RegisterInvalidation" || len(typeArgs) < 2 {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[ident.Name] {
				return true
			}
			position := fset.Position(call.Pos())
			eventType, eventImportPath := contractTypeName(fset, typeArgs[0], imports)
			queryType, queryImportPath := contractTypeName(fset, typeArgs[1], imports)
			invalidations = append(invalidations, Invalidation{
				EventCategory:       runtimecontracts.DomainEvent,
				EventType:           eventType,
				EventTypeImportPath: eventImportPath,
				QueryType:           queryType,
				QueryTypeImportPath: queryImportPath,
				Register:            register,
				Source:              source,
				Line:                position.Line,
				Column:              position.Column,
			})
			return true
		})
	}
	return invalidations
}

func contractTypeName(fset *token.FileSet, expr ast.Expr, imports map[string]string) (string, string) {
	name := exprString(fset, expr)
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil {
		return name, ""
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident == nil {
		return name, ""
	}
	return name, imports[ident.Name]
}

func validateContractTypes(contracts []Contract, types map[string]contractTypeInfo, importedTypes map[string]contractTypeInfo) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		diagnostics = append(diagnostics, validateContractType(contract, types, importedTypes)...)
		if contract.Kind == runtimecontracts.Command || contract.Kind == runtimecontracts.Query {
			diagnostics = append(diagnostics, validateContractResultType(contract, types, importedTypes)...)
		}
	}
	return diagnostics
}

func validateContractType(contract Contract, types map[string]contractTypeInfo, importedTypes map[string]contractTypeInfo) []Diagnostic {
	if strings.TrimSpace(contract.Type) == "" {
		return []Diagnostic{contractDiagnostic(contract, "contract_type_invalid", fmt.Sprintf("%s registration must declare a contract type", contract.Kind))}
	}
	return validateContractTypeName(contract, types, importedTypes, contract.Type, "contract_type_invalid", fmt.Sprintf("%s contract type", contract.Kind))
}

func validateContractResultType(contract Contract, types map[string]contractTypeInfo, importedTypes map[string]contractTypeInfo) []Diagnostic {
	if strings.TrimSpace(contract.Result) == "" {
		return []Diagnostic{contractDiagnostic(contract, "contract_result_invalid", fmt.Sprintf("%s registration must declare a result type", contract.Kind))}
	}
	return validateContractTypeName(contract, types, importedTypes, contract.Result, "contract_result_invalid", fmt.Sprintf("%s result type", contract.Kind))
}

func validateContractTypeName(contract Contract, types map[string]contractTypeInfo, importedTypes map[string]contractTypeInfo, name string, code string, label string) []Diagnostic {
	if isLocalIdentifier(name) {
		info, ok := types[name]
		return validateKnownContractType(contract, info, ok, name, code, label)
	}
	if !isSelectorName(name) {
		return nil
	}
	info, ok := importedTypes[name]
	if !ok {
		return nil
	}
	return validateKnownContractType(contract, info, true, name, code, label)
}

func validateKnownContractType(contract Contract, info contractTypeInfo, found bool, name string, code string, label string) []Diagnostic {
	if !found {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s was not found in the scanned package", label, name))}
	}
	if !info.Exported {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s must be exported", label, name))}
	}
	if !info.Struct {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s must be a struct", label, name))}
	}
	return nil
}

func validateEventNames(contracts []Contract) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		if message := eventNameMessage(contract); message != "" {
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_event_name_invalid", message))
		}
	}
	return diagnostics
}

func eventNameMessage(contract Contract) string {
	name := localContractName(contract.Type)
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "Changed") {
		return fmt.Sprintf("event %s is too vague; use a specific backend fact such as PatientCreated or PatientUpdated", name)
	}
	words := camelWords(name)
	if hasAnyWord(words, uiEventSubjects) && hasAnyWord(words, uiEventActions) {
		return fmt.Sprintf("event %s looks like a browser UI event; UI events must trigger commands or queries, not backend events", name)
	}
	return ""
}

func localContractName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	return name
}

var uiEventSubjects = map[string]bool{
	"Button":    true,
	"Checkbox":  true,
	"Component": true,
	"Dialog":    true,
	"Dropdown":  true,
	"Field":     true,
	"Form":      true,
	"Input":     true,
	"Modal":     true,
	"Page":      true,
	"Select":    true,
	"Tab":       true,
	"View":      true,
}

var uiEventActions = map[string]bool{
	"Blurred":   true,
	"Changed":   true,
	"Clicked":   true,
	"Closed":    true,
	"Focused":   true,
	"Hovered":   true,
	"Opened":    true,
	"Pressed":   true,
	"Selected":  true,
	"Submitted": true,
	"Toggled":   true,
	"Typed":     true,
}

func camelWords(value string) []string {
	var words []string
	start := 0
	var previous rune
	for index, current := range value {
		if index > 0 && previous >= 'a' && previous <= 'z' && current >= 'A' && current <= 'Z' {
			words = append(words, value[start:index])
			start = index
		}
		previous = current
	}
	if start < len(value) {
		words = append(words, value[start:])
	}
	return words
}

func hasAnyWord(words []string, choices map[string]bool) bool {
	for _, word := range words {
		if choices[word] {
			return true
		}
	}
	return false
}

func contractRegisterFunction(fn *ast.FuncDecl, aliases map[string]bool) string {
	if fn.Name == nil || fn.Name.Name == "init" || fn.Recv != nil || fn.Type == nil || fn.Type.Params == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		if isRegistryPointer(field.Type, aliases) {
			return fn.Name.Name
		}
	}
	return ""
}

func isRegistryPointer(expr ast.Expr, aliases map[string]bool) bool {
	pointer, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != "Registry" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && aliases[ident.Name]
}
