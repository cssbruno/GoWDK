package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
)

func validateComponentViewContract(component gwdkir.Component, ctx componentValidationContext) []ValidationError {
	var viewRefs componentViewRefs
	if len(component.Blocks.ViewNodes) > 0 {
		viewRefs = componentViewReferencesFromNodes(component.Blocks.ViewBody, component.Blocks.ViewNodes)
	} else {
		var err error
		viewRefs, err = componentViewReferences(component.Blocks.ViewBody)
		if err != nil {
			return []ValidationError{{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.View, component.Span),
				Message:       fmt.Sprintf("component %s view is invalid: %v", component.Name, err),
			}}
		}
	}

	helperFuncs := helperExprFunctions(ctx.Helpers)
	emits := componentEmitMap(component)
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validateComponentListDirectives(component, ctx.SymbolTypes, ctx.StateTypes, ctx.Handlers, helperFuncs)...)
	diagnostics = append(diagnostics, validateUnknownViewFields(component, ctx, viewRefs)...)
	diagnostics = append(diagnostics, validateViewEventExpressions(component, ctx, viewRefs, helperFuncs, emits)...)
	diagnostics = append(diagnostics, validateViewBooleanExpressions(component, ctx, viewRefs)...)
	diagnostics = append(diagnostics, validateViewAttributeExpressions(component, ctx, viewRefs)...)
	diagnostics = append(diagnostics, validateViewValueBinds(component, ctx, viewRefs)...)
	diagnostics = append(diagnostics, validateViewCheckedBinds(component, ctx, viewRefs)...)
	diagnostics = append(diagnostics, validateViewRefBinds(component, ctx, viewRefs)...)
	return diagnostics
}

func validateUnknownViewFields(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	var diagnostics []ValidationError
	seen := map[string]bool{}
	for _, field := range viewRefs.FieldRefs {
		if seen[field.Name] {
			continue
		}
		seen[field.Name] = true
		if ctx.Props[field.Name] || ctx.State[field.Name] {
			continue
		}
		if _, ok := ctx.ComputedTypes[field.Name]; ok {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_field_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          componentViewBodyOffsetSpan(component, field.Start, field.End),
			Message:       fmt.Sprintf("component %s view references unknown field %q", component.Name, field.Name),
		})
	}
	return diagnostics
}

func validateViewEventExpressions(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs, helperFuncs map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) []ValidationError {
	var diagnostics []ValidationError
	for _, eventExpr := range viewRefs.Events {
		if _, err := view.ParseEventDirective(eventExpr.Name); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, eventExpr.Start, eventExpr.End),
				Message:       fmt.Sprintf("component %s event directive %q is invalid: %v", component.Name, eventExpr.Name, err),
			})
			continue
		}
		readSymbols := mergeClientSymbols(ctx.SymbolTypes, view.DOMEventSymbols())
		if err := view.ValidateIslandEventExpressionTypedWithEvents(eventExpr.Expression, readSymbols, ctx.StateTypes, ctx.Handlers, helperFuncs, emits); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, eventExpr.Start, eventExpr.End),
				Message:       fmt.Sprintf("component %s event expression %q is invalid: %v", component.Name, eventExpr.Expression, err),
			})
		}
	}
	return diagnostics
}

func mergeClientSymbols(left, right map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	out := map[string]clientlang.ValueType{}
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func validateViewBooleanExpressions(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	var diagnostics []ValidationError
	for _, expr := range viewRefs.Bools {
		if err := view.ValidateIslandBoolExpressionTyped(expr.Value, ctx.SymbolTypes); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, expr.Start, expr.End),
				Message:       fmt.Sprintf("component %s bool expression %q is invalid: %v", component.Name, expr.Value, err),
			})
		}
	}
	return diagnostics
}

func validateViewAttributeExpressions(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	var diagnostics []ValidationError
	for _, attrExpr := range viewRefs.Attrs {
		if err := view.ValidateReactiveAttrExpressionTyped(attrExpr.Name, attrExpr.Expression, ctx.SymbolTypes); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, attrExpr.Start, attrExpr.End),
				Message:       fmt.Sprintf("component %s reactive attribute %s=%q is invalid: %v", component.Name, attrExpr.Name, attrExpr.Expression, err),
			})
		}
	}
	for _, toggle := range viewRefs.ClassToggles {
		if err := view.ValidateClassToggleExpressionTyped(toggle.Name, toggle.Expression, ctx.SymbolTypes); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, toggle.Start, toggle.End),
				Message:       fmt.Sprintf("component %s class toggle %s=%q is invalid: %v", component.Name, toggle.Name, toggle.Expression, err),
			})
		}
	}
	for _, binding := range viewRefs.StyleBindings {
		if err := view.ValidateStyleBindingExpressionTyped(binding.Name, binding.Expression, ctx.SymbolTypes); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_field_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          componentViewBodyOffsetSpan(component, binding.Start, binding.End),
				Message:       fmt.Sprintf("component %s style binding %s=%q is invalid: %v", component.Name, binding.Name, binding.Expression, err),
			})
		}
	}
	return diagnostics
}

