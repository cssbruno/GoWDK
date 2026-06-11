package view

import (
	"encoding/json"
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
	"strconv"
	"strings"
)

type Node interface {
	render(*renderContext, *renderOutput) error
}

// Text is escaped text content.
type Text struct {
	Value string
	Start int
	End   int
}

func (node Text) render(ctx *renderContext, out *renderOutput) error {
	if ctx.templateLoop == nil {
		if field, ok := islandTextBinding(node.Value); ok && ctx.bindFields[field] {
			value, _, err := interpolateValue(ctx, node.Value)
			if err != nil {
				return err
			}
			out.write(`<span data-gowdk-bind="`)
			out.write(gowhtml.Escape(field))
			out.write(`" data-gowdk-binding-text="`)
			out.write(ctx.nextBindingID())
			out.write(`">`)
			out.write(gowhtml.Escape(value))
			out.write(`</span>`)
			return nil
		}
	}
	return renderText(ctx, out, node.Value)
}

// ForDirective is a parsed g:for declaration.
type ForDirective struct {
	Var        string
	IndexVar   string
	Collection string
}

// ParseForDirective parses a g:for value such as "item in Items" or
// "item, i in Items".
func ParseForDirective(source string) (ForDirective, error) {
	match := forDirectivePattern.FindStringSubmatch(strings.TrimSpace(source))
	if match == nil {
		return ForDirective{}, fmt.Errorf("g:for must use \"item in Items\" or \"item, i in Items\" syntax")
	}
	item := strings.TrimSpace(match[1])
	if !isIdentifier(item) {
		return ForDirective{}, fmt.Errorf("g:for item name %q is invalid", item)
	}
	index := strings.TrimSpace(match[2])
	if index != "" {
		if !isIdentifier(index) {
			return ForDirective{}, fmt.Errorf("g:for index name %q is invalid", index)
		}
		if index == item {
			return ForDirective{}, fmt.Errorf("g:for item and index names must differ")
		}
	}
	collection := strings.TrimSpace(match[3])
	if collection == "" {
		return ForDirective{}, fmt.Errorf("g:for collection expression is empty")
	}
	return ForDirective{Var: item, IndexVar: index, Collection: collection}, nil
}

func (node Element) renderFor(ctx *renderContext, out *renderOutput, loop ForDirective, keyExpr string) error {
	items, err := loopItems(loop.Collection, ctx.values)
	if err != nil {
		return fmt.Errorf("g:for: %w", err)
	}
	group := ctx.nextLoopGroup()
	templateNode := node.withoutAttrs("g:for", "g:key")
	templateCtx := *ctx
	templateCtx.templateLoop = &templateLoopRender{}
	templateCtx.loopItem = &loopItemRender{Group: group, KeyExpr: keyExpr}
	templateCtx.readFields = boolSet(keysFromTypes(ctx.loopSymbols(loop)))
	var template renderOutput
	if err := templateNode.render(&templateCtx, &template); err != nil {
		return err
	}
	out.write(`<template data-gowdk-for="`)
	out.write(gowhtml.Escape(group))
	out.write(`" data-gowdk-binding-list="`)
	out.write(ctx.nextBindingID())
	out.write(`" data-gowdk-for-var="`)
	out.write(gowhtml.Escape(loop.Var))
	out.write(`" data-gowdk-for-source="`)
	out.write(gowhtml.Escape(loop.Collection))
	out.write(`" data-gowdk-for-key="`)
	out.write(gowhtml.Escape(keyExpr))
	if loop.IndexVar != "" {
		out.write(`" data-gowdk-for-index-var="`)
		out.write(gowhtml.Escape(loop.IndexVar))
	}
	out.write(`" data-gowdk-for-template="`)
	out.write(gowhtml.Escape(template.string()))
	out.write(`"></template>`)
	seenKeys := map[string]bool{}
	for index, item := range items {
		itemCtx, err := ctx.loopContext(loop, keyExpr, group, item, index)
		if err != nil {
			return err
		}
		key := itemCtx.loopItem.KeyValue
		if key != "" {
			if seenKeys[key] {
				return fmt.Errorf("g:for duplicate key %q", key)
			}
			seenKeys[key] = true
		}
		if err := templateNode.render(&itemCtx, out); err != nil {
			return err
		}
	}
	return nil
}

func (node Element) forDirective(ctx *renderContext) (ForDirective, string, bool, error) {
	var loop ForDirective
	var keyExpr string
	hasFor := false
	hasKey := false
	for _, attr := range node.Attrs {
		switch attr.Name {
		case "g:for":
			if hasFor {
				return ForDirective{}, "", false, fmt.Errorf("element declares multiple g:for directives")
			}
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return ForDirective{}, "", false, fmt.Errorf("g:for requires an expression value")
			}
			parsed, err := ParseForDirective(attr.Value)
			if err != nil {
				return ForDirective{}, "", false, err
			}
			loop = parsed
			hasFor = true
		case "g:key":
			if hasKey {
				return ForDirective{}, "", false, fmt.Errorf("element declares multiple g:key directives")
			}
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return ForDirective{}, "", false, fmt.Errorf("g:key requires an expression value")
			}
			keyExpr = strings.TrimSpace(attr.Value)
			hasKey = true
		}
	}
	if !hasFor {
		if hasKey {
			return ForDirective{}, "", false, fmt.Errorf("g:key requires g:for")
		}
		return ForDirective{}, "", false, nil
	}
	if !hasKey {
		return ForDirective{}, "", false, fmt.Errorf("g:for requires g:key for mutable lists")
	}
	if typ, _, err := clientlang.CheckExpr(loop.Collection, ctx.readSymbols()); err != nil {
		return ForDirective{}, "", false, fmt.Errorf("g:for collection %q is invalid: %w", loop.Collection, err)
	} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
		return ForDirective{}, "", false, fmt.Errorf("g:for collection %q must be array, got %s", loop.Collection, typ)
	}
	loopSymbols := ctx.loopSymbols(loop)
	if typ, _, err := clientlang.CheckExpr(keyExpr, loopSymbols); err != nil {
		return ForDirective{}, "", false, fmt.Errorf("g:key %q is invalid: %w", keyExpr, err)
	} else if typ == clientlang.TypeArray || typ == clientlang.TypeObject || typ == clientlang.TypeNil {
		return ForDirective{}, "", false, fmt.Errorf("g:key %q must be scalar, got %s", keyExpr, typ)
	}
	return loop, keyExpr, true, nil
}

