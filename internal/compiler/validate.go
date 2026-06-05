package compiler

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

var (
	cssReferencePattern                 = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	componentSimpleInterpolationPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// ValidationError is a compiler diagnostic that can be shown to users.
type ValidationError struct {
	Code          string
	PageID        string
	ComponentName string
	Source        string
	Span          manifest.SourceSpan
	Message       string
}

func (err ValidationError) Error() string {
	if err.PageID == "" {
		if err.ComponentName != "" {
			return fmt.Sprintf("%s: %s", err.ComponentName, err.Message)
		}
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.PageID, err.Message)
}

// ValidateManifest checks render-mode invariants that must hold before codegen.
func ValidateManifest(config gowdk.Config, app manifest.Manifest) error {
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validateUniquePages(app.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(app.Components)...)
	diagnostics = append(diagnostics, validateComponentGoContracts(app.Components)...)
	diagnostics = append(diagnostics, validateRedundantComponents(app.Components)...)
	diagnostics = append(diagnostics, validateUniqueLayouts(app.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(app.Pages, app.Layouts)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(app.Pages)...)
	for _, page := range app.Pages {
		diagnostics = append(diagnostics, ValidatePage(config, page)...)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}

func validateUniquePages(pages []manifest.Page) []ValidationError {
	seen := map[string]manifest.Page{}
	var diagnostics []ValidationError
	for _, page := range pages {
		if page.ID == "" {
			continue
		}
		first, exists := seen[page.ID]
		if !exists {
			seen[page.ID] = page
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "duplicate_page_id",
			PageID: page.ID,
			Source: page.Source,
			Span:   page.Spans.Page,
			Message: duplicateIdentityMessage(
				"page ID",
				page.ID,
				first.Source,
				page.Source,
			),
		})
	}
	return diagnostics
}

func validateUniqueLayouts(layouts []manifest.Layout) []ValidationError {
	seen := map[string]manifest.Layout{}
	var diagnostics []ValidationError
	for _, layout := range layouts {
		if layout.ID == "" {
			continue
		}
		first, exists := seen[layout.ID]
		if !exists {
			seen[layout.ID] = layout
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "duplicate_layout_id",
			Source: layout.Source,
			Span:   layout.Span,
			Message: duplicateIdentityMessage(
				"layout ID",
				layout.ID,
				first.Source,
				layout.Source,
			),
		})
	}
	return diagnostics
}

func validatePageLayoutReferences(pages []manifest.Page, layouts []manifest.Layout) []ValidationError {
	if len(layouts) == 0 {
		return nil
	}
	declared := map[string]bool{}
	for _, layout := range layouts {
		if layout.ID != "" {
			declared[layout.ID] = true
		}
	}
	var diagnostics []ValidationError
	for _, page := range pages {
		for _, layoutID := range page.Layouts {
			if declared[layoutID] {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unknown_layout_id",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.Layouts, layoutID, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s references layout %q, but no .layout.gwdk file declares @layout %s",
					page.ID,
					layoutID,
					layoutID,
				),
			})
		}
	}
	return diagnostics
}

func validateUniqueComponents(components []manifest.Component) []ValidationError {
	seen := map[string]manifest.Component{}
	var diagnostics []ValidationError
	for _, component := range components {
		if component.Name == "" {
			continue
		}
		first, exists := seen[component.Name]
		if !exists {
			seen[component.Name] = component
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "duplicate_component_name",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          component.Span,
			Message: duplicateIdentityMessage(
				"component name",
				component.Name,
				first.Source,
				component.Source,
			),
		})
	}
	return diagnostics
}

