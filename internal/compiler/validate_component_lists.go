package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
	"github.com/cssbruno/gowdk/internal/viewvalidation"
)

func validateComponentListDirectives(component gwdkir.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) []ValidationError {
	nodes := component.Blocks.ViewNodes
	if len(nodes) == 0 {
		var err error
		nodes, err = viewparse.Parse(component.Blocks.ViewBody)
		if err != nil {
			return nil
		}
	}
	var messages []spannedMessage
	validateListNodes(nodes, component, symbols, stateTypes, handlers, helpers, &messages)
	if len(messages) == 0 {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(messages))
	for _, message := range messages {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_field_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(message.Span, component.Blocks.Spans.View, component.Span),
			Message:       fmt.Sprintf("component %s list rendering is invalid: %s", component.Name, message.Message),
		})
	}
	return diagnostics
}

type spannedMessage struct {
	Message string
	Span    source.SourceSpan
}

func validateListNodes(nodes []viewmodel.Node, component gwdkir.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
				validateListElement(typed, loopAttr, component, symbols, stateTypes, handlers, helpers, messages)
				continue
			}
			validateListNodes(typed.Children, component, symbols, stateTypes, handlers, helpers, messages)
		case viewmodel.ComponentCall:
			validateListNodes(typed.Children, component, symbols, stateTypes, handlers, helpers, messages)
		case viewmodel.AwaitBlock:
			if _, err := clientlang.ValidateAwaitFetchExpression(typed.Expression, symbols, helpers); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("await block is invalid: %v", err), Span: componentViewBodyOffsetSpan(component, typed.Start, typed.End)})
				continue
			}
			validateListNodes(typed.Pending, component, symbols, stateTypes, handlers, helpers, messages)
			validateListNodes(typed.Then, component, awaitThenSymbols(symbols, typed.ResultName), stateTypes, handlers, helpers, messages)
			if typed.ErrorName != "" {
				validateListNodes(typed.Catch, component, awaitCatchSymbols(symbols, typed.ErrorName), stateTypes, handlers, helpers, messages)
			}
		}
	}
}

func validateListElement(element viewmodel.Element, loopAttr viewmodel.Attr, component gwdkir.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	loopSpan := attrExprSpan(component, loopAttr, loopAttr.Value)
	loop, err := viewparse.ParseForDirective(loopAttr.Value)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: err.Error(), Span: loopSpan})
		return
	}
	collectionType, _, err := clientlang.CheckExpr(loop.Collection, symbols)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:for collection %q is invalid: %v", loop.Collection, err), Span: attrExprSpan(component, loopAttr, loop.Collection)})
		return
	}
	if collectionType != clientlang.TypeArray && collectionType != clientlang.TypeUnknown {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:for collection %q must be array, got %s", loop.Collection, collectionType), Span: attrExprSpan(component, loopAttr, loop.Collection)})
		return
	}
	keyAttr, hasKey := elementKeyAttr(element)
	if !hasKey {
		*messages = append(*messages, spannedMessage{Message: "g:for requires g:key for mutable lists", Span: loopSpan})
		return
	}
	keyExpr := strings.TrimSpace(keyAttr.Value)
	keySpan := attrExprSpan(component, keyAttr, keyExpr)
	loopSymbols := loopSymbols(symbols, loop)
	keyType, _, err := clientlang.CheckExpr(keyExpr, loopSymbols)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:key %q is invalid: %v", keyExpr, err), Span: keySpan})
		return
	}
	if keyType == clientlang.TypeArray || keyType == clientlang.TypeObject || keyType == clientlang.TypeNil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:key %q must be scalar, got %s", keyExpr, keyType), Span: keySpan})
		return
	}
	validateLoopElementBody(component, element, loopSymbols, stateTypes, handlers, helpers, messages)
}

// attrExprSpan returns the exact source span of value inside attr, mapped from
// the component view-body offsets to file line/column. It points a diagnostic at
// the offending expression rather than the whole attribute or view block.
func attrExprSpan(component gwdkir.Component, attr viewmodel.Attr, value string) source.SourceSpan {
	start, end := attrValueOffset(component.Blocks.ViewBody, attr, value)
	return componentViewBodyOffsetSpan(component, start, end)
}

func elementKeyAttr(element viewmodel.Element) (viewmodel.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:key" {
			return attr, true
		}
	}
	return viewmodel.Attr{}, false
}

