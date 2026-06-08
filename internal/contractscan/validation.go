package contractscan

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func validateContracts(contracts []Contract, functions map[string]functionInfo) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		switch {
		case strings.TrimSpace(contract.Handler) == "":
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", fmt.Sprintf("%s registration must pass an exported handler function", contract.Kind)))
			continue
		case isLocalIdentifier(contract.Handler) && !ast.IsExported(contract.Handler):
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", fmt.Sprintf("%s handler %s must be exported", contract.Kind, contract.Handler)))
			continue
		}
		if !isLocalIdentifier(contract.Handler) && !isSelectorHandler(contract.Handler) {
			continue
		}
		function := functions[contract.Handler]
		if function.Signature == nil {
			if !isLocalIdentifier(contract.Handler) {
				continue
			}
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_missing", fmt.Sprintf("handler %s was not found in the scanned package", contract.Handler)))
			continue
		}
		if message := validateHandlerSignature(contract, function); message != "" {
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", message))
		}
	}
	return diagnostics
}

func duplicateCommandDiagnostics(contracts []Contract) []Diagnostic {
	byCommand := map[string][]Contract{}
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Command {
			continue
		}
		key := commandIdentity(contract)
		if key == "" {
			continue
		}
		byCommand[key] = append(byCommand[key], contract)
	}
	var diagnostics []Diagnostic
	for _, matches := range byCommand {
		if len(matches) < 2 {
			continue
		}
		handlers := make([]string, 0, len(matches))
		for _, match := range matches {
			handlers = append(handlers, emptyDiagnosticValue(match.Handler))
		}
		sort.Strings(handlers)
		for _, match := range matches[1:] {
			diagnostics = append(diagnostics, contractDiagnostic(match, "duplicate_command_owner", fmt.Sprintf("command %s has multiple owner registrations: %s", commandIdentity(match), strings.Join(handlers, ", "))))
		}
	}
	return diagnostics
}

func emittedEventCategoryDiagnostics(contracts []Contract) []Diagnostic {
	events := map[string]map[runtimecontracts.EventCategory]bool{}
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		key := eventIdentity(contract.TypeImportPath, contract.Type)
		if key == "" {
			continue
		}
		if events[key] == nil {
			events[key] = map[runtimecontracts.EventCategory]bool{}
		}
		events[key][contract.EventCategory] = true
	}
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Command || len(contract.Emits) == 0 {
			continue
		}
		for _, emit := range contract.Emits {
			key := eventIdentity(emit.TypeImportPath, emit.Type)
			registeredCategories := events[key]
			if len(registeredCategories) == 0 || registeredCategories[emit.Category] {
				continue
			}
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_event_category_invalid", fmt.Sprintf("command %s emits %s event %s but scanned registrations use event categories %s", contract.Type, emit.Category, emit.Type, eventCategoryList(registeredCategories))))
		}
	}
	return diagnostics
}

func eventIdentity(importPath string, typeName string) string {
	typeName = strings.TrimSpace(localContractName(typeName))
	if typeName == "" {
		return ""
	}
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return typeName
	}
	return importPath + "\x00" + typeName
}

func eventCategoryList(categories map[runtimecontracts.EventCategory]bool) string {
	values := make([]string, 0, len(categories))
	for category := range categories {
		values = append(values, string(category))
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func commandIdentity(contract Contract) string {
	if contract.Type == "" {
		return ""
	}
	if strings.Contains(contract.Type, ".") || contract.Package == "" {
		return contract.Type
	}
	return contract.Package + "." + contract.Type
}

func emptyDiagnosticValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unknown)"
	}
	return value
}

func validateHandlerSignature(contract Contract, function functionInfo) string {
	switch contract.Kind {
	case runtimecontracts.Query, runtimecontracts.Command:
		return validateRequestHandlerSignature(contract, function)
	case runtimecontracts.Event, runtimecontracts.Job:
		return validateEffectHandlerSignature(contract, function)
	default:
		return ""
	}
}

func validateRequestHandlerSignature(contract Contract, function functionInfo) string {
	signature := function.Signature
	if signature.Params().Len() != 2 {
		return fmt.Sprintf("%s handler %s must accept context.Context and %s", contract.Kind, contract.Handler, contract.Type)
	}
	if got := scanTypeString(signature.Params().At(0).Type(), function.Package); got != "context.Context" {
		return fmt.Sprintf("%s handler %s first parameter must be context.Context, got %s", contract.Kind, contract.Handler, got)
	}
	if got := scanTypeString(signature.Params().At(1).Type(), function.Package); got != contract.Type {
		return fmt.Sprintf("%s handler %s second parameter must be %s, got %s", contract.Kind, contract.Handler, contract.Type, got)
	}
	if signature.Results().Len() != 2 {
		return fmt.Sprintf("%s handler %s must return %s and error", contract.Kind, contract.Handler, contract.Result)
	}
	if got := scanTypeString(signature.Results().At(0).Type(), function.Package); got != contract.Result {
		return fmt.Sprintf("%s handler %s first result must be %s, got %s", contract.Kind, contract.Handler, contract.Result, got)
	}
	if got := scanTypeString(signature.Results().At(1).Type(), function.Package); got != "error" {
		return fmt.Sprintf("%s handler %s second result must be error, got %s", contract.Kind, contract.Handler, got)
	}
	return ""
}

func validateEffectHandlerSignature(contract Contract, function functionInfo) string {
	signature := function.Signature
	if signature.Params().Len() != 2 {
		return fmt.Sprintf("%s handler %s must accept context.Context and %s", contract.Kind, contract.Handler, contract.Type)
	}
	if got := scanTypeString(signature.Params().At(0).Type(), function.Package); got != "context.Context" {
		return fmt.Sprintf("%s handler %s first parameter must be context.Context, got %s", contract.Kind, contract.Handler, got)
	}
	if got := scanTypeString(signature.Params().At(1).Type(), function.Package); got != contract.Type {
		return fmt.Sprintf("%s handler %s second parameter must be %s, got %s", contract.Kind, contract.Handler, contract.Type, got)
	}
	if signature.Results().Len() != 1 {
		return fmt.Sprintf("%s handler %s must return error", contract.Kind, contract.Handler)
	}
	if got := scanTypeString(signature.Results().At(0).Type(), function.Package); got != "error" {
		return fmt.Sprintf("%s handler %s result must be error, got %s", contract.Kind, contract.Handler, got)
	}
	return ""
}

func scanTypeString(typ types.Type, local *types.Package) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		if local != nil && pkg.Path() == local.Path() {
			return ""
		}
		return pkg.Name()
	})
}

func isLocalIdentifier(value string) bool {
	if value == "" || strings.Contains(value, ".") {
		return false
	}
	return token.IsIdentifier(value)
}

func isSelectorHandler(value string) bool {
	qualifier, name, ok := strings.Cut(value, ".")
	return ok && token.IsIdentifier(qualifier) && token.IsIdentifier(name) && ast.IsExported(name)
}

func isSelectorName(value string) bool {
	qualifier, name, ok := strings.Cut(value, ".")
	return ok && token.IsIdentifier(qualifier) && token.IsIdentifier(name)
}