func (node Element) withoutAttrs(names ...string) Element {
	removed := map[string]bool{}
	for _, name := range names {
		removed[name] = true
	}
	next := node
	next.Attrs = make([]Attr, 0, len(node.Attrs))
	for _, attr := range node.Attrs {
		if removed[attr.Name] {
			continue
		}
		next.Attrs = append(next.Attrs, attr)
	}
	return next
}

func loopItems(expr string, values map[string]string) ([]any, error) {
	value, err := clientlang.EvalValue(expr, values)
	if err != nil {
		return nil, err
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("collection %q is not an array", expr)
	}
	return items, nil
}

func flattenLoopValues(prefix string, value any, values map[string]string) {
	values[prefix] = runtimeValueString(value)
	switch typed := value.(type) {
	case map[string]any:
		for field, fieldValue := range typed {
			flattenLoopValues(prefix+"."+field, fieldValue, values)
		}
	case []any:
		for index, item := range typed {
			flattenLoopValues(fmt.Sprintf("%s[%d]", prefix, index), item, values)
		}
	}
}

func runtimeValueString(value any) string {
	if scalar, ok := scalarString(value); ok {
		return scalar
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
}

func loopTemplateValue(expr string) string {
	return "{{" + strings.TrimSpace(expr) + "}}"
}

func (ctx *renderContext) readSymbols() map[string]clientlang.ValueType {
	if len(ctx.stateTypes) == 0 {
		return boolFieldSymbols(ctx.readFields)
	}
	symbols := boolFieldSymbols(ctx.readFields)
	if symbols == nil {
		symbols = map[string]clientlang.ValueType{}
	}
	for field, typ := range ctx.stateTypes {
		symbols[field] = typ
	}
	return symbols
}

func (ctx *renderContext) loopSymbols(loop ForDirective) map[string]clientlang.ValueType {
	symbols := ctx.readSymbols()
	if symbols == nil {
		symbols = map[string]clientlang.ValueType{}
	}
	itemType := symbols[loop.Collection+"[]"]
	if itemType == "" {
		itemType = clientlang.TypeObject
	}
	symbols[loop.Var] = itemType
	if loop.IndexVar != "" {
		symbols[loop.IndexVar] = clientlang.TypeInt
	}
	prefix := loop.Collection + "[]."
	additions := map[string]clientlang.ValueType{}
	for name, typ := range symbols {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		additions[loop.Var+"."+strings.TrimPrefix(name, prefix)] = typ
	}
	for name, typ := range additions {
		symbols[name] = typ
	}
	return symbols
}

func (ctx *renderContext) loopContext(loop ForDirective, keyExpr, group string, item any, index int) (renderContext, error) {
	values := cloneValues(ctx.values)
	flattenLoopValues(loop.Var, item, values)
	values["index"] = strconv.Itoa(index)
	if loop.IndexVar != "" {
		values[loop.IndexVar] = strconv.Itoa(index)
	}
	keyValue, err := clientlang.EvalScalar(keyExpr, values)
	if err != nil {
		return renderContext{}, fmt.Errorf("g:key %q: %w", keyExpr, err)
	}
	next := *ctx
	next.values = values
	next.readFields = boolSet(keys(values))
	next.bindFields = ctx.bindFields
	next.loopItem = &loopItemRender{Group: group, KeyExpr: keyExpr, KeyValue: keyValue}
	return next, nil
}

func (ctx *renderContext) nextLoopGroup() string {
	if ctx.loopSeq == nil {
		seq := 0
		ctx.loopSeq = &seq
	}
	*ctx.loopSeq = *ctx.loopSeq + 1
	return fmt.Sprintf("l%d", *ctx.loopSeq)
}

func (ctx *renderContext) nextBindingID() string {
	if ctx.bindingSeq == nil {
		seq := 0
		ctx.bindingSeq = &seq
	}
	*ctx.bindingSeq = *ctx.bindingSeq + 1
	return fmt.Sprintf("b%d", *ctx.bindingSeq)
}

func (ctx *renderContext) nextIslandID() string {
	if ctx.islandSeq == nil {
		seq := 0
		ctx.islandSeq = &seq
	}
	*ctx.islandSeq = *ctx.islandSeq + 1
	return fmt.Sprintf("i%d", *ctx.islandSeq)
}

func (ctx *renderContext) loopKeyValue(expr string) string {
	if ctx.templateLoop != nil {
		return loopTemplateValue(expr)
	}
	if ctx.loopItem != nil {
		return ctx.loopItem.KeyValue
	}
	return ""
}