func validateComponentGoContracts(components []manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		diagnostics = append(diagnostics, validateComponentImports(component)...)
		if component.PropsType.Name != "" && len(component.Props) > 0 {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_contract_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.PropsType.Span, component.Span),
				Message:       fmt.Sprintf("component %s declares both typed props and props {}", component.Name),
			})
			continue
		}

		props := map[string]bool{}
		propTypes := map[string]clientlang.ValueType{}
		for _, prop := range component.Props {
			props[prop.Name] = true
			propTypes[prop.Name] = clientlang.TypeString
		}
		if component.PropsType.Name != "" {
			resolved, err := gotypes.ResolveStruct(component.Imports, component.PropsType)
			if err != nil {
				diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.PropsType.Span, err))
			} else {
				for _, field := range resolved.Fields {
					props[field.Name] = true
					propTypes[field.Name] = clientlang.NormalizeType(field.Type)
				}
				for field, typ := range resolved.FieldTypes {
					propTypes[field] = clientlang.NormalizeType(typ)
				}
			}
		}

		state := map[string]bool{}
		stateTypes := map[string]clientlang.ValueType{}
		if component.State.Type.Name != "" {
			resolved, err := gotypes.ResolveStruct(component.Imports, component.State.Type)
			if err != nil {
				diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Type.Span, err))
			} else {
				for _, field := range resolved.Fields {
					state[field.Name] = true
					stateTypes[field.Name] = clientlang.NormalizeType(field.Type)
				}
				for field, typ := range resolved.FieldTypes {
					stateTypes[field] = clientlang.NormalizeType(typ)
				}
			}
			if err := gotypes.ValidateStateInit(component.Imports, component.State); err != nil {
				diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Init.Span, err))
			}
		} else if component.State.Init.Name != "" {
			diagnostics = append(diagnostics, componentContractDiagnostic(component, "component_contract_error", component.State.Init.Span, fmt.Errorf("component %s declares state init without a state type", component.Name)))
		}

		symbolTypes := mergeTypeSymbols(propTypes, stateTypes)
		handlers, helpers, refs, usedRefs, computedTypes, clientDiagnostics := validateComponentClient(component, stateTypes, symbolTypes)
		diagnostics = append(diagnostics, clientDiagnostics...)
		symbolTypes = mergeTypeSymbols(symbolTypes, computedTypes)

		for name := range props {
			if !state[name] {
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

		fieldRefs, eventExprs, boolExprs, attrExprs, classToggles, styleBindings, valueBinds, checkedBinds, refBinds, err := componentViewReferences(component.Blocks.ViewBody)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.View, component.Span),
				Message:       fmt.Sprintf("component %s view is invalid: %v", component.Name, err),
			})
			continue
		}
		helperFuncs := helperExprFunctions(helpers)
		emits := componentEmitMap(component)
		diagnostics = append(diagnostics, validateComponentListDirectives(component, symbolTypes, stateTypes, handlers, helperFuncs)...)
		for field := range fieldRefs {
			if props[field] || state[field] {
				continue
			}
			if _, ok := computedTypes[field]; ok {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.View, component.Span),
				Message:       fmt.Sprintf("component %s view references unknown field %q", component.Name, field),
			})
		}
		for _, eventExpr := range eventExprs {
			if _, err := view.ParseEventDirective(eventExpr.Name); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s event directive %q is invalid: %v", component.Name, eventExpr.Name, err),
				})
				continue
			}
			if err := view.ValidateIslandEventExpressionTypedWithEvents(eventExpr.Expression, symbolTypes, stateTypes, handlers, helperFuncs, emits); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s event expression %q is invalid: %v", component.Name, eventExpr.Expression, err),
				})
			}
		}
		for _, expr := range boolExprs {
			if err := view.ValidateIslandBoolExpressionTyped(expr, symbolTypes); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s bool expression %q is invalid: %v", component.Name, expr, err),
				})
			}
		}
		for _, attrExpr := range attrExprs {
			if err := view.ValidateReactiveAttrExpressionTyped(attrExpr.Name, attrExpr.Expression, symbolTypes); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s reactive attribute %s=%q is invalid: %v", component.Name, attrExpr.Name, attrExpr.Expression, err),
				})
			}
		}
		for _, toggle := range classToggles {
			if err := view.ValidateClassToggleExpressionTyped(toggle.Name, toggle.Expression, symbolTypes); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s class toggle %s=%q is invalid: %v", component.Name, toggle.Name, toggle.Expression, err),
				})
			}
		}
		for _, binding := range styleBindings {
			if err := view.ValidateStyleBindingExpressionTyped(binding.Name, binding.Expression, symbolTypes); err != nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s style binding %s=%q is invalid: %v", component.Name, binding.Name, binding.Expression, err),
				})
			}
		}
		for _, field := range valueBinds {
			typ, ok := stateTypes[field.Field]
			if !ok {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:value target %q must be a state field", component.Name, field.Field),
				})
				continue
			}
			if !isValueBindableElement(field.Element) {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:value target %q is on unsupported <%s>", component.Name, field.Field, field.Element),
				})
				continue
			}
			if field.Element == "input" && strings.EqualFold(field.InputType, "radio") && field.InputValue == "" {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:value radio target %q requires a static value attribute", component.Name, field.Field),
				})
				continue
			}
			if typ == clientlang.TypeString || typ == clientlang.TypeUnknown {
				continue
			}
			if typ == clientlang.TypeInt || typ == clientlang.TypeFloat {
				if field.Element == "input" && strings.EqualFold(field.InputType, "number") {
					continue
				}
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:value numeric target %q requires <input type=\"number\">", component.Name, field.Field),
				})
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.View, component.Span),
				Message:       fmt.Sprintf("component %s g:bind:value target %q must be string or numeric, got %s", component.Name, field.Field, typ),
			})
		}
		for _, field := range checkedBinds {
			typ, ok := stateTypes[field]
			if !ok {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:checked target %q must be a state field", component.Name, field),
				})
				continue
			}
			if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:bind:checked target %q must be bool, got %s", component.Name, field, typ),
				})
			}
		}
		boundRefs := map[string]bool{}
		for _, refName := range refBinds {
			if refs == nil {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:ref target %q is not declared", component.Name, refName),
				})
				continue
			}
			if _, ok := refs[refName]; !ok {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:ref target %q is not declared", component.Name, refName),
				})
				continue
			}
			if boundRefs[refName] {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_field_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message:       fmt.Sprintf("component %s g:ref target %q is bound more than once", component.Name, refName),
				})
				continue
			}
			boundRefs[refName] = true
		}
		for refName := range usedRefs {
			if !boundRefs[refName] {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "component_client_error",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
					Message:       fmt.Sprintf("component %s DOM ref %q is used but not bound with g:ref", component.Name, refName),
				})
			}
		}
	}
	return diagnostics
}