func validateLoopSubtree(component gwdkir.Component, node viewmodel.Node, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	switch typed := node.(type) {
	case viewmodel.Text:
		validateInterpolations(typed.Value, readSymbols, messages, componentViewBodyOffsetSpan(component, typed.Start, typed.End))
	case viewmodel.Element:
		if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
			validateListElement(typed, loopAttr, component, readSymbols, writeSymbols, handlers, helpers, messages)
			return
		}
		validateLoopElementBody(component, typed, readSymbols, writeSymbols, handlers, helpers, messages)
	case viewmodel.ComponentCall:
		for _, attr := range typed.Attrs {
			if strings.HasPrefix(attr.Name, "g:") {
				continue
			}
			validateInterpolations(attr.Value, readSymbols, messages, componentViewBodyOffsetSpan(component, attr.Start, attr.End))
		}
		for _, child := range typed.Children {
			validateLoopSubtree(component, child, readSymbols, writeSymbols, handlers, helpers, messages)
		}
	case viewmodel.AwaitBlock:
		if _, err := clientlang.ValidateAwaitFetchExpression(typed.Expression, readSymbols, helpers); err != nil {
			*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("await block is invalid: %v", err), Span: componentViewBodyOffsetSpan(component, typed.Start, typed.End)})
			return
		}
		for _, child := range typed.Pending {
			validateLoopSubtree(component, child, readSymbols, writeSymbols, handlers, helpers, messages)
		}
		for _, child := range typed.Then {
			validateLoopSubtree(component, child, awaitThenSymbols(readSymbols, typed.ResultName), writeSymbols, handlers, helpers, messages)
		}
		if typed.ErrorName != "" {
			for _, child := range typed.Catch {
				validateLoopSubtree(component, child, awaitCatchSymbols(readSymbols, typed.ErrorName), writeSymbols, handlers, helpers, messages)
			}
		}
	}
}

func awaitThenSymbols(symbols map[string]clientlang.ValueType, name string) map[string]clientlang.ValueType {
	return mergeTypeSymbols(symbols, map[string]clientlang.ValueType{name: clientlang.TypeUnknown})
}

func awaitCatchSymbols(symbols map[string]clientlang.ValueType, name string) map[string]clientlang.ValueType {
	return mergeTypeSymbols(symbols, map[string]clientlang.ValueType{
		name:              clientlang.TypeObject,
		name + ".message": clientlang.TypeString,
	})
}

func validateLoopElementBody(component gwdkir.Component, element viewmodel.Element, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" || attr.Name == "g:key" {
			continue
		}
		switch {
		case strings.HasPrefix(attr.Name, "g:on:"):
			if err := clientlang.ValidateIslandEventExpressionTypedWithFunctions(attr.Value, readSymbols, writeSymbols, handlers, helpers); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err), Span: attrExprSpan(component, attr, strings.TrimSpace(attr.Value))})
			}
		case attr.Name == "g:if" || attr.Name == "g:else-if":
			if err := clientlang.ValidateIslandBoolExpressionTyped(strings.TrimSpace(attr.Value), readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err), Span: attrExprSpan(component, attr, strings.TrimSpace(attr.Value))})
			}
		case attr.Name == "g:bind:value":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:bind:value target %q must be a state field", strings.TrimSpace(attr.Value)), Span: attrExprSpan(component, attr, strings.TrimSpace(attr.Value))})
			}
		case attr.Name == "g:bind:checked":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:bind:checked target %q must be a state field", strings.TrimSpace(attr.Value)), Span: attrExprSpan(component, attr, strings.TrimSpace(attr.Value))})
			}
		case strings.HasPrefix(attr.Name, "class:"):
			expr := expressionAttrSource(attr.Value)
			if err := viewvalidation.ValidateClassToggleExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err), Span: attrExprSpan(component, attr, expr)})
			}
		case strings.HasPrefix(attr.Name, "style:"):
			expr := expressionAttrSource(attr.Value)
			if err := viewvalidation.ValidateStyleBindingExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err), Span: attrExprSpan(component, attr, expr)})
			}
		case attr.Expression:
			expr := expressionAttrSource(attr.Value)
			if err := viewvalidation.ValidateReactiveAttrExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err), Span: attrExprSpan(component, attr, expr)})
			}
		default:
			validateInterpolations(attr.Value, readSymbols, messages, componentViewBodyOffsetSpan(component, attr.Start, attr.End))
		}
	}
	for _, child := range element.Children {
		validateLoopSubtree(component, child, readSymbols, writeSymbols, handlers, helpers, messages)
	}
}
