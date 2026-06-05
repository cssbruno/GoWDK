package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func validateComponentListDirectives(component manifest.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) []ValidationError {
	nodes, err := view.Parse(component.Blocks.ViewBody)
	if err != nil {
		return nil
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
	Span    manifest.SourceSpan
}

func validateListNodes(nodes []view.Node, component manifest.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
				validateListElement(typed, loopAttr, component, symbols, stateTypes, handlers, helpers, messages)
				continue
			}
			validateListNodes(typed.Children, component, symbols, stateTypes, handlers, helpers, messages)
		case view.ComponentCall:
			validateListNodes(typed.Children, component, symbols, stateTypes, handlers, helpers, messages)
		}
	}
}

func validateListElement(element view.Element, loopAttr view.Attr, component manifest.Component, symbols map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	loopSpan := viewBodyNeedleSpan(component, loopAttr.Value)
	loop, err := view.ParseForDirective(loopAttr.Value)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: err.Error(), Span: loopSpan})
		return
	}
	collectionType, _, err := clientlang.CheckExpr(loop.Collection, symbols)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:for collection %q is invalid: %v", loop.Collection, err), Span: viewBodyNeedleSpan(component, loop.Collection)})
		return
	}
	if collectionType != clientlang.TypeArray && collectionType != clientlang.TypeUnknown {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:for collection %q must be array, got %s", loop.Collection, collectionType), Span: viewBodyNeedleSpan(component, loop.Collection)})
		return
	}
	keyExpr, ok := elementKeyExpression(element)
	if !ok {
		*messages = append(*messages, spannedMessage{Message: "g:for requires g:key for mutable lists", Span: loopSpan})
		return
	}
	loopSymbols := loopSymbols(symbols, loop)
	keyType, _, err := clientlang.CheckExpr(keyExpr, loopSymbols)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:key %q is invalid: %v", keyExpr, err), Span: viewBodyNeedleSpan(component, keyExpr)})
		return
	}
	if keyType == clientlang.TypeArray || keyType == clientlang.TypeObject || keyType == clientlang.TypeNil {
		*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:key %q must be scalar, got %s", keyExpr, keyType), Span: viewBodyNeedleSpan(component, keyExpr)})
		return
	}
	validateLoopElementBody(element, loopSymbols, stateTypes, handlers, helpers, messages)
}

func validateLoopSubtree(node view.Node, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	switch typed := node.(type) {
	case view.Text:
		validateInterpolations(typed.Value, readSymbols, messages)
	case view.Element:
		if loopAttr, hasLoop := elementForDirective(typed); hasLoop {
			validateListElement(typed, loopAttr, manifest.Component{}, readSymbols, writeSymbols, handlers, helpers, messages)
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

func validateLoopElementBody(element view.Element, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, messages *[]spannedMessage) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" || attr.Name == "g:key" {
			continue
		}
		switch {
		case strings.HasPrefix(attr.Name, "g:on:"):
			if err := view.ValidateIslandEventExpressionTypedWithFunctions(attr.Value, readSymbols, writeSymbols, handlers, helpers); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err)})
			}
		case attr.Name == "g:if" || attr.Name == "g:else-if":
			if err := view.ValidateIslandBoolExpressionTyped(strings.TrimSpace(attr.Value), readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, attr.Value, err)})
			}
		case attr.Name == "g:bind:value":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:bind:value target %q must be a state field", strings.TrimSpace(attr.Value))})
			}
		case attr.Name == "g:bind:checked":
			if _, ok := writeSymbols[strings.TrimSpace(attr.Value)]; !ok {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("g:bind:checked target %q must be a state field", strings.TrimSpace(attr.Value))})
			}
		case strings.HasPrefix(attr.Name, "class:"):
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateClassToggleExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err)})
			}
		case strings.HasPrefix(attr.Name, "style:"):
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateStyleBindingExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err)})
			}
		case attr.Expression:
			expr := expressionAttrSource(attr.Value)
			if err := view.ValidateReactiveAttrExpressionTyped(attr.Name, expr, readSymbols); err != nil {
				*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("%s=%q is invalid: %v", attr.Name, expr, err)})
			}
		default:
			validateInterpolations(attr.Value, readSymbols, messages)
		}
	}
	for _, child := range element.Children {
		validateLoopSubtree(child, readSymbols, writeSymbols, handlers, helpers, messages)
	}
}