func validateComponentClient(component manifest.Component, stateTypes map[string]clientlang.ValueType, symbolTypes map[string]clientlang.ValueType) (map[string]clientlang.Handler, map[string]clientlang.Helper, map[string]clientlang.Ref, map[string]bool, map[string]clientlang.ValueType, []ValidationError) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil, nil, nil, nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, nil, nil, nil, nil, []ValidationError{{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
			Message:       fmt.Sprintf("component %s client block is invalid: %v", component.Name, err),
		}}
	}
	handlers := program.HandlerMap()
	helpers := program.HelperMap()
	helperFuncs := helperExprFunctions(helpers)
	emits := componentEmitMap(component)
	refs := program.RefMap()
	usedRefs := map[string]bool{}
	computedTypes := map[string]clientlang.ValueType{}
	var diagnostics []ValidationError
	readSymbols := mergeTypeSymbols(nil, symbolTypes)
	for _, computed := range program.Computed {
		if _, exists := symbolTypes[computed.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %q conflicts with a prop or state field", component.Name, computed.Name),
			})
			continue
		}
		if _, exists := computedTypes[computed.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %q is declared more than once", component.Name, computed.Name),
			})
			continue
		}
		declared := clientlang.NormalizeType(computed.Type)
		computedTypes[computed.Name] = declared
		readSymbols[computed.Name] = declared
	}
	orderedComputeds, err := program.OrderedComputed()
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
			Message:       fmt.Sprintf("component %s computed dependency graph is invalid: %v", component.Name, err),
		})
	}
	for _, computed := range orderedComputeds {
		typ, _, err := clientlang.CheckExpr(computed.Expr, readSymbols)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientExpressionErrorSpan(component, "return "+computed.Expr, computed.ExprSpan, err),
				Message:       fmt.Sprintf("component %s computed %s expression %q is invalid: %v", component.Name, computed.Name, computed.Expr, err),
			})
			continue
		}
		declared := clientlang.NormalizeType(computed.Type)
		if declared != clientlang.TypeUnknown && typ != clientlang.TypeUnknown && declared != typ && !compatibleNumericType(typ, declared) {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %s returns %s, not %s", component.Name, computed.Name, typ, declared),
			})
			continue
		}
	}
	if err := validateHelperCallGraph(helpers); err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
			Message:       fmt.Sprintf("component %s helper call graph is invalid: %v", component.Name, err),
		})
	}
	for _, function := range program.Functions {
		if function.ReturnType == "" {
			continue
		}
		helper := helpers[function.Name]
		readFields := mergeTypeSymbols(nil, readSymbols)
		for _, param := range function.Params {
			readFields[param.Name] = clientlang.NormalizeType(param.Type)
		}
		actual, _, err := clientlang.CheckExprWithFunctions(helper.Return, readFields, helperFuncs)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientExpressionErrorSpan(component, function.Statements[len(function.Statements)-1], functionReturnSpan(function), err),
				Message:       fmt.Sprintf("component %s helper function %s return expression %q is invalid: %v", component.Name, function.Name, helper.Return, err),
			})
			continue
		}
		declared := helper.ReturnType
		if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s helper function %s cannot return %s expression", component.Name, function.Name, actual),
			})
			continue
		}
		if declared != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && declared != actual && !compatibleNumericType(actual, declared) {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s helper function %s returns %s, not %s", component.Name, function.Name, actual, declared),
			})
		}
	}
	for _, function := range program.Functions {
		if function.ReturnType != "" {
			continue
		}
		readFields := mergeTypeSymbols(nil, readSymbols)
		for _, param := range function.Params {
			readFields[param.Name] = clientlang.NormalizeType(param.Type)
		}
		functionRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(function.Statements, stateTypes, readFields, refs, helperFuncs, function.Async, emits)
		for refName := range functionRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, function.Statements, function.StatementSpans, err),
				Message:       fmt.Sprintf("component %s client function %s is invalid: %v", component.Name, function.Name, err),
			})
		}
	}
	mountRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(program.Mount, stateTypes, readSymbols, refs, helperFuncs, false, emits)
	for refName := range mountRefs {
		usedRefs[refName] = true
	}
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          clientStatementErrorSpan(component, program.Mount, program.MountSpans, err),
			Message:       fmt.Sprintf("component %s mount block is invalid: %v", component.Name, err),
		})
	}
	destroyRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(program.Destroy, stateTypes, readSymbols, refs, helperFuncs, false, emits)
	for refName := range destroyRefs {
		usedRefs[refName] = true
	}
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          clientStatementErrorSpan(component, program.Destroy, program.DestroySpans, err),
			Message:       fmt.Sprintf("component %s destroy block is invalid: %v", component.Name, err),
		})
	}
	for _, effect := range program.Effects {
		if _, ok := stateTypes[effect.Field]; !ok {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s effect dependency %q must be a state field", component.Name, effect.Field),
			})
		}
		effectRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(effect.Statements, stateTypes, readSymbols, refs, helperFuncs, false, emits)
		for refName := range effectRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, effect.Statements, effect.StatementSpans, err),
				Message:       fmt.Sprintf("component %s effect block for %q is invalid: %v", component.Name, effect.Field, err),
			})
		}
		cleanupRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(effect.Cleanup, stateTypes, readSymbols, refs, helperFuncs, false, emits)
		for refName := range cleanupRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, effect.Cleanup, effect.CleanupSpans, err),
				Message:       fmt.Sprintf("component %s effect cleanup for %q is invalid: %v", component.Name, effect.Field, err),
			})
		}
	}
	return handlers, helpers, refs, usedRefs, computedTypes, diagnostics
}

