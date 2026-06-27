package viewrender

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/viewparse"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

func renderTextNode(node Text, ctx *renderContext, out *renderOutput) error {
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

type ForDirective = viewparse.ForDirective

// ParseForDirective parses a g:for value such as "item in Items" or
// "item, i in Items".
func ParseForDirective(source string) (ForDirective, error) {
	return viewparse.ParseForDirective(source)
}

func renderForElement(node Element, ctx *renderContext, out *renderOutput, loop ForDirective, keyExpr string) error {
	group := ctx.nextLoopGroup()
	templateNode := elementWithoutAttrs(node, "g:for", "g:key")
	templateCtx := *ctx
	templateCtx.templateLoop = &templateLoopRender{}
	templateCtx.loopItem = &loopItemRender{Group: group, KeyExpr: keyExpr}
	templateCtx.readFields = boolSet(keysFromTypes(ctx.loopSymbols(loop)))
	var template renderOutput
	if err := renderElement(templateNode, &templateCtx, &template); err != nil {
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
	out.write(`">`)
	out.write(template.string())
	out.write(`</template>`)
	if ctx.templateLoop != nil {
		return nil
	}
	items, err := loopItems(loop.Collection, ctx.values)
	if err != nil {
		return fmt.Errorf("g:for: %w", err)
	}
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
		if err := renderElement(templateNode, &itemCtx, out); err != nil {
			return err
		}
	}
	return nil
}

func elementForDirective(node Element, ctx *renderContext) (ForDirective, string, bool, error) {
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

func elementWithoutAttrs(node Element, names ...string) Element {
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
		if symbols[loop.Collection] == clientlang.TypeUnknown {
			itemType = clientlang.TypeUnknown
		}
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
	return ctx.idAllocator().nextLoopGroup()
}

func (ctx *renderContext) nextBindingID() string {
	return ctx.idAllocator().nextBindingID()
}

func (ctx *renderContext) nextIslandID() string {
	return ctx.idAllocator().nextIslandID()
}

func (ctx *renderContext) nextAwaitID() string {
	return ctx.idAllocator().nextAwaitID()
}

func (ctx *renderContext) idAllocator() *renderIDAllocator {
	if ctx.ids == nil {
		ctx.ids = &renderIDAllocator{}
	}
	return ctx.ids
}

func (ids *renderIDAllocator) nextLoopGroup() string {
	ids.loop++
	return fmt.Sprintf("l%d", ids.loop)
}

func (ids *renderIDAllocator) nextBindingID() string {
	ids.binding++
	return fmt.Sprintf("b%d", ids.binding)
}

func (ids *renderIDAllocator) nextIslandID() string {
	ids.island++
	return fmt.Sprintf("i%d", ids.island)
}

func (ids *renderIDAllocator) nextAwaitID() string {
	ids.await++
	return fmt.Sprintf("a%d", ids.await)
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
