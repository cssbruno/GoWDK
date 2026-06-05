package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/manifest"
	"strings"
)

func validateComponentGoContracts(components []manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		diagnostics = append(diagnostics, validateComponentGoContract(component)...)
	}
	return diagnostics
}

type componentContracts struct {
	Props      map[string]bool
	PropTypes  map[string]clientlang.ValueType
	State      map[string]bool
	StateTypes map[string]clientlang.ValueType
}

type componentValidationContext struct {
	Props         map[string]bool
	State         map[string]bool
	StateTypes    map[string]clientlang.ValueType
	SymbolTypes   map[string]clientlang.ValueType
	ComputedTypes map[string]clientlang.ValueType
	Handlers      map[string]clientlang.Handler
	Helpers       map[string]clientlang.Helper
	Refs          map[string]clientlang.Ref
	UsedRefs      map[string]bool
}

func validateComponentGoContract(component manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validateComponentImports(component)...)
	if component.PropsType.Name != "" && len(component.Props) > 0 {
		return append(diagnostics, ValidationError{
			Code:          "component_contract_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.PropsType.Span, component.Span),
			Message:       fmt.Sprintf("component %s declares both typed props and props {}", component.Name),
		})
	}

	contracts, contractDiagnostics := resolveComponentContracts(component)
	diagnostics = append(diagnostics, contractDiagnostics...)

	symbolTypes := mergeTypeSymbols(contracts.PropTypes, contracts.StateTypes)
	handlers, helpers, refs, usedRefs, computedTypes, clientDiagnostics := validateComponentClient(component, contracts.StateTypes, symbolTypes)
	diagnostics = append(diagnostics, clientDiagnostics...)

	ctx := componentValidationContext{
		Props:         contracts.Props,
		State:         contracts.State,
		StateTypes:    contracts.StateTypes,
		SymbolTypes:   mergeTypeSymbols(symbolTypes, computedTypes),
		ComputedTypes: computedTypes,
		Handlers:      handlers,
		Helpers:       helpers,
		Refs:          refs,
		UsedRefs:      usedRefs,
	}
	diagnostics = append(diagnostics, validateComponentContractOverlap(component, contracts)...)
	diagnostics = append(diagnostics, validateComponentViewContract(component, ctx)...)
	return diagnostics
}

func resolveComponentContracts(component manifest.Component) (componentContracts, []ValidationError) {
	contracts := componentContracts{
		Props:      map[string]bool{},
		PropTypes:  map[string]clientlang.ValueType{},
		State:      map[string]bool{},
		StateTypes: map[string]clientlang.ValueType{},
	}
	for _, prop := range component.Props {
		contracts.Props[prop.Name] = true
		contracts.PropTypes[prop.Name] = clientlang.TypeString
	}

	var diagnostics []ValidationError
	if component.PropsType.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.PropsType)
		if err != nil {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.PropsType.Span, err))
		} else {
			addResolvedFields(contracts.Props, contracts.PropTypes, resolved)
		}
	}

	if component.State.Type.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.State.Type)
		if err != nil {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Type.Span, err))
		} else {
			addResolvedFields(contracts.State, contracts.StateTypes, resolved)
		}
		if err := gotypes.ValidateStateInit(component.Imports, component.State); err != nil {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Init.Span, err))
		}
	} else if component.State.Init.Name != "" {
		diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Init.Span, fmt.Errorf("component %s declares state init without a state type", component.Name)))
	}

	return contracts, diagnostics
}

func addResolvedFields(names map[string]bool, types map[string]clientlang.ValueType, resolved gotypes.Struct) {
	for _, field := range resolved.Fields {
		names[field.Name] = true
		types[field.Name] = clientlang.NormalizeType(field.Type)
	}
	for field, typ := range resolved.FieldTypes {
		types[field] = clientlang.NormalizeType(typ)
	}
}

func validateComponentContractOverlap(component manifest.Component, contracts componentContracts) []ValidationError {
	var diagnostics []ValidationError
	for name := range contracts.Props {
		if !contracts.State[name] {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_contract_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.PropsType.Span, component.State.Type.Span, component.Span),
			Message:       fmt.Sprintf("component %s declares %q in both props and state contracts", component.Name, name),
		})
	}
	return diagnostics
}

func validateComponentImports(component manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	seen := map[string]manifest.Import{}
	for _, item := range component.Imports {
		if err := gotypes.ValidateImportPath(item.Path); err != nil {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "invalid_go_import", item.Span, err))
			continue
		}
		alias, err := gotypes.EffectiveImportAlias(item)
		if err != nil {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "invalid_go_import", item.Span, err))
			continue
		}
		if first, exists := seen[alias]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "duplicate_go_import_alias",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          item.Span,
				Message:       duplicateIdentityMessage("component import alias", alias, importSource(component.Source, first), importSource(component.Source, item)),
			})
			continue
		}
		seen[alias] = item
	}
	return diagnostics
}

func componentContractDiagnostic(component manifest.Component, code string, span manifest.SourceSpan, err error) ValidationError {
	return ValidationError{
		Code:          code,
		ComponentName: component.Name,
		Source:        component.Source,
		Span:          firstSpan(span, component.Span),
		Message:       fmt.Sprintf("component %s: %v", component.Name, err),
	}
}

func importSource(source string, item manifest.Import) string {
	if source == "" {
		return ""
	}
	name := item.Alias
	if strings.TrimSpace(name) == "" {
		name = item.Path
	}
	return fmt.Sprintf("%s import %s", source, name)
}