func componentEmitMap(component manifest.Component) map[string]clientlang.Emit {
	if len(component.Emits) == 0 {
		return nil
	}
	out := map[string]clientlang.Emit{}
	for _, event := range component.Emits {
		params := make([]string, 0, len(event.Params))
		paramTypes := make([]clientlang.ValueType, 0, len(event.Params))
		for _, param := range event.Params {
			params = append(params, param.Name)
			paramTypes = append(paramTypes, clientlang.NormalizeType(param.Type))
		}
		out[event.Name] = clientlang.Emit{Name: event.Name, Params: params, ParamTypes: paramTypes}
	}
	return out
}

func clientStatementErrorSpan(component manifest.Component, statements []string, spans []clientlang.Span, err error) manifest.SourceSpan {
	var statementErr view.StatementValidationError
	if errors.As(err, &statementErr) && statementErr.Index >= 0 && statementErr.Index < len(spans) {
		if statementErr.Index < len(statements) {
			return clientExpressionErrorSpan(component, statements[statementErr.Index], spans[statementErr.Index], statementErr.Err)
		}
		return clientSpan(component, spans[statementErr.Index])
	}
	return firstSpan(component.Blocks.Spans.Client, component.Span)
}

func clientExpressionErrorSpan(component manifest.Component, statement string, span clientlang.Span, err error) manifest.SourceSpan {
	var exprErr clientlang.ExprValidationError
	if !errors.As(err, &exprErr) || exprErr.Span.StartColumn <= 0 {
		return clientSpan(component, span)
	}
	exprStart := expressionStartColumn(statement)
	if exprStart <= 0 {
		return clientSpan(component, span)
	}
	return clientSpanColumns(component, span, exprStart+exprErr.Span.StartColumn-1, exprStart+exprErr.Span.EndColumn-1)
}

func expressionStartColumn(statement string) int {
	trimmed := strings.TrimSpace(statement)
	if strings.HasPrefix(trimmed, "return ") {
		return strings.Index(statement, "return") + len("return") + 2
	}
	if index := strings.Index(statement, "="); index >= 0 {
		column := index + 2
		for column <= len(statement) && statement[column-1] == ' ' {
			column++
		}
		return column
	}
	return 0
}

func functionReturnSpan(function clientlang.Function) clientlang.Span {
	if len(function.StatementSpans) == 0 {
		return function.Span
	}
	return function.StatementSpans[len(function.StatementSpans)-1]
}

func clientSpan(component manifest.Component, span clientlang.Span) manifest.SourceSpan {
	return clientSpanColumns(component, span, 1, 2)
}

func clientSpanColumns(component manifest.Component, span clientlang.Span, startColumn, endColumn int) manifest.SourceSpan {
	if span.StartLine <= 0 {
		return firstSpan(component.Blocks.Spans.Client, component.Span)
	}
	base := component.Blocks.Spans.Client.Start.Line
	if base <= 0 {
		return firstSpan(component.Blocks.Spans.Client, component.Span)
	}
	startLine := base + span.StartLine
	endLine := base + span.EndLine
	if endLine < startLine {
		endLine = startLine
	}
	if startColumn <= 0 {
		startColumn = 1
	}
	if endColumn <= startColumn {
		endColumn = startColumn + 1
	}
	return manifest.SourceSpan{
		Start: manifest.SourcePosition{Line: startLine, Column: startColumn},
		End:   manifest.SourcePosition{Line: endLine, Column: endColumn},
	}
}