func validateViewValueBinds(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	var diagnostics []ValidationError
	for _, field := range viewRefs.ValueBinds {
		diagnostics = append(diagnostics, validateViewValueBind(component, ctx, field)...)
	}
	return diagnostics
}

func validateViewValueBind(component gwdkir.Component, ctx componentValidationContext, field valueBindExpr) []ValidationError {
	typ, ok := ctx.StateTypes[field.Field]
	if !ok {
		return []ValidationError{componentFieldErrorAt(component, componentViewBodyOffsetSpan(component, field.Start, field.End), fmt.Sprintf("component %s g:bind:value target %q must be a state field", component.Name, field.Field))}
	}
	if !isValueBindableElement(field.Element) {
		return []ValidationError{componentFieldErrorAt(component, componentViewBodyOffsetSpan(component, field.Start, field.End), fmt.Sprintf("component %s g:bind:value target %q is on unsupported <%s>", component.Name, field.Field, field.Element))}
	}
	if field.Element == "input" && strings.EqualFold(field.InputType, "radio") && field.InputValue == "" {
		return []ValidationError{componentFieldErrorAt(component, componentViewBodyOffsetSpan(component, field.Start, field.End), fmt.Sprintf("component %s g:bind:value radio target %q requires a literal value attribute", component.Name, field.Field))}
	}
	if typ == clientlang.TypeString || typ == clientlang.TypeUnknown {
		return nil
	}
	if typ == clientlang.TypeInt || typ == clientlang.TypeFloat {
		if field.Element == "input" && strings.EqualFold(field.InputType, "number") {
			return nil
		}
		return []ValidationError{componentFieldErrorAt(component, componentViewBodyOffsetSpan(component, field.Start, field.End), fmt.Sprintf("component %s g:bind:value numeric target %q requires <input type=\"number\">", component.Name, field.Field))}
	}
	return []ValidationError{componentFieldErrorAt(component, componentViewBodyOffsetSpan(component, field.Start, field.End), fmt.Sprintf("component %s g:bind:value target %q must be string or numeric, got %s", component.Name, field.Field, typ))}
}

func validateViewCheckedBinds(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	var diagnostics []ValidationError
	for _, field := range viewRefs.CheckedBinds {
		span := componentViewBodyOffsetSpan(component, field.Start, field.End)
		typ, ok := ctx.StateTypes[field.Name]
		if !ok {
			diagnostics = append(diagnostics, componentFieldErrorAt(component, span, fmt.Sprintf("component %s g:bind:checked target %q must be a state field", component.Name, field.Name)))
			continue
		}
		if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
			diagnostics = append(diagnostics, componentFieldErrorAt(component, span, fmt.Sprintf("component %s g:bind:checked target %q must be bool, got %s", component.Name, field.Name, typ)))
		}
	}
	return diagnostics
}

func validateViewRefBinds(component gwdkir.Component, ctx componentValidationContext, viewRefs componentViewRefs) []ValidationError {
	boundRefs := map[string]bool{}
	var diagnostics []ValidationError
	for _, refName := range viewRefs.RefBinds {
		span := componentViewBodyOffsetSpan(component, refName.Start, refName.End)
		if ctx.Refs == nil {
			diagnostics = append(diagnostics, componentFieldErrorAt(component, span, fmt.Sprintf("component %s g:ref target %q is not declared", component.Name, refName.Name)))
			continue
		}
		if _, ok := ctx.Refs[refName.Name]; !ok {
			diagnostics = append(diagnostics, componentFieldErrorAt(component, span, fmt.Sprintf("component %s g:ref target %q is not declared", component.Name, refName.Name)))
			continue
		}
		if boundRefs[refName.Name] {
			diagnostics = append(diagnostics, componentFieldErrorAt(component, span, fmt.Sprintf("component %s g:ref target %q is bound more than once", component.Name, refName.Name)))
			continue
		}
		boundRefs[refName.Name] = true
	}
	for refName := range ctx.UsedRefs {
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
	return diagnostics
}

func componentFieldError(component gwdkir.Component, message string) ValidationError {
	return componentFieldErrorAt(component, firstSpan(component.Blocks.Spans.View, component.Span), message)
}

func componentFieldErrorAt(component gwdkir.Component, span source.SourceSpan, message string) ValidationError {
	return ValidationError{
		Code:          "component_field_error",
		ComponentName: component.Name,
		Source:        component.Source,
		Span:          span,
		Message:       message,
	}
}