func mergeTypeSymbols(left, right map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	output := map[string]clientlang.ValueType{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}

func helperExprFunctions(helpers map[string]clientlang.Helper) map[string]clientlang.ExprFunction {
	if len(helpers) == 0 {
		return nil
	}
	out := map[string]clientlang.ExprFunction{}
	for name, helper := range helpers {
		out[name] = clientlang.ExprFunction{
			Params: append([]clientlang.ValueType(nil), helper.ParamTypes...),
			Return: helper.ReturnType,
		}
	}
	return out
}

func validateHelperCallGraph(helpers map[string]clientlang.Helper) error {
	if len(helpers) == 0 {
		return nil
	}
	graph := map[string][]string{}
	for name, helper := range helpers {
		calls, err := clientlang.ExprCalls(helper.Return)
		if err != nil {
			return fmt.Errorf("%s return expression: %w", name, err)
		}
		for _, call := range calls {
			if _, ok := helpers[call]; ok {
				graph[name] = append(graph[name], call)
			}
		}
	}
	state := map[string]int{}
	var stack []string
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("cycle %s", helperCycle(stack, name))
		case 2:
			return nil
		}
		state[name] = 1
		stack = append(stack, name)
		for _, next := range graph[name] {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
		return nil
	}
	for name := range helpers {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func helperCycle(stack []string, repeated string) string {
	start := 0
	for index, name := range stack {
		if name == repeated {
			start = index
			break
		}
	}
	cycle := append([]string(nil), stack[start:]...)
	cycle = append(cycle, repeated)
	return strings.Join(cycle, " -> ")
}

func compatibleNumericType(actual, expected clientlang.ValueType) bool {
	if actual == clientlang.TypeUnknown || expected == clientlang.TypeUnknown {
		return true
	}
	return (actual == clientlang.TypeInt || actual == clientlang.TypeFloat) &&
		(expected == clientlang.TypeInt || expected == clientlang.TypeFloat)
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

func validateRedundantComponents(components []manifest.Component) []ValidationError {
	seenNames := map[string]bool{}
	seen := map[string]manifest.Component{}
	var diagnostics []ValidationError
	for _, component := range components {
		if component.Name == "" || seenNames[component.Name] {
			continue
		}
		seenNames[component.Name] = true
		fingerprint := componentFingerprint(component)
		if fingerprint == "" {
			continue
		}
		first, exists := seen[fingerprint]
		if !exists {
			seen[fingerprint] = component
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "redundant_component_implementation",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Span, component.Blocks.Spans.View),
			Message: fmt.Sprintf(
				"component %q duplicates implementation of component %q; first declared in %s and duplicated in %s",
				component.Name,
				first.Name,
				first.Source,
				component.Source,
			),
		})
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

type reactiveAttrExpr struct {
	Name       string
	Expression string
}

type eventExpr struct {
	Name       string
	Expression string
}

type classToggleExpr struct {
	Name       string
	Expression string
}

type styleBindingExpr struct {
	Name       string
	Expression string
}

type valueBindExpr struct {
	Field      string
	Element    string
	InputType  string
	InputValue string
}

func componentViewReferences(source string) (map[string]bool, []eventExpr, []string, []reactiveAttrExpr, []classToggleExpr, []styleBindingExpr, []valueBindExpr, []string, []string, error) {
	nodes, err := view.Parse(source)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	fields := map[string]bool{}
	var events []eventExpr
	var bools []string
	var attrs []reactiveAttrExpr
	var classToggles []classToggleExpr
	var styleBindings []styleBindingExpr
	var valueBinds []valueBindExpr
	var checkedBinds []string
	var refBinds []string
	collectComponentViewReferences(nodes, fields, &events, &bools, &attrs, &classToggles, &styleBindings, &valueBinds, &checkedBinds, &refBinds)
	return fields, events, bools, attrs, classToggles, styleBindings, valueBinds, checkedBinds, refBinds, nil
}

func validateComponentListDirectives(component manifest.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) []ValidationError {
	nodes, err := view.Parse(component.Blocks.ViewBody)
	if err != nil {
		return nil
	}
	var messages []string
	validateListNodes(nodes, symbols, stateTypes, handlers, helpers, &messages)
	if len(messages) == 0 {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(messages))
	for _, message := range messages {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_field_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.View, component.Span),
			Message:       fmt.Sprintf("component %s list rendering is invalid: %s", component.Name, message),
		})
	}
	return diagnostics
}

func validateListNodes(nodes []view.Node, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]string) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
				validateListElement(typed, loopAttr, symbols, stateTypes, handlers, helpers, messages)
				continue
			}
			validateListNodes(typed.Children, symbols, stateTypes, handlers, helpers, messages)
		case view.ComponentCall:
			validateListNodes(typed.Children, symbols, stateTypes, handlers, helpers, messages)
		}
	}
}

func validateListElement(element view.Element, loopAttr view.Attr, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]string) {
	loop, err := view.ParseForDirective(loopAttr.Value)
	if err != nil {
		*messages = append(*messages, err.Error())
		return
	}
	collectionType, _, err := clientlang.CheckExpr(loop.Collection, symbols)
	if err != nil {
		*messages = append(*messages, fmt.Sprintf("g:for collection %q is invalid: %v", loop.Collection, err))
		return
	}
	if collectionType != clientlang.TypeArray && collectionType != clientlang.TypeUnknown {
		*messages = append(*messages, fmt.Sprintf("g:for collection %q must be array, got %s", loop.Collection, collectionType))
		return
	}
	keyExpr, ok := elementKeyExpression(element)
	if !ok {
		*messages = append(*messages, "g:for requires g:key for mutable lists")
		return
	}
	loopSymbols := loopSymbols(symbols, loop)
	keyType, _, err := clientlang.CheckExpr(keyExpr, loopSymbols)
	if err != nil {
		*messages = append(*messages, fmt.Sprintf("g:key %q is invalid: %v", keyExpr, err))
		return
	}
	if keyType == clientlang.TypeArray || keyType == clientlang.TypeObject || keyType == clientlang.TypeNil {
		*messages = append(*messages, fmt.Sprintf("g:key %q must be scalar, got %s", keyExpr, keyType))
		return
	}
	validateLoopElementBody(element, loopSymbols, stateTypes, handlers, helpers, messages)
}

func validateLoopSubtree(node view.Node, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]string) {
	switch typed := node.(type) {
	case view.Text:
		validateInterpolations(typed.Value, readSymbols, messages)
	case view.Element:
		if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
			validateListElement(typed, loopAttr, readSymbols, writeSymbols, handlers, helpers, messages)
			return
		}
		validateLoopElementBody(typed, readSymbols, writeSymbols, handlers, helpers, messages)
	case view.ComponentCall:
		for _, attr := range typed.Attrs {
			if strings.HasPrefix(attr.Name, "g:") {
				continue
			}
			validateInterpolations(attr.Value, readSymbols, messages)
		}
		for _, child := range typed.Children {
			validateLoopSubtree(child, readSymbols, writeSymbols, handlers, helpers, messages)
		}
	}
}

func validateLoopElementBody(element view.Element, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]string) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" || attr.Name == "g:key" {
			continue
		}
		switch {
		case strings.HasPrefix(attr.Name, "g:on:"):
			if err := view.ValidateIslandEventExpressionTypedWithFunctions(attr.Value, readSymbols, writeSymbols, handlers, helpers); err != nil {
				*messages = append(*messages, fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err))
			}
		case attr.Name == "g:if" || attr.Name == "g:else-if":
			if err := view.ValidateIslandBoolExpressionTyped(strings.TrimSpace(attr.Value), readSymbols); err != nil {
				*messages = append(*messages, fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err))
			}
		case attr.Name == "g:bind:value":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, fmt.Sprintf("g:bind:value target %q must be a state field", strings.TrimSpace(attr.Value)))
			}
		case attr.Name == "g:bind:checked":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, fmt.Sprintf("g:bind:checked target %q must be a state field", strings.TrimSpace(attr.Value)))
			}
		case strings.HasPrefix(attr.Name, "class:"):
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateClassToggleExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err))
			}
		case strings.HasPrefix(attr.Name, "style:"):
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateStyleBindingExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err))
			}
		case attr.Expression:
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateReactiveAttrExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err))
			}
		default:
			validateInterpolations(attr.Value, readSymbols, messages)
		}
	}
	for _, child := range element.Children {
		validateLoopSubtree(child, readSymbols, writeSymbols, handlers, helpers, messages)
	}
}

func validateInterpolations(value string, symbols map[string]clientlang.ValueType, messages *[]string) {
	exprs, err := interpolationExpressions(value)
	if err != nil {
		*messages = append(*messages, err.Error())
		return
	}
	for _, expr := range exprs {
		typ, _, err := clientlang.CheckExpr(expr, symbols)
		if err != nil {
			*messages = append(*messages, fmt.Sprintf("interpolation %q is invalid: %v", expr, err))
			continue
		}
		if typ == clientlang.TypeArray || typ == clientlang.TypeObject {
			*messages = append(*messages, fmt.Sprintf("interpolation %q must be scalar, got %s", expr, typ))
		}
	}
}

func interpolationExpressions(value string) ([]string, error) {
	var expressions []string
	for strings.Contains(value, "{") {
		start := strings.Index(value, "{")
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return nil, fmt.Errorf("unterminated interpolation")
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if expr == "" {
			return nil, fmt.Errorf("empty interpolation")
		}
		expressions = append(expressions, expr)
		value = value[end+1:]
	}
	return expressions, nil
}

func loopSymbols(symbols map[string]clientlang.ValueType, loop view.ForDirective) map[string]clientlang.ValueType {
	out := mergeTypeSymbols(nil, symbols)
	itemType := out[loop.Collection+"[]"]
	if itemType == "" {
		itemType = clientlang.TypeObject
	}
	out[loop.Var] = itemType
	if loop.IndexVar != "" {
		out[loop.IndexVar] = clientlang.TypeInt
	}
	prefix := loop.Collection + "[]."
	for name, typ := range symbols {
		if strings.HasPrefix(name, prefix) {
			out[loop.Var+"."+strings.TrimPrefix(name, prefix)] = typ
		}
	}
	return out
}

func elementForDirective(element view.Element) (view.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" {
			return attr, true
		}
	}
	return view.Attr{}, false
}

func elementKeyExpression(element view.Element) (string, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:key" {
			return strings.TrimSpace(attr.Value), true
		}
	}
	return "", false
}

func collectComponentViewReferences(nodes []view.Node, fields map[string]bool, events *[]eventExpr, bools *[]string, attrs *[]reactiveAttrExpr, classToggles *[]classToggleExpr, styleBindings *[]styleBindingExpr, valueBinds *[]valueBindExpr, checkedBinds *[]string, refBinds *[]string) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Text:
			collectSimpleInterpolations(typed.Value, fields)
		case view.Element:
			if loop, ok := elementForDirective(typed); ok {
				if parsed, err := view.ParseForDirective(loop.Value); err == nil {
					for _, field := range expressionFields(parsed.Collection) {
						fields[field] = true
					}
				}
				continue
			}
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:on:") {
					*events = append(*events, eventExpr{Name: attr.Name, Expression: strings.TrimSpace(attr.Value)})
					for _, field := range view.IslandExpressionFields(attr.Value) {
						fields[field] = true
					}
					continue
				}
				if attr.Name == "g:if" {
					expr := strings.TrimSpace(attr.Value)
					*bools = append(*bools, expr)
					for _, field := range expressionFields(expr) {
						fields[field] = true
					}
					continue
				}
				if attr.Name == "g:else-if" {
					expr := strings.TrimSpace(attr.Value)
					*bools = append(*bools, expr)
					for _, field := range expressionFields(expr) {
						fields[field] = true
					}
					continue
				}
				if attr.Name == "g:bind:value" {
					field := strings.TrimSpace(attr.Value)
					*valueBinds = append(*valueBinds, valueBindExpr{
						Field:      field,
						Element:    typed.Name,
						InputType:  staticAttrValue(typed.Attrs, "type"),
						InputValue: staticAttrValue(typed.Attrs, "value"),
					})
					fields[field] = true
					continue
				}
				if attr.Name == "g:bind:checked" {
					field := strings.TrimSpace(attr.Value)
					*checkedBinds = append(*checkedBinds, field)
					fields[field] = true
					continue
				}
				if attr.Name == "g:ref" {
					*refBinds = append(*refBinds, strings.TrimSpace(attr.Value))
					continue
				}
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				if strings.HasPrefix(attr.Name, "class:") {
					expr := expressionAttrSource(attr.Value)
					*classToggles = append(*classToggles, classToggleExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						fields[field] = true
					}
					continue
				}
				if strings.HasPrefix(attr.Name, "style:") {
					expr := expressionAttrSource(attr.Value)
					*styleBindings = append(*styleBindings, styleBindingExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						fields[field] = true
					}
					continue
				}
				if attr.Expression {
					expr := expressionAttrSource(attr.Value)
					*attrs = append(*attrs, reactiveAttrExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						fields[field] = true
					}
					continue
				}
				collectSimpleInterpolations(attr.Value, fields)
			}
			collectComponentViewReferences(typed.Children, fields, events, bools, attrs, classToggles, styleBindings, valueBinds, checkedBinds, refBinds)
		case view.ComponentCall:
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				collectSimpleInterpolations(attr.Value, fields)
			}
			collectComponentViewReferences(typed.Children, fields, events, bools, attrs, classToggles, styleBindings, valueBinds, checkedBinds, refBinds)
		}
	}
}

func expressionFields(expr string) []string {
	fields, err := clientlang.ExprFields(expr)
	if err != nil {
		return nil
	}
	return fields
}

func expressionAttrSource(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func isValueBindableElement(name string) bool {
	switch name {
	case "input", "textarea", "select":
		return true
	default:
		return false
	}
}

func staticAttrValue(attrs []view.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name == name && !attr.Boolean && !attr.Expression {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func collectSimpleInterpolations(value string, fields map[string]bool) {
	for _, match := range componentSimpleInterpolationPattern.FindAllStringSubmatch(value, -1) {
		fields[match[1]] = true
	}
}

func componentFingerprint(component manifest.Component) string {
	parts := []string{
		"props=" + componentPropsFingerprint(component),
		"state=" + componentStateFingerprint(component),
		"client=" + componentClientFingerprint(component),
		"view=" + componentViewFingerprint(component),
	}
	return strings.Join(parts, "\n")
}

func componentPropsFingerprint(component manifest.Component) string {
	if component.PropsType.Name != "" {
		return "type:" + canonicalGoType(component.Imports, component.PropsType)
	}
	if len(component.Props) == 0 {
		return "legacy:"
	}
	props := make([]string, 0, len(component.Props))
	for _, prop := range component.Props {
		props = append(props, prop.Name+":"+prop.Type)
	}
	sort.Strings(props)
	return "legacy:" + strings.Join(props, ",")
}

func componentStateFingerprint(component manifest.Component) string {
	if component.State.Type.Name == "" {
		return ""
	}
	return canonicalGoType(component.Imports, component.State.Type) + "=init:" + canonicalGoFunc(component.Imports, component.State.Init)
}

func componentViewFingerprint(component manifest.Component) string {
	canonical, err := view.Canonical(component.Blocks.ViewBody)
	if err == nil {
		return canonical
	}
	return strings.Join(strings.Fields(component.Blocks.ViewBody), " ")
}

func componentClientFingerprint(component manifest.Component) string {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return ""
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err == nil {
		return program.Canonical()
	}
	return strings.Join(strings.Fields(component.Blocks.ClientBody), " ")
}

func canonicalGoType(imports []manifest.Import, ref manifest.GoTypeRef) string {
	path, err := gotypes.ImportPathForAlias(imports, ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}

func canonicalGoFunc(imports []manifest.Import, ref manifest.GoFuncRef) string {
	path, err := gotypes.ImportPathForAlias(imports, ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}

func duplicateIdentityMessage(kind, value, firstSource, duplicateSource string) string {
	message := fmt.Sprintf("duplicate %s %q", kind, value)
	if firstSource != "" && duplicateSource != "" {
		return fmt.Sprintf("%s; first declared in %s and duplicated in %s", message, firstSource, duplicateSource)
	}
	return message
}

// ValidatePage checks one page for compile-first render mode rules.
func ValidatePage(config gowdk.Config, page manifest.Page) []ValidationError {
	mode := page.RenderMode(config.Render.DefaultMode())
	var diagnostics []ValidationError
	pageRoute, pageRouteIssues := parseRoute(page.Route)
	diagnostics = append(diagnostics, routeDiagnostics(page, "page route", pageRouteIssues, page.Spans.Route, page.Spans.RouteParams)...)
	for _, api := range page.Blocks.APIs {
		if api.Route == "" {
			continue
		}
		label := "api route"
		if api.Name != "" {
			label = fmt.Sprintf("api %s route", api.Name)
		}
		_, issues := parseRoute(api.Route)
		diagnostics = append(diagnostics, routeDiagnostics(page, label, issues, api.RouteSpan, api.RouteParams)...)
	}

	if mode.RequiresSSR() && !config.HasFeature(gowdk.FeatureSSR) {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_ssr_addon",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s.page.gwdk uses @render %s, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
				page.ID,
				mode,
			),
		})
	}
	if mode == gowdk.Hybrid && !page.Blocks.Load {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "hybrid_requires_explicit_request_policy",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s uses @render hybrid, but no accepted request-time full-page policy is declared. Current hybrid pages must declare load {} so they do not become implicit SSR",
				page.ID,
			),
		})
	}

	if !page.Blocks.View {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_view_block",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Page, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s declares a page route but is missing view {}. Current pages must render HTML for their GET route",
				page.ID,
			),
		})
	}

	var params []string
	if len(pageRouteIssues) == 0 {
		params = pageRoute.Params
	}
	if mode.IsBuildTime() && len(params) > 0 && !page.Paths {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "static_dynamic_route_missing_paths",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstNamedSpan(page.Spans.RouteParams, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s has dynamic route params: {%s}, but render mode is %s and no paths block exists. Fix: add paths { ... } or use @render ssr",
				page.ID,
				strings.Join(params, ", "),
				mode,
			),
		})
	}

	if page.Blocks.Load && mode != gowdk.SSR && mode != gowdk.Hybrid {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "load_requires_request_render",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Blocks.Spans.Load, page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s declares load {}, but load runs at request time and requires @render ssr or @render hybrid",
				page.ID,
			),
		})
	}
	diagnostics = append(diagnostics, validatePageCSS(page)...)

	return diagnostics
}

func validatePageCSS(page manifest.Page) []ValidationError {
	if len(page.CSS) == 0 {
		return nil
	}
	if len(page.CSS) > 1 && containsCSSReference(page.CSS, "none") {
		return []ValidationError{{
			Code:   "invalid_css_selection",
			PageID: page.ID,
			Source: page.Source,
			Span:   spanForName(page.Spans.CSS, "none", page.Spans.Page),
			Message: fmt.Sprintf(
				"%s uses @css none with other CSS inputs. Fix: use @css none by itself or remove none",
				page.ID,
			),
		}}
	}

	seen := map[string]bool{}
	var diagnostics []ValidationError
	for _, name := range page.CSS {
		if !cssReferencePattern.MatchString(name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "invalid_css_selection",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s uses invalid @css input %q. CSS inputs must be identifiers such as default, page, forms, or blog.post",
					page.ID,
					name,
				),
			})
			continue
		}
		if seen[name] {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "duplicate_css_selection",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf("%s repeats @css input %q", page.ID, name),
			})
			continue
		}
		seen[name] = true
	}
	return diagnostics
}

func containsCSSReference(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstSpan(spans ...manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if hasSpan(span) {
			return span
		}
	}
	return manifest.SourceSpan{}
}

func firstNamedSpan(spans []manifest.NamedSpan, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForName(spans []manifest.NamedSpan, name string, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if item.Name == name && hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func hasSpan(span manifest.SourceSpan) bool {
	return span.Start.Line > 0 && span.Start.Column > 0 && span.End.Line > 0 && span.End.Column > 0
}

// ValidationErrors is a set of compiler diagnostics.
type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}
