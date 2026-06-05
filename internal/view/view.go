// Package view parses and renders the first static subset of view {} markup.
package view

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

var (
	islandFieldPattern         = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	islandIncDecPattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$`)
	islandAssignPattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	islandTogglePattern        = regexp.MustCompile(`^!\s*([A-Za-z_][A-Za-z0-9_]*)$`)
	islandNumberPattern        = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
	islandTextBindingPattern   = regexp.MustCompile(`^\s*\{([A-Za-z_][A-Za-z0-9_]*)\}\s*$`)
	islandRefCallPattern       = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$`)
	islandLetPattern           = regexp.MustCompile(`^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	islandAwaitFetchPattern    = regexp.MustCompile(`^await\s+fetchJSON\[(.+)\]\((.*)\)$`)
	forDirectivePattern        = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(?:\s*,\s*([A-Za-z_][A-Za-z0-9_]*))?\s+in\s+(.+)$`)
	eventNamePattern           = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	stylePropertyPattern       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	styleCustomPropertyPattern = regexp.MustCompile(`^--[A-Za-z0-9_-]+$`)
)

// Node is a static view markup node.
type Node interface {
	render(*renderContext, *strings.Builder) error
}

// Text is escaped text content.
type Text struct {
	Value string
}

func (node Text) render(ctx *renderContext, out *strings.Builder) error {
	if ctx.templateLoop == nil {
		if field, ok := islandTextBinding(node.Value); ok && ctx.bindFields[field] {
			value, _, err := interpolateValue(ctx, node.Value)
			if err != nil {
				return err
			}
			out.WriteString(`<span data-gowdk-bind="`)
			out.WriteString(gowhtml.Escape(field))
			out.WriteString(`" data-gowdk-binding-text="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteString(`">`)
			out.WriteString(gowhtml.Escape(value))
			out.WriteString(`</span>`)
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

func (node Element) renderFor(ctx *renderContext, out *strings.Builder, loop ForDirective, keyExpr string) error {
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
	var template strings.Builder
	if err := templateNode.render(&templateCtx, &template); err != nil {
		return err
	}
	out.WriteString(`<template data-gowdk-for="`)
	out.WriteString(gowhtml.Escape(group))
	out.WriteString(`" data-gowdk-binding-list="`)
	out.WriteString(ctx.nextBindingID())
	out.WriteString(`" data-gowdk-for-var="`)
	out.WriteString(gowhtml.Escape(loop.Var))
	out.WriteString(`" data-gowdk-for-source="`)
	out.WriteString(gowhtml.Escape(loop.Collection))
	out.WriteString(`" data-gowdk-for-key="`)
	out.WriteString(gowhtml.Escape(keyExpr))
	if loop.IndexVar != "" {
		out.WriteString(`" data-gowdk-for-index-var="`)
		out.WriteString(gowhtml.Escape(loop.IndexVar))
	}
	out.WriteString(`" data-gowdk-for-template="`)
	out.WriteString(gowhtml.Escape(template.String()))
	out.WriteString(`"></template>`)
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

// Element is a lowercase HTML element.
type Element struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

func (node Element) render(ctx *renderContext, out *strings.Builder) error {
	if node.Name == "slot" {
		if ctx.slotHTML != "" {
			out.WriteString(ctx.slotHTML)
			return nil
		}
		for _, child := range node.Children {
			if err := child.render(ctx, out); err != nil {
				return err
			}
		}
		return nil
	}
	if loop, keyExpr, ok, err := node.forDirective(ctx); err != nil {
		return err
	} else if ok {
		return node.renderFor(ctx, out, loop, keyExpr)
	}
	if ctx.conditional != nil {
		out.WriteString(`<!--gowdk-if:`)
		out.WriteString(gowhtml.Escape(ctx.conditional.Marker()))
		out.WriteString(`:start-->`)
	}
	out.WriteByte('<')
	out.WriteString(node.Name)
	directives, err := node.postDirectives(ctx)
	if err != nil {
		return err
	}
	valueBinding, err := node.valueBinding(ctx)
	if err != nil {
		return err
	}
	checkedBinding, err := node.checkedBinding(ctx)
	if err != nil {
		return err
	}
	styleBindings, err := node.styleBindings(ctx)
	if err != nil {
		return err
	}
	classToggles, err := node.classToggles(ctx)
	if err != nil {
		return err
	}
	for _, binding := range styleBindings {
		out.WriteString(` data-gowdk-style-`)
		out.WriteString(binding.Property)
		out.WriteString(`="`)
		out.WriteString(gowhtml.Escape(binding.Expression))
		out.WriteString(`" data-gowdk-binding-style-`)
		out.WriteString(binding.Property)
		out.WriteString(`="`)
		out.WriteString(ctx.nextBindingID())
		out.WriteByte('"')
		if binding.Unit != "" {
			out.WriteString(` data-gowdk-style-unit-`)
			out.WriteString(binding.Property)
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(binding.Unit))
			out.WriteByte('"')
		}
	}
	for _, toggle := range classToggles {
		out.WriteString(` data-gowdk-class-`)
		out.WriteString(toggle.Name)
		out.WriteString(`="`)
		out.WriteString(gowhtml.Escape(toggle.Expression))
		out.WriteString(`" data-gowdk-binding-class-`)
		out.WriteString(toggle.Name)
		out.WriteString(`="`)
		out.WriteString(ctx.nextBindingID())
		out.WriteByte('"')
	}
	if classValue := node.initialClassValue(ctx, classToggles); classValue != "" {
		out.WriteString(` class="`)
		out.WriteString(gowhtml.Escape(classValue))
		out.WriteByte('"')
	}
	styleValue, err := node.initialStyleValue(ctx, styleBindings)
	if err != nil {
		return err
	}
	if styleValue != "" {
		out.WriteString(` style="`)
		out.WriteString(gowhtml.Escape(styleValue))
		out.WriteByte('"')
	}
	if ctx.loopItem != nil {
		out.WriteString(` data-gowdk-for-item="`)
		out.WriteString(gowhtml.Escape(ctx.loopItem.Group))
		out.WriteString(`" data-gowdk-key-value="`)
		out.WriteString(gowhtml.Escape(ctx.loopKeyValue(ctx.loopItem.KeyExpr)))
		out.WriteByte('"')
	}
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:on:") {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return fmt.Errorf("%s requires an expression value", attr.Name)
			}
			eventDirective, err := ParseEventDirective(attr.Name)
			if err != nil {
				return err
			}
			if err := ValidateIslandEventExpressionTypedWithEvents(attr.Value, ctx.readSymbols(), ctx.stateTypes, ctx.handlers, nil, ctx.emits); err != nil {
				return fmt.Errorf("%s: %w", attr.Name, err)
			}
			out.WriteString(` data-gowdk-on-`)
			out.WriteString(eventDirective.Event)
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(attr.Value))
			out.WriteString(`" data-gowdk-binding-on-`)
			out.WriteString(eventDirective.Event)
			out.WriteString(`="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			if options := eventDirective.RuntimeOptions(); options != "" {
				out.WriteString(` data-gowdk-event-`)
				out.WriteString(eventDirective.Event)
				out.WriteString(`="`)
				out.WriteString(gowhtml.Escape(options))
				out.WriteByte('"')
			}
			continue
		}
		if attr.Name == "g:if" {
			if ctx.conditional != nil {
				continue
			}
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return fmt.Errorf("g:if requires an expression value")
			}
			if err := ValidateIslandBoolExpression(attr.Value, ctx.readFields); err != nil {
				return fmt.Errorf("g:if: %w", err)
			}
			out.WriteString(` data-gowdk-if="`)
			out.WriteString(gowhtml.Escape(attr.Value))
			out.WriteString(`" data-gowdk-binding-if="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			if visible, err := clientlang.EvalBool(attr.Value, ctx.values); err == nil && !visible {
				out.WriteString(` hidden`)
			}
			continue
		}
		if attr.Name == "g:else-if" || attr.Name == "g:else" {
			if ctx.conditional != nil {
				continue
			}
			return fmt.Errorf("%s must follow a sibling g:if or g:else-if", attr.Name)
		}
		if attr.Name == "g:for" || attr.Name == "g:key" {
			continue
		}
		if attr.Name == "g:bind:value" {
			out.WriteString(` data-gowdk-bind-value="`)
			out.WriteString(gowhtml.Escape(valueBinding))
			out.WriteString(`" data-gowdk-binding-value="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			if bindingType := valueBindingRuntimeType(valueBinding, ctx.stateTypes); bindingType != "" {
				out.WriteString(` data-gowdk-bind-type="`)
				out.WriteString(gowhtml.Escape(bindingType))
				out.WriteByte('"')
			}
			if node.Name == "input" {
				out.WriteString(` value="`)
				out.WriteString(gowhtml.Escape(ctx.values[valueBinding]))
				out.WriteByte('"')
			}
			continue
		}
		if attr.Name == "g:bind:checked" {
			out.WriteString(` data-gowdk-bind-checked="`)
			out.WriteString(gowhtml.Escape(checkedBinding))
			out.WriteString(`" data-gowdk-binding-checked="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			if ctx.values[checkedBinding] == "true" {
				out.WriteString(` checked`)
			}
			continue
		}
		if attr.Name == "g:ref" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return fmt.Errorf("g:ref requires a ref name")
			}
			refName := strings.TrimSpace(attr.Value)
			if err := validateDOMRef(refName, ctx.refs); err != nil {
				return fmt.Errorf("g:ref: %w", err)
			}
			out.WriteString(` data-gowdk-ref="`)
			out.WriteString(gowhtml.Escape(refName))
			out.WriteString(`" data-gowdk-binding-ref="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			continue
		}
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Name == "class" && !attr.Boolean {
			continue
		}
		if attr.Name == "style" && !attr.Boolean {
			continue
		}
		if node.Name == "option" && ctx.selectBound && attr.Name == "selected" {
			continue
		}
		if valueBinding != "" && node.staticInputType("radio") && attr.Name == "checked" {
			continue
		}
		if isClassToggleAttr(attr.Name) {
			continue
		}
		if isStyleBindingAttr(attr.Name) {
			continue
		}
		if valueBinding != "" && attr.Name == "value" && !node.staticInputType("radio") {
			return fmt.Errorf("element with g:bind:value must not declare value")
		}
		if checkedBinding != "" && attr.Name == "checked" {
			return fmt.Errorf("element with g:bind:checked must not declare checked")
		}
		if attr.Expression && len(ctx.readFields) > 0 {
			expr := expressionAttrSource(attr.Value)
			if err := validateReactiveAttr(attr.Name, expr, ctx.readFields); err != nil {
				return err
			}
			out.WriteString(` data-gowdk-attr-`)
			out.WriteString(attr.Name)
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(expr))
			out.WriteString(`" data-gowdk-binding-attr-`)
			out.WriteString(attr.Name)
			out.WriteString(`="`)
			out.WriteString(ctx.nextBindingID())
			out.WriteByte('"')
			value, ok, err := reactiveAttrValue(attr.Name, expr, ctx.values)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			out.WriteByte(' ')
			out.WriteString(attr.Name)
			if !isBooleanHTMLAttr(attr.Name) {
				out.WriteString(`="`)
				out.WriteString(gowhtml.Escape(value))
				out.WriteByte('"')
			}
			continue
		}
		if directives.Route != "" && (attr.Name == "method" || attr.Name == "action") {
			return fmt.Errorf("form with g:post must not declare %q", attr.Name)
		}
		value, tainted, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return err
		}
		if tainted && unsafeRouteParamAttr(attr.Name) {
			return fmt.Errorf("route param interpolation is not allowed in %q attributes", attr.Name)
		}
		out.WriteByte(' ')
		out.WriteString(attr.Name)
		if attr.Value != "" || !attr.Boolean {
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(value))
			out.WriteByte('"')
		}
	}
	if selected, err := node.optionSelected(ctx); err != nil {
		return err
	} else if selected {
		out.WriteString(` selected`)
	}
	if checked, err := node.radioChecked(ctx, valueBinding); err != nil {
		return err
	} else if checked {
		out.WriteString(` checked`)
	}
	if directives.Route != "" {
		out.WriteString(` method="post" action="`)
		out.WriteString(gowhtml.Escape(directives.Route))
		out.WriteByte('"')
	}
	if directives.Target != "" {
		out.WriteString(` data-gowdk-target="`)
		out.WriteString(gowhtml.Escape(directives.Target))
		out.WriteByte('"')
	}
	if directives.Swap != "" {
		out.WriteString(` data-gowdk-swap="`)
		out.WriteString(gowhtml.Escape(directives.Swap))
		out.WriteByte('"')
	}
	if ctx.conditional != nil {
		out.WriteString(` data-gowdk-if-group="`)
		out.WriteString(gowhtml.Escape(ctx.conditional.Group))
		out.WriteString(`" data-gowdk-if-index="`)
		out.WriteString(strconv.Itoa(ctx.conditional.Index))
		out.WriteString(`" data-gowdk-binding-if="`)
		out.WriteString(ctx.nextBindingID())
		out.WriteByte('"')
		if ctx.conditional.Condition != "" {
			out.WriteString(` data-gowdk-if="`)
			out.WriteString(gowhtml.Escape(ctx.conditional.Condition))
			out.WriteByte('"')
		} else {
			out.WriteString(` data-gowdk-else`)
		}
		if !ctx.conditional.Visible {
			out.WriteString(` hidden`)
		}
	}
	out.WriteByte('>')
	childCtx := ctx
	if ctx.conditional != nil {
		next := *ctx
		next.conditional = nil
		childCtx = &next
	}
	if ctx.loopItem != nil {
		next := *childCtx
		next.loopItem = nil
		childCtx = &next
	}
	if node.Name == "select" && valueBinding != "" {
		next := *childCtx
		next.selectBound = true
		next.selectValue = ctx.values[valueBinding]
		childCtx = &next
	}
	if node.Name == "textarea" && valueBinding != "" {
		out.WriteString(gowhtml.Escape(ctx.values[valueBinding]))
	} else {
		for _, child := range node.Children {
			if err := child.render(childCtx, out); err != nil {
				return err
			}
		}
	}
	out.WriteString("</")
	out.WriteString(node.Name)
	out.WriteByte('>')
	if ctx.conditional != nil {
		out.WriteString(`<!--gowdk-if:`)
		out.WriteString(gowhtml.Escape(ctx.conditional.Marker()))
		out.WriteString(`:end-->`)
	}
	return nil
}

func (node Element) optionSelected(ctx *renderContext) (bool, error) {
	if node.Name != "option" || !ctx.selectBound {
		return false, nil
	}
	value, err := node.optionValue(ctx)
	if err != nil {
		return false, err
	}
	return value == ctx.selectValue, nil
}

func (node Element) optionValue(ctx *renderContext) (string, error) {
	for _, attr := range node.Attrs {
		if attr.Name != "value" || attr.Boolean {
			continue
		}
		value, _, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return "", err
		}
		return value, nil
	}
	var text strings.Builder
	for _, child := range node.Children {
		typed, ok := child.(Text)
		if !ok {
			return "", nil
		}
		value, _, err := interpolateValue(ctx, typed.Value)
		if err != nil {
			return "", err
		}
		text.WriteString(value)
	}
	return strings.TrimSpace(text.String()), nil
}

func (node Element) radioChecked(ctx *renderContext, field string) (bool, error) {
	if field == "" || node.Name != "input" || !node.staticInputType("radio") {
		return false, nil
	}
	value, ok, err := node.staticAttrInterpolated(ctx, "value")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("g:bind:value on radio <input> requires a static value attribute")
	}
	return value == ctx.values[field], nil
}

func (node Element) staticAttrInterpolated(ctx *renderContext, name string) (string, bool, error) {
	for _, attr := range node.Attrs {
		if attr.Name != name || attr.Boolean {
			continue
		}
		value, _, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return "", false, err
		}
		return value, true, nil
	}
	return "", false, nil
}

type styleBinding struct {
	Property   string
	Unit       string
	Expression string
}

func (node Element) styleBindings(ctx *renderContext) ([]styleBinding, error) {
	var bindings []styleBinding
	for _, attr := range node.Attrs {
		if !isStyleBindingAttr(attr.Name) {
			continue
		}
		binding, err := parseStyleBindingAttr(attr.Name)
		if err != nil {
			return nil, err
		}
		if !attr.Expression || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return nil, fmt.Errorf("style binding directive %q requires an expression value", attr.Name)
		}
		expr := expressionAttrSource(attr.Value)
		if err := validateStyleBinding(expr, ctx.readFields); err != nil {
			return nil, fmt.Errorf("style binding %s: %w", attr.Name, err)
		}
		binding.Expression = expr
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func (node Element) initialStyleValue(ctx *renderContext, bindings []styleBinding) (string, error) {
	var declarations []string
	for _, attr := range node.Attrs {
		if attr.Name != "style" || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		value, tainted, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return "", err
		}
		if tainted && unsafeRouteParamAttr(attr.Name) {
			return "", fmt.Errorf("route param interpolation is not allowed in %q attributes", attr.Name)
		}
		declarations = append(declarations, strings.TrimSpace(value))
	}
	for _, binding := range bindings {
		value, err := clientlang.EvalScalar(binding.Expression, ctx.values)
		if err != nil || value == "" {
			continue
		}
		declarations = append(declarations, binding.Property+": "+value+binding.Unit)
	}
	return strings.Join(declarations, "; "), nil
}

func isStyleBindingAttr(name string) bool {
	return strings.HasPrefix(name, "style:")
}

func parseStyleBindingAttr(name string) (styleBinding, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(name, "style:"))
	if raw == "" {
		return styleBinding{}, fmt.Errorf("style binding directive %q requires a property name", name)
	}
	property := raw
	unit := ""
	if dot := strings.LastIndex(raw, "."); dot >= 0 {
		property = raw[:dot]
		unit = raw[dot+1:]
		if unit == "" {
			return styleBinding{}, fmt.Errorf("style binding directive %q has empty unit suffix", name)
		}
		if unit == "%" {
			unit = "%"
		} else if !isSupportedStyleUnit(unit) {
			return styleBinding{}, fmt.Errorf("style binding directive %q uses unsupported unit suffix %q", name, unit)
		}
	}
	if !isSupportedStyleProperty(property) {
		return styleBinding{}, fmt.Errorf("style binding directive %q has unsupported property name %q", name, property)
	}
	return styleBinding{Property: property, Unit: unit}, nil
}

func isSupportedStyleProperty(name string) bool {
	if strings.HasPrefix(name, "--") {
		return len(name) > 2 && styleCustomPropertyPattern.MatchString(name)
	}
	return stylePropertyPattern.MatchString(name)
}

func isSupportedStyleUnit(unit string) bool {
	switch unit {
	case "px", "rem", "em", "vh", "vw", "vmin", "vmax", "ch", "ex", "lh", "rlh", "ms", "s", "%":
		return true
	default:
		return false
	}
}

type classToggle struct {
	Name       string
	Expression string
}

func (node Element) classToggles(ctx *renderContext) ([]classToggle, error) {
	var toggles []classToggle
	for _, attr := range node.Attrs {
		if !isClassToggleAttr(attr.Name) {
			continue
		}
		name := classToggleName(attr.Name)
		if name == "" {
			return nil, fmt.Errorf("class toggle directive %q requires a class name", attr.Name)
		}
		if !attr.Expression || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return nil, fmt.Errorf("class toggle directive %q requires an expression value", attr.Name)
		}
		expr := expressionAttrSource(attr.Value)
		if err := ValidateIslandBoolExpression(expr, ctx.readFields); err != nil {
			return nil, fmt.Errorf("class toggle %s: %w", attr.Name, err)
		}
		toggles = append(toggles, classToggle{Name: name, Expression: expr})
	}
	return toggles, nil
}

func (node Element) initialClassValue(ctx *renderContext, toggles []classToggle) string {
	var classes []string
	for _, attr := range node.Attrs {
		if attr.Name != "class" || attr.Boolean {
			continue
		}
		for _, className := range strings.Fields(attr.Value) {
			classes = appendUniqueClass(classes, className)
		}
	}
	for _, toggle := range toggles {
		active, err := clientlang.EvalBool(toggle.Expression, ctx.values)
		if err == nil && active {
			classes = appendUniqueClass(classes, toggle.Name)
		}
	}
	return strings.Join(classes, " ")
}

func appendUniqueClass(classes []string, className string) []string {
	if className == "" {
		return classes
	}
	for _, existing := range classes {
		if existing == className {
			return classes
		}
	}
	return append(classes, className)
}

func isClassToggleAttr(name string) bool {
	return strings.HasPrefix(name, "class:")
}

func classToggleName(name string) string {
	return strings.TrimSpace(strings.TrimPrefix(name, "class:"))
}

func (node Element) valueBinding(ctx *renderContext) (string, error) {
	field := ""
	for _, attr := range node.Attrs {
		if attr.Name != "g:bind:value" {
			continue
		}
		if field != "" {
			return "", fmt.Errorf("element declares multiple g:bind:value directives")
		}
		if node.Name != "input" && node.Name != "textarea" && node.Name != "select" {
			return "", fmt.Errorf("g:bind:value is only supported on <input>, <textarea>, and <select> in this build slice")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", fmt.Errorf("g:bind:value requires a field value")
		}
		field = strings.TrimSpace(attr.Value)
		if err := validateIslandField(field, ctx.stateFields); err != nil {
			return "", fmt.Errorf("g:bind:value: %w", err)
		}
		if node.Name == "input" && node.staticInputType("radio") {
			if _, ok, err := node.staticAttrInterpolated(ctx, "value"); err != nil {
				return "", err
			} else if !ok {
				return "", fmt.Errorf("g:bind:value on radio <input> requires a static value attribute")
			}
		}
		typ := ctx.stateTypes[field]
		if typ == clientlang.TypeInt || typ == clientlang.TypeFloat {
			if node.Name != "input" || !node.staticInputType("number") {
				return "", fmt.Errorf("g:bind:value numeric target %q requires <input type=\"number\">", field)
			}
		}
		if typ == clientlang.TypeBool {
			return "", fmt.Errorf("g:bind:value target %q must be string or numeric, got %s", field, typ)
		}
	}
	return field, nil
}

func valueBindingRuntimeType(field string, stateTypes map[string]clientlang.ValueType) string {
	switch stateTypes[field] {
	case clientlang.TypeInt:
		return "int"
	case clientlang.TypeFloat:
		return "float"
	default:
		return ""
	}
}

func validateDOMRef(name string, refs map[string]clientlang.Ref) error {
	if !islandFieldPattern.MatchString(name) {
		return fmt.Errorf("invalid DOM ref %q", name)
	}
	if refs == nil {
		return nil
	}
	if _, ok := refs[name]; !ok {
		return fmt.Errorf("unknown DOM ref %q", name)
	}
	return nil
}

func (node Element) checkedBinding(ctx *renderContext) (string, error) {
	field := ""
	for _, attr := range node.Attrs {
		if attr.Name != "g:bind:checked" {
			continue
		}
		if field != "" {
			return "", fmt.Errorf("element declares multiple g:bind:checked directives")
		}
		if node.Name != "input" || !node.staticInputType("checkbox") {
			return "", fmt.Errorf("g:bind:checked is only supported on checkbox <input> in this build slice")
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", fmt.Errorf("g:bind:checked requires a field value")
		}
		field = strings.TrimSpace(attr.Value)
		if err := validateIslandField(field, ctx.stateFields); err != nil {
			return "", fmt.Errorf("g:bind:checked: %w", err)
		}
	}
	return field, nil
}

func (node Element) staticInputType(value string) bool {
	for _, attr := range node.Attrs {
		if attr.Name != "type" || attr.Boolean {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(attr.Value), value)
	}
	return value == "text"
}

type postDirectives struct {
	Action string
	Route  string
	Target string
	Swap   string
}

func (node Element) postDirectives(ctx *renderContext) (postDirectives, error) {
	directives, err := node.directiveValues()
	if err != nil {
		return postDirectives{}, err
	}
	if directives.Action == "" {
		if directives.Target != "" || directives.Swap != "" {
			return postDirectives{}, fmt.Errorf("g:target and g:swap require g:post")
		}
		return postDirectives{}, nil
	}
	route, ok := ctx.actions[directives.Action]
	if !ok {
		return postDirectives{}, fmt.Errorf("unknown action %q for g:post", directives.Action)
	}
	directives.Route = route
	return directives, nil
}

func (node Element) postActionName() (string, error) {
	directives, err := node.directiveValues()
	if err != nil {
		return "", err
	}
	return directives.Action, nil
}

func (node Element) directiveValues() (postDirectives, error) {
	var directives postDirectives
	for _, attr := range node.Attrs {
		if !strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if strings.HasPrefix(attr.Name, "g:on:") {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("%s requires an expression value", attr.Name)
			}
			continue
		}
		if attr.Name == "g:if" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:if requires an expression value")
			}
			continue
		}
		if attr.Name == "g:else-if" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:else-if requires an expression value")
			}
			continue
		}
		if attr.Name == "g:else" {
			if !attr.Boolean && strings.TrimSpace(attr.Value) != "" {
				return postDirectives{}, fmt.Errorf("g:else must not have a value")
			}
			continue
		}
		if attr.Name == "g:for" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:for requires an expression value")
			}
			continue
		}
		if attr.Name == "g:key" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:key requires an expression value")
			}
			continue
		}
		if attr.Name == "g:bind:value" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:bind:value requires a field value")
			}
			continue
		}
		if attr.Name == "g:bind:checked" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:bind:checked requires a field value")
			}
			continue
		}
		if attr.Name == "g:ref" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:ref requires a ref name")
			}
			continue
		}
		if attr.Name != "g:post" && attr.Name != "g:target" && attr.Name != "g:swap" {
			return postDirectives{}, fmt.Errorf("unsupported directive attribute %q in static build", attr.Name)
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return postDirectives{}, fmt.Errorf("%s requires a value", attr.Name)
		}
		if node.Name != "form" {
			return postDirectives{}, fmt.Errorf("%s is only supported on <form>", attr.Name)
		}
		switch attr.Name {
		case "g:post":
			if directives.Action != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:post directives")
			}
			directives.Action = strings.TrimSpace(attr.Value)
		case "g:target":
			if directives.Target != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:target directives")
			}
			target := strings.TrimSpace(attr.Value)
			if strings.ContainsAny(target, "{}") {
				return postDirectives{}, fmt.Errorf("g:target %q must be static", target)
			}
			if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n") {
				return postDirectives{}, fmt.Errorf("g:target %q must be a static id selector", target)
			}
			directives.Target = target
		case "g:swap":
			if directives.Swap != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:swap directives")
			}
			swap := strings.TrimSpace(attr.Value)
			if !isSupportedSwapMode(swap) {
				return postDirectives{}, fmt.Errorf("unsupported g:swap mode %q", swap)
			}
			directives.Swap = swap
		}
	}
	if directives.Swap != "" && directives.Target == "" {
		return postDirectives{}, fmt.Errorf("g:swap requires g:target")
	}
	return directives, nil
}

func isSupportedSwapMode(value string) bool {
	switch value {
	case "innerHTML", "outerHTML":
		return true
	default:
		return false
	}
}

func validateReactiveAttr(name, expr string, fields map[string]bool) error {
	if unsafeReactiveAttr(name) {
		return fmt.Errorf("reactive attribute %q is not supported before safe URL/style/event rules are defined", name)
	}
	_, _, err := clientlang.CheckExpr(expr, boolFieldSymbols(fields))
	if err != nil {
		return fmt.Errorf("attribute %s: %w", name, err)
	}
	return nil
}

func validateStyleBinding(expr string, fields map[string]bool) error {
	_, _, err := clientlang.CheckExpr(expr, boolFieldSymbols(fields))
	return err
}

func reactiveAttrValue(name, expr string, values map[string]string) (string, bool, error) {
	if isBooleanHTMLAttr(name) {
		visible, err := clientlang.EvalBool(expr, values)
		if err != nil {
			return "", false, err
		}
		return "", visible, nil
	}
	value, err := clientlang.EvalScalar(expr, values)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func unsafeReactiveAttr(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "on") && len(name) > 2 {
		return true
	}
	if name == "style" {
		return true
	}
	switch name {
	case "href", "src", "srcset", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href":
		return true
	default:
		return false
	}
}

func isBooleanHTMLAttr(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected":
		return true
	default:
		return false
	}
}

func expressionAttrSource(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

// EventDirective is a parsed g:on:<event>[.<modifier>] directive.
type EventDirective struct {
	Event      string
	Prevent    bool
	Stop       bool
	Once       bool
	Capture    bool
	DebounceMS int
	ThrottleMS int
}

// ParseEventDirective validates and splits a g:on directive name.
func ParseEventDirective(name string) (EventDirective, error) {
	if !strings.HasPrefix(name, "g:on:") {
		return EventDirective{}, fmt.Errorf("event directive %q must start with g:on:", name)
	}
	raw := strings.TrimPrefix(name, "g:on:")
	if raw == "" {
		return EventDirective{}, fmt.Errorf("g:on directive requires an event name")
	}
	parts := strings.Split(raw, ".")
	directive := EventDirective{Event: parts[0]}
	if !eventNamePattern.MatchString(directive.Event) {
		return EventDirective{}, fmt.Errorf("g:on directive has invalid event name %q", directive.Event)
	}
	seen := map[string]bool{}
	for _, modifier := range parts[1:] {
		if modifier == "" {
			return EventDirective{}, fmt.Errorf("g:on:%s has an empty event modifier", raw)
		}
		key := modifier
		if strings.HasPrefix(modifier, "debounce(") {
			key = "debounce"
		} else if strings.HasPrefix(modifier, "throttle(") {
			key = "throttle"
		}
		if seen[key] {
			return EventDirective{}, fmt.Errorf("g:on:%s repeats %s modifier", raw, key)
		}
		seen[key] = true
		switch modifier {
		case "prevent":
			directive.Prevent = true
		case "stop":
			directive.Stop = true
		case "once":
			directive.Once = true
		case "capture":
			directive.Capture = true
		default:
			if strings.HasPrefix(modifier, "debounce(") && strings.HasSuffix(modifier, ")") {
				ms, err := parseEventDurationMS(strings.TrimSuffix(strings.TrimPrefix(modifier, "debounce("), ")"))
				if err != nil {
					return EventDirective{}, fmt.Errorf("g:on:%s has invalid debounce duration: %w", raw, err)
				}
				directive.DebounceMS = ms
				continue
			}
			if strings.HasPrefix(modifier, "throttle(") && strings.HasSuffix(modifier, ")") {
				ms, err := parseEventDurationMS(strings.TrimSuffix(strings.TrimPrefix(modifier, "throttle("), ")"))
				if err != nil {
					return EventDirective{}, fmt.Errorf("g:on:%s has invalid throttle duration: %w", raw, err)
				}
				directive.ThrottleMS = ms
				continue
			}
			return EventDirective{}, fmt.Errorf("g:on:%s uses unsupported event modifier %q", raw, modifier)
		}
	}
	if directive.DebounceMS > 0 && directive.ThrottleMS > 0 {
		return EventDirective{}, fmt.Errorf("g:on:%s cannot combine debounce and throttle", raw)
	}
	return directive, nil
}

func parseEventDurationMS(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	multiplier := 1
	number := value
	switch {
	case strings.HasSuffix(value, "ms"):
		number = strings.TrimSuffix(value, "ms")
	case strings.HasSuffix(value, "s"):
		number = strings.TrimSuffix(value, "s")
		multiplier = 1000
	default:
		return 0, fmt.Errorf("duration must use ms or s")
	}
	raw, err := strconv.Atoi(strings.TrimSpace(number))
	if err != nil || raw <= 0 {
		return 0, fmt.Errorf("duration must be a positive integer")
	}
	return raw * multiplier, nil
}

// RuntimeOptions returns the compact modifier string emitted into HTML.
func (directive EventDirective) RuntimeOptions() string {
	var options []string
	if directive.Prevent {
		options = append(options, "prevent")
	}
	if directive.Stop {
		options = append(options, "stop")
	}
	if directive.Once {
		options = append(options, "once")
	}
	if directive.Capture {
		options = append(options, "capture")
	}
	if directive.DebounceMS > 0 {
		options = append(options, fmt.Sprintf("debounce:%d", directive.DebounceMS))
	}
	if directive.ThrottleMS > 0 {
		options = append(options, fmt.Sprintf("throttle:%d", directive.ThrottleMS))
	}
	return strings.Join(options, " ")
}

// ComponentCall invokes a parsed component with static string props.
type ComponentCall struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

func (node ComponentCall) render(ctx *renderContext, out *strings.Builder) error {
	component, ok := ctx.components[node.Name]
	if !ok {
		return fmt.Errorf("missing component %q", node.Name)
	}
	if ctx.stack[node.Name] {
		return fmt.Errorf("recursive component %q", node.Name)
	}

	mode, err := node.islandMode()
	if err != nil {
		return err
	}
	props := map[string]string{}
	propExpressions := map[string]string{}
	taintedValues := map[string]bool{}
	var parentListeners []parentComponentListener
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			if attr.Name == "g:island" {
				continue
			}
			if strings.HasPrefix(attr.Name, "g:on:") {
				listener, err := node.parentListener(attr, component, ctx)
				if err != nil {
					return err
				}
				parentListeners = append(parentListeners, listener)
				continue
			}
			return fmt.Errorf("component %s uses unsupported directive attribute %q", node.Name, attr.Name)
		}
		if attr.Boolean {
			return fmt.Errorf("component %s prop %q requires a string value", node.Name, attr.Name)
		}
		value, tainted, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return err
		}
		props[attr.Name] = value
		if attr.Expression {
			propExpressions[attr.Name] = expressionAttrSource(attr.Value)
		}
		if tainted {
			taintedValues[attr.Name] = true
		}
	}
	for _, prop := range component.Props {
		if _, ok := props[prop]; !ok {
			return fmt.Errorf("component %s missing required prop %q", node.Name, prop)
		}
	}
	for prop := range props {
		if !component.HasProp(prop) {
			return fmt.Errorf("component %s does not declare prop %q", node.Name, prop)
		}
	}
	slotHTML, err := renderNodes(node.Children, ctx)
	if err != nil {
		return err
	}

	values := mergeValues(component.State, props)
	computedStrings, computedValues, err := evalComputedValues(component.Computed, values)
	if err != nil {
		return err
	}
	values = mergeValues(values, computedStrings)
	bindValues := mergeValues(component.State, computedStrings)
	if len(propExpressions) > 0 {
		bindValues = mergeValues(bindValues, props)
	}
	childCtx := renderContext{
		components:  ctx.components,
		values:      values,
		tainted:     taintedValues,
		actions:     ctx.actions,
		stack:       cloneStack(ctx.stack),
		slotHTML:    slotHTML,
		stateFields: boolSet(keys(component.State)),
		readFields:  boolSet(keys(values)),
		bindFields:  boolSet(keys(bindValues)),
		handlers:    component.Handlers,
		stateTypes:  component.StateTypes,
		refs:        component.Refs,
		emits:       component.Emits,
		bindingSeq:  ctx.bindingSeq,
		islandSeq:   ctx.islandSeq,
	}
	childCtx.stack[node.Name] = true
	body, err := render(component.Body, childCtx)
	if err != nil {
		return err
	}
	if component.StateJSON != "" || component.HandlersJSON != "" || mode != "" || len(component.Emits) > 0 || len(propExpressions) > 0 {
		if mode == "" {
			mode = "js"
		}
		stateJSON := componentStateJSON(component.StateJSON, props, computedValues)
		if stateJSON == "" {
			stateJSON = "{}"
		}
		islandID := ctx.nextIslandID()
		out.WriteString(`<gowdk-island data-gowdk-component="`)
		out.WriteString(gowhtml.Escape(node.Name))
		out.WriteString(`" data-gowdk-island="`)
		out.WriteString(gowhtml.Escape(islandID))
		out.WriteString(`" data-gowdk-runtime="`)
		out.WriteString(gowhtml.Escape(mode))
		out.WriteString(`" data-gowdk-state="`)
		out.WriteString(gowhtml.Escape(stateJSON))
		out.WriteByte('"')
		if component.HandlersJSON != "" {
			out.WriteString(` data-gowdk-client="`)
			out.WriteString(gowhtml.Escape(component.HandlersJSON))
			out.WriteByte('"')
		}
		if len(propExpressions) > 0 {
			propsJSON, err := json.Marshal(propExpressions)
			if err != nil {
				return err
			}
			out.WriteString(` data-gowdk-props="`)
			out.WriteString(gowhtml.Escape(string(propsJSON)))
			out.WriteByte('"')
		}
		for _, listener := range parentListeners {
			out.WriteString(` data-gowdk-parent-on-`)
			out.WriteString(listener.Event)
			out.WriteString(`="`)
			out.WriteString(gowhtml.Escape(listener.Expression))
			out.WriteByte('"')
			if listener.Modifiers != "" {
				out.WriteString(` data-gowdk-parent-event-`)
				out.WriteString(listener.Event)
				out.WriteString(`="`)
				out.WriteString(gowhtml.Escape(listener.Modifiers))
				out.WriteByte('"')
			}
		}
		out.WriteByte('>')
		out.WriteString(body)
		out.WriteString(`</gowdk-island>`)
		return nil
	}
	out.WriteString(body)
	return nil
}

type parentComponentListener struct {
	Event      string
	Expression string
	Modifiers  string
}

func (node ComponentCall) parentListener(attr Attr, component Component, ctx *renderContext) (parentComponentListener, error) {
	if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return parentComponentListener{}, fmt.Errorf("%s requires an expression value", attr.Name)
	}
	directive, err := ParseEventDirective(attr.Name)
	if err != nil {
		return parentComponentListener{}, err
	}
	event, ok := component.Emits[directive.Event]
	if !ok {
		return parentComponentListener{}, fmt.Errorf("component %s does not emit event %q", node.Name, directive.Event)
	}
	readSymbols := mergeClientSymbols(ctx.readSymbols(), eventPayloadSymbols(event))
	if err := ValidateIslandEventExpressionTypedWithFunctions(attr.Value, readSymbols, ctx.stateTypes, ctx.handlers, nil); err != nil {
		return parentComponentListener{}, fmt.Errorf("%s: %w", attr.Name, err)
	}
	return parentComponentListener{
		Event:      directive.Event,
		Expression: attr.Value,
		Modifiers:  directive.RuntimeOptions(),
	}, nil
}

func eventPayloadSymbols(event clientlang.Emit) map[string]clientlang.ValueType {
	out := map[string]clientlang.ValueType{"event": clientlang.TypeObject}
	for index, name := range event.Params {
		typ := clientlang.TypeUnknown
		if index < len(event.ParamTypes) {
			typ = event.ParamTypes[index]
		}
		out["event."+name] = typ
	}
	return out
}

func (node ComponentCall) islandMode() (string, error) {
	mode := ""
	for _, attr := range node.Attrs {
		if attr.Name != "g:island" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", fmt.Errorf("component %s g:island requires a value", node.Name)
		}
		value := strings.TrimSpace(attr.Value)
		if value != "wasm" {
			return "", fmt.Errorf("component %s uses unsupported g:island value %q", node.Name, value)
		}
		mode = value
	}
	return mode, nil
}

func renderNodes(nodes []Node, ctx *renderContext) (string, error) {
	if len(nodes) == 0 {
		return "", nil
	}
	var out strings.Builder
	groupSeq := 0
	inChain := false
	chainMatched := false
	chainIndex := 0
	for _, node := range nodes {
		nodeCtx := ctx
		if element, ok := node.(Element); ok {
			branch, err := conditionalBranch(element, ctx)
			if err != nil {
				return "", err
			}
			switch branch.Kind {
			case "if":
				groupSeq++
				inChain = true
				chainMatched = branch.Visible
				chainIndex = 0
				next := *ctx
				next.conditional = &conditionalRender{
					Group:     fmt.Sprintf("c%d", groupSeq),
					Index:     chainIndex,
					Condition: branch.Condition,
					Visible:   branch.Visible,
				}
				nodeCtx = &next
			case "else-if":
				if !inChain {
					return "", fmt.Errorf("g:else-if must follow a sibling g:if or g:else-if")
				}
				chainIndex++
				visible := !chainMatched && branch.Visible
				if visible {
					chainMatched = true
				}
				next := *ctx
				next.conditional = &conditionalRender{
					Group:     fmt.Sprintf("c%d", groupSeq),
					Index:     chainIndex,
					Condition: branch.Condition,
					Visible:   visible,
				}
				nodeCtx = &next
			case "else":
				if !inChain {
					return "", fmt.Errorf("g:else must follow a sibling g:if or g:else-if")
				}
				chainIndex++
				visible := !chainMatched
				chainMatched = true
				next := *ctx
				next.conditional = &conditionalRender{
					Group:   fmt.Sprintf("c%d", groupSeq),
					Index:   chainIndex,
					Visible: visible,
				}
				nodeCtx = &next
				inChain = false
			default:
				inChain = false
				chainMatched = false
				chainIndex = 0
			}
		} else if !ignorableConditionalSeparator(node) {
			inChain = false
			chainMatched = false
			chainIndex = 0
		}
		if err := node.render(nodeCtx, &out); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

type conditionalRender struct {
	Group     string
	Index     int
	Condition string
	Visible   bool
}

func (conditional conditionalRender) Marker() string {
	return conditional.Group + "-" + strconv.Itoa(conditional.Index)
}

type conditionalBranchInfo struct {
	Kind      string
	Condition string
	Visible   bool
}

func conditionalBranch(node Element, ctx *renderContext) (conditionalBranchInfo, error) {
	hasIf := false
	hasElseIf := false
	hasElse := false
	var condition string
	for _, attr := range node.Attrs {
		switch attr.Name {
		case "g:if":
			hasIf = true
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:if requires an expression value")
			}
			condition = strings.TrimSpace(attr.Value)
		case "g:else-if":
			hasElseIf = true
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:else-if requires an expression value")
			}
			condition = strings.TrimSpace(attr.Value)
		case "g:else":
			hasElse = true
			if !attr.Boolean && strings.TrimSpace(attr.Value) != "" {
				return conditionalBranchInfo{}, fmt.Errorf("g:else must not have a value")
			}
		}
	}
	count := 0
	for _, set := range []bool{hasIf, hasElseIf, hasElse} {
		if set {
			count++
		}
	}
	if count == 0 {
		return conditionalBranchInfo{}, nil
	}
	if count > 1 {
		return conditionalBranchInfo{}, fmt.Errorf("element cannot combine g:if, g:else-if, and g:else")
	}
	if hasElse {
		return conditionalBranchInfo{Kind: "else"}, nil
	}
	if err := ValidateIslandBoolExpression(condition, ctx.readFields); err != nil {
		if hasIf {
			return conditionalBranchInfo{}, fmt.Errorf("g:if: %w", err)
		}
		return conditionalBranchInfo{}, fmt.Errorf("g:else-if: %w", err)
	}
	visible := false
	if evaluated, err := clientlang.EvalBool(condition, ctx.values); err == nil {
		visible = evaluated
	}
	if hasIf {
		return conditionalBranchInfo{Kind: "if", Condition: condition, Visible: visible}, nil
	}
	return conditionalBranchInfo{Kind: "else-if", Condition: condition, Visible: visible}, nil
}

func ignorableConditionalSeparator(node Node) bool {
	text, ok := node.(Text)
	return ok && strings.TrimSpace(text.Value) == ""
}

// Attr is a static HTML attribute.
type Attr struct {
	Name       string
	Value      string
	Boolean    bool
	Expression bool
}

// Component is a static component template known to the view renderer.
type Component struct {
	Name         string
	Props        []string
	State        map[string]string
	StateJSON    string
	Handlers     map[string]clientlang.Handler
	HandlersJSON string
	StateTypes   map[string]clientlang.ValueType
	Refs         map[string]clientlang.Ref
	Emits        map[string]clientlang.Emit
	Computed     []clientlang.Computed
	Body         string
}

// HasProp reports whether a component declares a prop.
func (component Component) HasProp(name string) bool {
	for _, prop := range component.Props {
		if prop == name {
			return true
		}
	}
	return false
}

// Parse parses a static markup fragment.
func Parse(source string) ([]Node, error) {
	parser := parser{source: []rune(source)}
	nodes, err := parser.nodes("")
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if !parser.done() {
		return nil, parser.errorf("unexpected content")
	}
	return nodes, nil
}

// RenderStatic renders a static markup fragment with escaped text and attrs.
func RenderStatic(source string) (string, error) {
	return RenderWithComponents(source, nil)
}

// RenderWithComponents renders a static markup fragment with component support.
func RenderWithComponents(source string, components map[string]Component) (string, error) {
	return RenderWithData(source, components, nil)
}

// RenderWithData renders a static markup fragment with component support and
// string interpolation data.
func RenderWithData(source string, components map[string]Component, data map[string]string) (string, error) {
	return RenderWithOptions(source, components, data, Options{})
}

// Options configures static view rendering.
type Options struct {
	Actions map[string]string
}

// ActionFormField describes one direct static form field for a g:post action.
type ActionFormField struct {
	Name     string
	Required bool
}

// Dependencies records static dependencies visible in the first view subset.
type Dependencies struct {
	StaticAssets    []string
	CSSClasses      []string
	StyleAttributes []string
}

// ComponentIslandUsage records one component call that explicitly selects an
// island runtime.
type ComponentIslandUsage struct {
	Component string
	Mode      string
}

// ComponentCallUsage records one component call and its optional island mode.
type ComponentCallUsage struct {
	Component     string
	Island        string
	ReactiveProps bool
}

// RenderWithOptions renders a static markup fragment with component support,
// interpolation data, and page-scoped action routes.
func RenderWithOptions(source string, components map[string]Component, data map[string]string, options Options) (string, error) {
	bindingSeq := 0
	islandSeq := 0
	return render(source, renderContext{
		components:  components,
		values:      cloneValues(data),
		actions:     cloneValues(options.Actions),
		stack:       map[string]bool{},
		stateFields: map[string]bool{},
		readFields:  map[string]bool{},
		bindFields:  map[string]bool{},
		bindingSeq:  &bindingSeq,
		islandSeq:   &islandSeq,
	})
}

// ActionFormFields returns direct static HTML control names grouped by g:post
// action name. Component-hidden controls are intentionally not inferred in this
// first decoder slice.
func ActionFormFields(source string) (map[string][]string, error) {
	schema, err := ActionFormSchema(source)
	if err != nil {
		return nil, err
	}
	fields := map[string][]string{}
	for action, actionFields := range schema {
		for _, field := range actionFields {
			fields[action] = append(fields[action], field.Name)
		}
	}
	return fields, nil
}

// ViewDependencies returns direct static asset and style references from a
// static markup fragment. Interpolated and external URLs are not reported.
func ViewDependencies(source string) (Dependencies, error) {
	nodes, err := Parse(source)
	if err != nil {
		return Dependencies{}, err
	}
	assets := map[string]bool{}
	classes := map[string]bool{}
	styles := map[string]bool{}
	collectViewDependencies(nodes, assets, classes, styles)
	return Dependencies{
		StaticAssets:    sortedKeys(assets),
		CSSClasses:      sortedKeys(classes),
		StyleAttributes: sortedKeys(styles),
	}, nil
}

// ActionFormSchema returns direct static HTML controls grouped by g:post action
// name. Duplicate field names are merged, and Required is true if any matching
// direct control is required.
func ActionFormSchema(source string) (map[string][]ActionFormField, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	fields := map[string]map[string]ActionFormField{}
	if err := collectActionFormFields(nodes, fields); err != nil {
		return nil, err
	}
	schema := map[string][]ActionFormField{}
	for action := range fields {
		names := make([]string, 0, len(fields[action]))
		for name := range fields[action] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			schema[action] = append(schema[action], fields[action][name])
		}
	}
	return schema, nil
}

// ComponentReferences returns unique component names directly referenced by a
// static markup fragment.
func ComponentReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	collectComponentReferences(nodes, names)
	if len(names) == 0 {
		return nil, nil
	}
	refs := make([]string, 0, len(names))
	for name := range names {
		refs = append(refs, name)
	}
	sort.Strings(refs)
	return refs, nil
}

// ComponentIslandUsages returns component calls that explicitly set g:island.
func ComponentIslandUsages(source string) ([]ComponentIslandUsage, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var usages []ComponentIslandUsage
	if err := collectComponentIslandUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

// ComponentCallUsages returns component calls with optional g:island metadata.
func ComponentCallUsages(source string) ([]ComponentCallUsage, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	var usages []ComponentCallUsage
	if err := collectComponentCallUsages(nodes, &usages); err != nil {
		return nil, err
	}
	return usages, nil
}

// Canonical returns a deterministic AST-backed representation of a view body.
func Canonical(source string) (string, error) {
	nodes, err := Parse(stripLineComments(source))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	writeCanonicalNodes(&out, nodes)
	return out.String(), nil
}

func writeCanonicalNodes(out *strings.Builder, nodes []Node) {
	for _, node := range nodes {
		writeCanonicalNode(out, node)
	}
}

func writeCanonicalNode(out *strings.Builder, node Node) {
	switch typed := node.(type) {
	case Text:
		out.WriteString("text(")
		out.WriteString(strconv.Quote(strings.Join(strings.Fields(typed.Value), " ")))
		out.WriteByte(')')
	case Element:
		out.WriteString("element(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	case ComponentCall:
		out.WriteString("component(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	}
}

func writeCanonicalAttrs(out *strings.Builder, attrs []Attr) {
	normalized := make([]Attr, 0, len(attrs))
	for _, attr := range attrs {
		value := strings.TrimSpace(attr.Value)
		if attr.Name == "class" {
			classes := strings.Fields(value)
			sort.Strings(classes)
			value = strings.Join(classes, " ")
		}
		value = canonicalAttrValue(attr.Name, value, attr.Expression)
		normalized = append(normalized, Attr{Name: attr.Name, Value: value, Boolean: attr.Boolean, Expression: attr.Expression})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		if normalized[i].Value != normalized[j].Value {
			return normalized[i].Value < normalized[j].Value
		}
		return !normalized[i].Boolean && normalized[j].Boolean
	})
	out.WriteByte('{')
	for index, attr := range normalized {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(attr.Name)
		if attr.Boolean {
			out.WriteString(":bool")
			continue
		}
		if attr.Expression {
			out.WriteString(":expr")
		}
		out.WriteByte('=')
		out.WriteString(strconv.Quote(attr.Value))
	}
	out.WriteByte('}')
}

func canonicalAttrValue(name string, value string, expression bool) string {
	if strings.HasPrefix(name, "g:on:") {
		return clientlang.CanonicalStatement(value)
	}
	if expression || name == "g:if" || name == "g:else-if" || name == "g:key" ||
		strings.HasPrefix(name, "class:") || strings.HasPrefix(name, "style:") {
		expr := expressionAttrSource(value)
		if canonical, err := clientlang.CanonicalExpr(expr); err == nil {
			return canonical
		}
		return strings.Join(strings.Fields(expr), " ")
	}
	return value
}

// ParamReferences returns unique param("name") route-param references directly
// visible in the current static markup subset.
func ParamReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	collectParamReferences(nodes, names)
	return sortedKeys(names), nil
}

func render(source string, ctx renderContext) (string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return "", err
	}
	if err := validateFragmentTargetReferences(nodes); err != nil {
		return "", err
	}
	if ctx.loopSeq == nil {
		seq := 0
		ctx.loopSeq = &seq
	}
	if ctx.bindingSeq == nil {
		seq := 0
		ctx.bindingSeq = &seq
	}
	if ctx.islandSeq == nil {
		seq := 0
		ctx.islandSeq = &seq
	}
	return renderNodes(nodes, &ctx)
}

func validateFragmentTargetReferences(nodes []Node) error {
	ids := map[string]bool{}
	targets := map[string]bool{}
	collectIDsAndTargets(nodes, ids, targets)
	for target := range targets {
		id := strings.TrimPrefix(target, "#")
		if !ids[id] {
			return fmt.Errorf("g:target %q does not reference a static id in this view", target)
		}
	}
	return nil
}

func collectIDsAndTargets(nodes []Node, ids map[string]bool, targets map[string]bool) {
	for _, node := range nodes {
		element, ok := node.(Element)
		if !ok {
			continue
		}
		hasPost := false
		for _, attr := range element.Attrs {
			if attr.Name == "g:post" {
				hasPost = true
				break
			}
		}
		for _, attr := range element.Attrs {
			if attr.Boolean {
				continue
			}
			switch attr.Name {
			case "id":
				id := strings.TrimSpace(attr.Value)
				if id != "" && !strings.ContainsAny(id, "{}") {
					ids[id] = true
				}
			case "g:target":
				target := strings.TrimSpace(attr.Value)
				if hasPost && target != "" && !strings.ContainsAny(target, "{}") {
					targets[target] = true
				}
			}
		}
		collectIDsAndTargets(element.Children, ids, targets)
	}
}

func collectParamReferences(nodes []Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Text:
			collectParamReferencesFromString(typed.Value, names)
		case Element:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		case ComponentCall:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		}
	}
}

func collectParamReferencesFromString(value string, names map[string]bool) {
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			return
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if name, ok := routeParamExpression(expr); ok {
			names[name] = true
		}
		value = value[end+1:]
	}
}

func collectComponentReferences(nodes []Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			names[typed.Name] = true
			collectComponentReferences(typed.Children, names)
		case Element:
			collectComponentReferences(typed.Children, names)
		}
	}
}

func collectComponentIslandUsages(nodes []Node, usages *[]ComponentIslandUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			if mode != "" {
				*usages = append(*usages, ComponentIslandUsage{Component: typed.Name, Mode: mode})
			}
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentIslandUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectComponentCallUsages(nodes []Node, usages *[]ComponentCallUsage) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case ComponentCall:
			mode, err := typed.islandMode()
			if err != nil {
				return err
			}
			*usages = append(*usages, ComponentCallUsage{
				Component:     typed.Name,
				Island:        mode,
				ReactiveProps: typed.hasReactiveProps(),
			})
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		case Element:
			if err := collectComponentCallUsages(typed.Children, usages); err != nil {
				return err
			}
		}
	}
	return nil
}

func (node ComponentCall) hasReactiveProps() bool {
	for _, attr := range node.Attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Expression {
			return true
		}
	}
	return false
}

func collectViewDependencies(nodes []Node, assets, classes, styles map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			for _, attr := range typed.Attrs {
				switch attr.Name {
				case "class":
					for _, className := range strings.Fields(attr.Value) {
						if !strings.ContainsAny(className, "{}") {
							classes[className] = true
						}
					}
				case "style":
					style := strings.TrimSpace(attr.Value)
					if style != "" && !strings.ContainsAny(style, "{}") {
						styles[style] = true
					}
				case "src", "href", "poster":
					if isStaticAssetReference(attr.Value) {
						assets[strings.TrimSpace(attr.Value)] = true
					}
				}
			}
			collectViewDependencies(typed.Children, assets, classes, styles)
		case ComponentCall:
			collectViewDependencies(typed.Children, assets, classes, styles)
		}
	}
}

func collectActionFormFields(nodes []Node, fields map[string]map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			action, err := typed.postActionName()
			if err != nil {
				return err
			}
			if action != "" {
				if fields[action] == nil {
					fields[action] = map[string]ActionFormField{}
				}
				if err := validateActionForm(typed); err != nil {
					return err
				}
				if err := collectNamedControls(typed.Children, fields[action]); err != nil {
					return err
				}
				continue
			}
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateActionForm(element Element) error {
	for _, attr := range element.Attrs {
		if attr.Name != "enctype" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		value := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(value, "{}") {
			return fmt.Errorf("action form enctype %q must be static", value)
		}
		if strings.EqualFold(value, "multipart/form-data") {
			return fmt.Errorf("multipart action forms are not supported before upload security rules are defined")
		}
	}
	return nil
}

func collectNamedControls(nodes []Node, fields map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			if field, ok, err := controlField(typed); err != nil {
				return err
			} else if ok {
				previous := fields[field.Name]
				field.Required = field.Required || previous.Required
				fields[field.Name] = field
			}
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func controlField(element Element) (ActionFormField, bool, error) {
	switch element.Name {
	case "input", "textarea", "select":
	default:
		return ActionFormField{}, false, nil
	}
	var field ActionFormField
	inputType := ""
	for _, attr := range element.Attrs {
		if attr.Name == "required" {
			field.Required = true
			continue
		}
		if element.Name == "input" && attr.Name == "type" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				continue
			}
			inputType = strings.TrimSpace(attr.Value)
			continue
		}
		if attr.Name != "name" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		name := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(name, "{}") {
			return ActionFormField{}, false, fmt.Errorf("action form field name %q must be static", name)
		}
		field.Name = name
	}
	if field.Name == "" {
		return ActionFormField{}, false, nil
	}
	if strings.ContainsAny(inputType, "{}") {
		return ActionFormField{}, false, fmt.Errorf("action form input %q type %q must be static", field.Name, inputType)
	}
	if strings.EqualFold(inputType, "file") {
		return ActionFormField{}, false, fmt.Errorf("file input %q is not supported before upload security rules are defined", field.Name)
	}
	return field, true, nil
}

func sortedKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isStaticAssetReference(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "{}") || strings.HasPrefix(value, "#") {
		return false
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"http://", "https://", "//", "mailto:", "tel:", "data:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

type renderContext struct {
	components   map[string]Component
	values       map[string]string
	tainted      map[string]bool
	actions      map[string]string
	stack        map[string]bool
	slotHTML     string
	stateFields  map[string]bool
	readFields   map[string]bool
	bindFields   map[string]bool
	conditional  *conditionalRender
	handlers     map[string]clientlang.Handler
	stateTypes   map[string]clientlang.ValueType
	refs         map[string]clientlang.Ref
	emits        map[string]clientlang.Emit
	loopSeq      *int
	bindingSeq   *int
	islandSeq    *int
	loopItem     *loopItemRender
	templateLoop *templateLoopRender
	selectBound  bool
	selectValue  string
}

type loopItemRender struct {
	Group    string
	KeyExpr  string
	KeyValue string
}

type templateLoopRender struct{}

type parser struct {
	source []rune
	index  int
}

func (parser *parser) nodes(until string) ([]Node, error) {
	var nodes []Node
	for {
		if parser.done() {
			if until != "" {
				return nil, parser.errorf("missing closing tag </%s>", until)
			}
			return nodes, nil
		}
		if until != "" && parser.startsWith("</") {
			name, err := parser.closeTag()
			if err != nil {
				return nil, err
			}
			if name != until {
				return nil, parser.errorf("expected closing tag </%s>, got </%s>", until, name)
			}
			return nodes, nil
		}
		if parser.startsWith("</") {
			return nil, parser.errorf("unexpected closing tag")
		}
		if parser.peek() == '<' {
			node, err := parser.element()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}
		if text := parser.text(); strings.TrimSpace(text) != "" {
			nodes = append(nodes, Text{Value: text})
		}
	}
}

func (parser *parser) element() (Node, error) {
	if !parser.consume("<") {
		return nil, parser.errorf("expected element")
	}
	name, err := parser.name()
	if err != nil {
		return nil, err
	}
	if isComponentName(name) {
		return parser.componentCall(name)
	}
	if !isLowerHTMLName(name) {
		return nil, parser.errorf("unsupported element <%s>; this build slice supports lowercase HTML tags only", name)
	}

	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			return Element{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
			attrs, err := normalizeHTMLAttrs(attrs)
			if err != nil {
				return nil, err
			}
			children, err := parser.nodes(name)
			if err != nil {
				return nil, err
			}
			return Element{Name: name, Attrs: attrs, Children: children}, nil
		case parser.done():
			return nil, parser.errorf("unterminated <%s> tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) componentCall(name string) (ComponentCall, error) {
	var attrs []Attr
	for {
		parser.skipSpace()
		switch {
		case parser.consume("/>"):
			return ComponentCall{Name: name, Attrs: attrs}, nil
		case parser.consume(">"):
			children, err := parser.nodes(name)
			if err != nil {
				return ComponentCall{}, err
			}
			return ComponentCall{Name: name, Attrs: attrs, Children: children}, nil
		case parser.done():
			return ComponentCall{}, parser.errorf("unterminated <%s> component tag", name)
		default:
			attr, err := parser.attr()
			if err != nil {
				return ComponentCall{}, err
			}
			attrs = append(attrs, attr)
		}
	}
}

func (parser *parser) attr() (Attr, error) {
	if attr, ok, err := parser.shorthandAttr(); ok || err != nil {
		return attr, err
	}
	name, err := parser.attrName()
	if err != nil {
		return Attr{}, err
	}
	if !isAttrName(name) {
		return Attr{}, parser.errorf("unsupported attribute name %q", name)
	}

	parser.skipSpace()
	if !parser.consume("=") {
		return Attr{Name: name, Boolean: true}, nil
	}
	parser.skipSpace()
	if strings.HasPrefix(name, "g:") {
		return parser.directiveAttr(name)
	}
	if parser.startsWith("{") {
		value, err := parser.expressionAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value, Expression: true}, nil
	}
	value, err := parser.quotedAttrValue(name)
	if err != nil {
		return Attr{}, err
	}
	return Attr{Name: name, Value: value}, nil
}

func (parser *parser) attrName() (string, error) {
	if parser.done() || !isNameStart(parser.peek()) {
		return "", parser.errorf("expected attribute name")
	}
	start := parser.index
	parser.advance()
	for !parser.done() && isAttrNamePart(parser.peek()) {
		parser.advance()
	}
	return string(parser.source[start:parser.index]), nil
}

func (parser *parser) expressionAttrValue(name string) (string, error) {
	expr, err := parser.bracedAttrExpression(name)
	if err != nil {
		return "", err
	}
	if expr == "" {
		return "", parser.errorf("empty expression attribute %q", name)
	}
	return "{" + expr + "}", nil
}

func (parser *parser) shorthandAttr() (Attr, bool, error) {
	if parser.done() {
		return Attr{}, false, nil
	}
	prefix := parser.peek()
	if prefix != '.' && prefix != '#' {
		return Attr{}, false, nil
	}
	parser.advance()
	start := parser.index
	for !parser.done() && isShorthandPart(parser.peek()) {
		parser.advance()
	}
	if start == parser.index {
		return Attr{}, true, parser.errorf("empty shorthand attribute")
	}
	value := string(parser.source[start:parser.index])
	switch prefix {
	case '.':
		return Attr{Name: "class", Value: value}, true, nil
	case '#':
		return Attr{Name: "id", Value: value}, true, nil
	default:
		return Attr{}, false, nil
	}
}

func normalizeHTMLAttrs(attrs []Attr) ([]Attr, error) {
	var classValues []string
	var out []Attr
	id := ""
	for _, attr := range attrs {
		switch attr.Name {
		case "class":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			for _, className := range strings.Fields(attr.Value) {
				classValues = append(classValues, className)
			}
		case "id":
			if attr.Boolean {
				out = append(out, attr)
				continue
			}
			if id != "" {
				return nil, fmt.Errorf("element declares multiple id attributes")
			}
			id = attr.Value
			out = append(out, attr)
		default:
			out = append(out, attr)
		}
	}
	if len(classValues) > 0 {
		out = append([]Attr{{Name: "class", Value: strings.Join(classValues, " ")}}, out...)
	}
	return out, nil
}

func (parser *parser) directiveAttr(name string) (Attr, error) {
	if parser.startsWith("{") {
		value, err := parser.bracedAttrExpression(name)
		if err != nil {
			return Attr{}, err
		}
		return Attr{Name: name, Value: value}, nil
	}
	if parser.startsWith(`"`) {
		value, err := parser.quotedAttrValue(name)
		if err != nil {
			return Attr{}, err
		}
		value = strings.TrimSpace(value)
		return Attr{Name: name, Value: value}, nil
	}
	return Attr{}, parser.errorf("directive attribute %q must use {name}", name)
}

func (parser *parser) bracedAttrExpression(name string) (string, error) {
	if !parser.consume("{") {
		return "", parser.errorf("attribute %q must use an expression value", name)
	}
	start := parser.index
	depth := 0
	inString := false
	escaped := false
	for !parser.done() {
		char := parser.peek()
		if escaped {
			escaped = false
			parser.advance()
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			parser.advance()
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			if depth == 0 {
				expr := strings.TrimSpace(string(parser.source[start:parser.index]))
				parser.advance()
				return expr, nil
			}
			depth--
		}
		parser.advance()
	}
	return "", parser.errorf("unterminated expression attribute %q", name)
}

func (parser *parser) quotedAttrValue(name string) (string, error) {
	if !parser.consume(`"`) {
		return "", parser.errorf("attribute %q must use a quoted string value", name)
	}
	start := parser.index - 1
	escaped := false
	for !parser.done() {
		switch parser.peek() {
		case '\\':
			escaped = !escaped
			parser.advance()
		case '"':
			if escaped {
				escaped = false
				parser.advance()
				continue
			}
			parser.advance()
			value, err := strconv.Unquote(string(parser.source[start:parser.index]))
			if err != nil {
				return "", parser.errorf("invalid attribute %q string: %v", name, err)
			}
			return value, nil
		default:
			escaped = false
			parser.advance()
		}
	}
	return "", parser.errorf("unterminated attribute %q", name)
}

func (parser *parser) closeTag() (string, error) {
	if !parser.consume("</") {
		return "", parser.errorf("expected closing tag")
	}
	name, err := parser.name()
	if err != nil {
		return "", err
	}
	parser.skipSpace()
	if !parser.consume(">") {
		return "", parser.errorf("expected > after closing tag")
	}
	return name, nil
}

func (parser *parser) name() (string, error) {
	if parser.done() || !isNameStart(parser.peek()) {
		return "", parser.errorf("expected name")
	}
	start := parser.index
	parser.advance()
	for !parser.done() && isNamePart(parser.peek()) {
		parser.advance()
	}
	return string(parser.source[start:parser.index]), nil
}

func (parser *parser) text() string {
	start := parser.index
	for !parser.done() && parser.peek() != '<' {
		parser.advance()
	}
	return string(parser.source[start:parser.index])
}

func (parser *parser) skipSpace() {
	for !parser.done() && unicode.IsSpace(parser.peek()) {
		parser.advance()
	}
}

func (parser *parser) consume(value string) bool {
	if !parser.startsWith(value) {
		return false
	}
	parser.index += len([]rune(value))
	return true
}

func (parser *parser) startsWith(value string) bool {
	runes := []rune(value)
	if parser.index+len(runes) > len(parser.source) {
		return false
	}
	for offset, r := range runes {
		if parser.source[parser.index+offset] != r {
			return false
		}
	}
	return true
}

func (parser *parser) done() bool {
	return parser.index >= len(parser.source)
}

func (parser *parser) peek() rune {
	return parser.source[parser.index]
}

func (parser *parser) advance() {
	parser.index++
}

func (parser *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("view parse error at offset %d: %s", parser.index, fmt.Sprintf(format, args...))
}

func isLowerHTMLName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		case i > 0 && r == '-':
		default:
			return false
		}
	}
	return true
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}

func isAttrName(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "g:on:") {
		_, err := ParseEventDirective(value)
		return err == nil
	}
	if strings.HasPrefix(value, "style:") {
		_, err := parseStyleBindingAttr(value)
		return err == nil
	}
	for i, r := range value {
		switch {
		case isNameStart(r):
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == ':' || r == '_'):
		default:
			return false
		}
	}
	return true
}

func isNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isNamePart(r rune) bool {
	return isNameStart(r) || unicode.IsDigit(r) || r == '-' || r == ':'
}

func isAttrNamePart(r rune) bool {
	return isNamePart(r) || r == '.' || r == '%' || r == '(' || r == ')'
}

func isShorthandPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':'
}

func renderText(ctx *renderContext, out *strings.Builder, value string) error {
	text, _, err := interpolateValue(ctx, value)
	if err != nil {
		return err
	}
	out.WriteString(gowhtml.Escape(text))
	return nil
}

func interpolate(ctx *renderContext, value string) (string, error) {
	resolved, _, err := interpolateValue(ctx, value)
	return resolved, err
}

func interpolateValue(ctx *renderContext, value string) (string, bool, error) {
	if !strings.Contains(value, "{") {
		return value, false, nil
	}
	var out strings.Builder
	tainted := false
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			out.WriteString(value)
			return out.String(), tainted, nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", false, fmt.Errorf("unterminated interpolation")
		}
		end += start
		out.WriteString(value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if name == "" {
			return "", false, fmt.Errorf("empty interpolation")
		}
		if ctx.templateLoop != nil {
			out.WriteString(loopTemplateValue(name))
			value = value[end+1:]
			continue
		}
		if param, ok := routeParamExpression(name); ok {
			resolved, ok := ctx.values[param]
			if !ok {
				return "", false, fmt.Errorf("unknown route param %q", param)
			}
			tainted = true
			out.WriteString(resolved)
			value = value[end+1:]
			continue
		}
		resolved, ok := ctx.values[name]
		if !ok {
			return "", false, fmt.Errorf("unknown interpolation %q", name)
		}
		if ctx.tainted[name] {
			tainted = true
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
}

func unsafeRouteParamAttr(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "on") && len(name) > 2 {
		return true
	}
	switch name {
	case "style", "srcdoc":
		return true
	case "href", "src", "srcset", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href":
		return true
	default:
		return false
	}
}

func routeParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !isIdentifier(name) {
		return "", false
	}
	return name, true
}

// ValidateIslandEventExpression validates the first generated-JS event
// expression subset. When fields is non-nil, every referenced field must exist.
// Named client function calls are valid only when handlers declares that
// function.
func ValidateIslandEventExpression(expr string, fields map[string]bool, handlers ...map[string]clientlang.Handler) error {
	symbols := boolFieldSymbols(fields)
	return ValidateIslandEventExpressionTyped(expr, symbols, symbols, firstHandlerMap(handlers))
}

// ValidateIslandBoolExpression validates a g:if-style bool expression.
func ValidateIslandBoolExpression(expr string, fields map[string]bool) error {
	symbols := boolFieldSymbols(fields)
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("expression must be bool, got %s", typ)
	}
	return nil
}

// ValidateIslandBoolExpressionTyped validates a g:if-style bool expression
// with scalar type information.
func ValidateIslandBoolExpressionTyped(expr string, symbols map[string]clientlang.ValueType) error {
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("expression must be bool, got %s", typ)
	}
	return nil
}

// ValidateReactiveAttrExpressionTyped validates a first-slice reactive
// attribute expression.
func ValidateReactiveAttrExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if unsafeReactiveAttr(name) {
		return fmt.Errorf("reactive attribute %q is not supported before safe URL/style/event rules are defined", name)
	}
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if isBooleanHTMLAttr(name) && typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("boolean attribute %q requires bool expression, got %s", name, typ)
	}
	return nil
}

// ValidateClassToggleExpressionTyped validates a class:name directive
// expression.
func ValidateClassToggleExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if classToggleName(name) == "" {
		return fmt.Errorf("class toggle directive %q requires a class name", name)
	}
	return ValidateIslandBoolExpressionTyped(expr, symbols)
}

// ValidateStyleBindingExpressionTyped validates a style:name directive
// expression.
func ValidateStyleBindingExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if _, err := parseStyleBindingAttr(name); err != nil {
		return err
	}
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ == clientlang.TypeBool {
		return fmt.Errorf("style binding requires string or numeric expression, got %s", typ)
	}
	if typ == clientlang.TypeObject || typ == clientlang.TypeArray {
		return fmt.Errorf("style binding requires string or numeric expression, got %s", typ)
	}
	return nil
}

// ValidateIslandEventExpressionTyped validates an event expression with scalar
// type information.
func ValidateIslandEventExpressionTyped(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler) error {
	return ValidateIslandEventExpressionTypedWithFunctions(expr, readSymbols, writeSymbols, handlers, nil)
}

// ValidateIslandEventExpressionTypedWithFunctions validates an event expression
// with scalar type information and return-valued helper functions.
func ValidateIslandEventExpressionTypedWithFunctions(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) error {
	return ValidateIslandEventExpressionTypedWithEvents(expr, readSymbols, writeSymbols, handlers, helpers, nil)
}

// ValidateIslandEventExpressionTypedWithEvents validates an event expression,
// including component event dispatch statements.
func ValidateIslandEventExpressionTypedWithEvents(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if emit, ok := clientlang.ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	if call, ok := clientlang.ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, helpers)
		}
		if handlers == nil {
			return fmt.Errorf("unknown island client function %q", call.Name)
		}
		handler, exists := handlers[call.Name]
		if !exists {
			return fmt.Errorf("unknown island client function %q", call.Name)
		}
		if len(call.Args) != len(handler.Params) {
			return fmt.Errorf("island client function %s expects %d arguments, got %d", call.Name, len(handler.Params), len(call.Args))
		}
		for index, arg := range call.Args {
			typ, _, err := clientlang.CheckExprWithFunctions(arg, readSymbols, helpers)
			if err != nil {
				return err
			}
			expected := handlerParamType(handler, index)
			if expected != clientlang.TypeUnknown && typ != expected && !compatibleNumericType(typ, expected) {
				return fmt.Errorf("island client function %s argument %d expects %s, got %s", call.Name, index+1, expected, typ)
			}
		}
		return nil
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

func validateEmitCall(call clientlang.EmitCall, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	if emits == nil {
		return fmt.Errorf("unknown component event %q", call.Name)
	}
	event, exists := emits[call.Name]
	if !exists {
		return fmt.Errorf("unknown component event %q", call.Name)
	}
	if len(call.Args) != len(event.Params) {
		return fmt.Errorf("component event %s expects %d arguments, got %d", call.Name, len(event.Params), len(call.Args))
	}
	for index, arg := range call.Args {
		typ, _, err := clientlang.CheckExprWithFunctions(arg, readSymbols, helpers)
		if err != nil {
			return err
		}
		expected := clientlang.TypeUnknown
		if index < len(event.ParamTypes) {
			expected = event.ParamTypes[index]
		}
		if expected != clientlang.TypeUnknown && typ != expected && !compatibleNumericType(typ, expected) {
			return fmt.Errorf("component event %s argument %d expects %s, got %s", call.Name, index+1, expected, typ)
		}
	}
	return nil
}

// ValidateIslandStateStatement validates a client statement that may write only
// writeFields and may read readFields plus scalar literals.
func ValidateIslandStateStatement(expr string, writeFields map[string]bool, readFields map[string]bool) error {
	return ValidateIslandStateStatementTyped(expr, boolFieldSymbols(writeFields), boolFieldSymbols(readFields))
}

// ValidateIslandStateStatementTyped validates a state statement with scalar
// type information.
func ValidateIslandStateStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType) error {
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, nil)
}

// ValidateIslandStateStatementTypedWithFunctions validates a state statement
// with scalar type information and return-valued helper functions.
func ValidateIslandStateStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if call, ok := clientlang.ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, helpers)
		}
		return fmt.Errorf("unsupported island event expression %q", expr)
	}
	if match := islandIncDecPattern.FindStringSubmatch(expr); match != nil {
		typ, err := validateIslandSymbol(match[1], writeSymbols)
		if err != nil {
			return err
		}
		if !compatibleNumericType(typ, clientlang.TypeInt) {
			return fmt.Errorf("operator %s requires numeric island field %q", match[2], match[1])
		}
		return nil
	}
	if islandFieldPattern.MatchString(expr) {
		_, err := validateIslandSymbol(expr, readSymbols)
		return err
	}
	if match := islandAssignPattern.FindStringSubmatch(expr); match != nil {
		left := strings.TrimSpace(match[1])
		leftType, err := validateIslandSymbol(left, writeSymbols)
		if err != nil {
			return err
		}
		if leftType == clientlang.TypeObject || leftType == clientlang.TypeArray {
			return fmt.Errorf("cannot assign to non-scalar island field %q", left)
		}
		right := strings.TrimSpace(match[2])
		rightType, _, err := clientlang.CheckExprWithFunctions(right, readSymbols, helpers)
		if err != nil {
			return err
		}
		if rightType == clientlang.TypeObject || rightType == clientlang.TypeArray {
			return fmt.Errorf("cannot assign %s expression to island field %q", rightType, left)
		}
		if leftType != clientlang.TypeUnknown && rightType != leftType && !compatibleNumericType(rightType, leftType) {
			return fmt.Errorf("cannot assign %s expression to %s field %q", rightType, leftType, left)
		}
		return nil
	}
	return fmt.Errorf("unsupported island event expression %q", expr)
}

// ValidateIslandClientStatementTyped validates a client statement that may
// mutate state or call a safe DOM ref method.
func ValidateIslandClientStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) error {
	return ValidateIslandClientStatementTypedWithFunctions(expr, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementTypedWithFunctions validates a client statement
// that may mutate state, call a safe DOM ref method, or read helper functions.
func ValidateIslandClientStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) error {
	return ValidateIslandClientStatementTypedWithEvents(expr, writeSymbols, readSymbols, refs, helpers, nil)
}

// ValidateIslandClientStatementTypedWithEvents validates a client statement
// that may mutate state, call a safe DOM ref method, read helper functions, or
// dispatch declared component events.
func ValidateIslandClientStatementTypedWithEvents(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	if refName, ok := IslandRefStatement(expr); ok {
		if refs == nil {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		if _, exists := refs[refName]; !exists {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		return nil
	}
	if emit, ok := clientlang.ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

// ValidateIslandClientStatementsTyped validates an ordered client statement
// block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTyped(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithFunctions(statements, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementsTypedWithFunctions validates an ordered client
// statement block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTypedWithFunctions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithOptions(statements, writeSymbols, readSymbols, refs, helpers, false)
}

// ValidateIslandClientStatementsTypedWithOptions validates an ordered client
// statement block with the same local-variable rules as
// ValidateIslandClientStatementsTypedWithFunctions. Async blocks may use
// compiler-owned await expressions.
func ValidateIslandClientStatementsTypedWithOptions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithEvents(statements, writeSymbols, readSymbols, refs, helpers, async, nil)
}

// ValidateIslandClientStatementsTypedWithEvents validates an ordered client
// statement block with optional component event dispatch support.
func ValidateIslandClientStatementsTypedWithEvents(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool, emits map[string]clientlang.Emit) (map[string]bool, error) {
	locals := mergeClientSymbols(nil, readSymbols)
	usedRefs := map[string]bool{}
	for index, statement := range statements {
		if refName, ok := IslandRefStatement(statement); ok {
			usedRefs[refName] = true
		}
		if local, ok, err := parseLetStatement(statement); err != nil {
			return usedRefs, StatementValidationError{Index: index, Err: err}
		} else if ok {
			if strings.Contains(local.Expr, "await ") {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("await is not supported in let statements")}
			}
			if _, exists := writeSymbols[local.Name]; exists {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q conflicts with a state field", local.Name)}
			}
			if _, exists := locals[local.Name]; exists {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q is already declared", local.Name)}
			}
			typ := clientlang.NormalizeType(local.Type)
			if !isSupportedLocalType(typ) {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q uses unsupported type %q", local.Name, local.Type)}
			}
			actual, _, err := clientlang.CheckExprWithFunctions(local.Expr, locals, helpers)
			if err != nil {
				return usedRefs, StatementValidationError{Index: index, Err: err}
			}
			if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q cannot use %s expression", local.Name, actual)}
			}
			if typ != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && typ != actual && !compatibleNumericType(actual, typ) {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q expects %s, got %s", local.Name, typ, actual)}
			}
			locals[local.Name] = typ
			continue
		}
		if strings.Contains(statement, "await ") {
			if !async {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("await is only supported inside async client functions")}
			}
			if err := validateAwaitFetchAssignment(statement, writeSymbols, locals, helpers); err != nil {
				return usedRefs, StatementValidationError{Index: index, Err: err}
			}
			continue
		}
		if err := ValidateIslandClientStatementTypedWithEvents(statement, writeSymbols, locals, refs, helpers, emits); err != nil {
			return usedRefs, StatementValidationError{Index: index, Err: err}
		}
	}
	return usedRefs, nil
}

// StatementValidationError identifies the statement index that failed within a
// client statement block.
type StatementValidationError struct {
	Index int
	Err   error
}

func (err StatementValidationError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err StatementValidationError) Unwrap() error {
	return err.Err
}

func validateAwaitFetchAssignment(statement string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	match := islandAssignPattern.FindStringSubmatch(strings.TrimSpace(statement))
	if match == nil {
		return fmt.Errorf("await fetchJSON must assign to a state field")
	}
	left := strings.TrimSpace(match[1])
	leftType, err := validateIslandSymbol(left, writeSymbols)
	if err != nil {
		return err
	}
	right := strings.TrimSpace(match[2])
	fetch := islandAwaitFetchPattern.FindStringSubmatch(right)
	if fetch == nil {
		return fmt.Errorf("await supports only fetchJSON[T](urlExpr)")
	}
	fetchType := clientlang.NormalizeType(strings.TrimSpace(fetch[1]))
	if fetchType != clientlang.TypeUnknown && leftType != clientlang.TypeUnknown && fetchType != leftType && !compatibleNumericType(fetchType, leftType) {
		return fmt.Errorf("cannot assign fetched %s value to %s field %q", fetchType, leftType, left)
	}
	urlType, _, err := clientlang.CheckExprWithFunctions(strings.TrimSpace(fetch[2]), readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("fetchJSON url: %w", err)
	}
	if urlType != clientlang.TypeString && urlType != clientlang.TypeUnknown {
		return fmt.Errorf("fetchJSON url must be string, got %s", urlType)
	}
	return nil
}

type letStatement struct {
	Name string
	Type string
	Expr string
}

func parseLetStatement(statement string) (letStatement, bool, error) {
	match := islandLetPattern.FindStringSubmatch(strings.TrimSpace(statement))
	if match == nil {
		if strings.HasPrefix(strings.TrimSpace(statement), "let ") {
			return letStatement{}, false, fmt.Errorf("let statement must use `let name type = expr`")
		}
		return letStatement{}, false, nil
	}
	return letStatement{Name: match[1], Type: match[2], Expr: strings.TrimSpace(match[3])}, true, nil
}

func isSupportedLocalType(typ clientlang.ValueType) bool {
	switch typ {
	case clientlang.TypeString, clientlang.TypeInt, clientlang.TypeFloat, clientlang.TypeBool:
		return true
	default:
		return false
	}
}

func mergeClientSymbols(left, right map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	output := map[string]clientlang.ValueType{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}

// IslandRefStatement reports whether expr is a safe DOM ref method call.
func IslandRefStatement(expr string) (string, bool) {
	match := islandRefCallPattern.FindStringSubmatch(strings.TrimSpace(expr))
	if match == nil {
		return "", false
	}
	return match[1], true
}

// IslandExpressionFields returns field references in a supported island event
// expression.
func IslandExpressionFields(expr string) []string {
	expr = strings.TrimSpace(expr)
	if call, ok := clientlang.ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return arrayMutationFields(call)
		}
		return islandCallFields(call)
	}
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" {
			seen[name] = true
		}
	}
	if match := islandIncDecPattern.FindStringSubmatch(expr); match != nil {
		add(match[1])
		return sortedKeys(seen)
	}
	if islandFieldPattern.MatchString(expr) {
		add(expr)
		return sortedKeys(seen)
	}
	if match := islandAssignPattern.FindStringSubmatch(expr); match != nil {
		add(strings.TrimSpace(match[1]))
		right := strings.TrimSpace(match[2])
		if fields, err := clientlang.ExprFields(right); err == nil {
			for _, field := range fields {
				add(field)
			}
		}
	}
	return sortedKeys(seen)
}

func isArrayMutationCall(name string) bool {
	switch name {
	case "append", "remove", "move":
		return true
	default:
		return false
	}
}

func validateArrayMutationCall(call clientlang.Call, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType) error {
	return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, nil)
}

func validateArrayMutationCallWithFunctions(call clientlang.Call, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	switch call.Name {
	case "append":
		if len(call.Args) != 2 {
			return fmt.Errorf("append expects 2 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("append target %q must be array, got %s", field, typ)
		}
		itemFields, err := parseObjectLiteral(call.Args[1])
		if err != nil {
			return fmt.Errorf("append item: %w", err)
		}
		itemSymbols := itemFieldSymbols(field, readSymbols)
		for name, expr := range itemFields {
			expected, ok := itemSymbols[name]
			if !ok {
				return fmt.Errorf("append item has unknown field %q", name)
			}
			actual, _, err := clientlang.CheckExprWithFunctions(expr, readSymbols, helpers)
			if err != nil {
				return fmt.Errorf("append item field %s: %w", name, err)
			}
			if expected == clientlang.TypeArray || expected == clientlang.TypeObject {
				return fmt.Errorf("append item field %s must be scalar", name)
			}
			if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
				return fmt.Errorf("append item field %s cannot use %s expression", name, actual)
			}
			if expected != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && expected != actual && !compatibleNumericType(actual, expected) {
				return fmt.Errorf("append item field %s expects %s, got %s", name, expected, actual)
			}
		}
		return nil
	case "remove":
		if len(call.Args) != 2 {
			return fmt.Errorf("remove expects 2 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("remove target %q must be array, got %s", field, typ)
		}
		return validateArrayIndexExprWithFunctions("remove", call.Args[1], readSymbols, helpers)
	case "move":
		if len(call.Args) != 3 {
			return fmt.Errorf("move expects 3 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("move target %q must be array, got %s", field, typ)
		}
		if err := validateArrayIndexExprWithFunctions("move", call.Args[1], readSymbols, helpers); err != nil {
			return err
		}
		return validateArrayIndexExprWithFunctions("move", call.Args[2], readSymbols, helpers)
	default:
		return fmt.Errorf("unsupported array mutation %q", call.Name)
	}
}

func validateArrayIndexExpr(name, expr string, readSymbols map[string]clientlang.ValueType) error {
	return validateArrayIndexExprWithFunctions(name, expr, readSymbols, nil)
}

func validateArrayIndexExprWithFunctions(name, expr string, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	typ, _, err := clientlang.CheckExprWithFunctions(expr, readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("%s index: %w", name, err)
	}
	if typ != clientlang.TypeInt && typ != clientlang.TypeUnknown {
		return fmt.Errorf("%s index must be int, got %s", name, typ)
	}
	return nil
}

func itemFieldSymbols(arrayField string, symbols map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	out := map[string]clientlang.ValueType{}
	prefix := arrayField + "[]."
	for name, typ := range symbols {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = typ
		}
	}
	return out
}

func parseObjectLiteral(source string) (map[string]string, error) {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, "{") || !strings.HasSuffix(source, "}") {
		return nil, fmt.Errorf("must use { Field: expr }")
	}
	body := strings.TrimSpace(source[1 : len(source)-1])
	if body == "" {
		return nil, fmt.Errorf("must declare at least one field")
	}
	parts, err := splitTopLevelComma(body)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, part := range parts {
		name, expr, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("field %q must use name: expr", part)
		}
		name = strings.TrimSpace(name)
		expr = strings.TrimSpace(expr)
		if !isIdentifier(name) {
			return nil, fmt.Errorf("invalid field name %q", name)
		}
		if expr == "" {
			return nil, fmt.Errorf("field %s has empty expression", name)
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate field %q", name)
		}
		fields[name] = expr
	}
	return fields, nil
}

func splitTopLevelComma(source string) ([]string, error) {
	var parts []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced expression")
			}
		case ',':
			if depth > 0 {
				continue
			}
			part := strings.TrimSpace(source[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty item")
			}
			parts = append(parts, part)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced expression")
	}
	part := strings.TrimSpace(source[start:])
	if part == "" {
		return nil, fmt.Errorf("empty item")
	}
	return append(parts, part), nil
}

func arrayMutationFields(call clientlang.Call) []string {
	seen := map[string]bool{}
	if len(call.Args) > 0 {
		field := strings.TrimSpace(call.Args[0])
		if field != "" {
			seen[field] = true
		}
	}
	for _, arg := range call.Args[1:] {
		if objectFields, err := parseObjectLiteral(arg); err == nil {
			for _, expr := range objectFields {
				if fields, err := clientlang.ExprFields(expr); err == nil {
					for _, field := range fields {
						seen[field] = true
					}
				}
			}
			continue
		}
		if fields, err := clientlang.ExprFields(arg); err == nil {
			for _, field := range fields {
				seen[field] = true
			}
		}
	}
	return sortedKeys(seen)
}

func validateIslandField(field string, fields map[string]bool) error {
	if !islandFieldPattern.MatchString(field) {
		return fmt.Errorf("invalid island field %q", field)
	}
	if fields != nil && !fields[field] {
		return fmt.Errorf("unknown island field %q", field)
	}
	return nil
}

func validateIslandReadableValue(value string, fields map[string]bool) error {
	value = strings.TrimSpace(value)
	if isIslandScalarLiteral(value) {
		return nil
	}
	if islandFieldPattern.MatchString(value) {
		return validateIslandField(value, fields)
	}
	return fmt.Errorf("unsupported island value %q", value)
}

func validateIslandSymbol(field string, symbols map[string]clientlang.ValueType) (clientlang.ValueType, error) {
	if !islandFieldPattern.MatchString(field) {
		return clientlang.TypeUnknown, fmt.Errorf("invalid island field %q", field)
	}
	typ, ok := symbols[field]
	if symbols != nil && !ok {
		return clientlang.TypeUnknown, fmt.Errorf("unknown island field %q", field)
	}
	return typ, nil
}

func boolFieldSymbols(fields map[string]bool) map[string]clientlang.ValueType {
	if fields == nil {
		return nil
	}
	symbols := map[string]clientlang.ValueType{}
	for field, ok := range fields {
		if ok {
			symbols[field] = clientlang.TypeUnknown
		}
	}
	return symbols
}

func firstHandlerMap(handlers []map[string]clientlang.Handler) map[string]clientlang.Handler {
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func handlerParamType(handler clientlang.Handler, index int) clientlang.ValueType {
	if index < 0 || index >= len(handler.ParamTypes) {
		return clientlang.TypeUnknown
	}
	return handler.ParamTypes[index]
}

func compatibleNumericType(actual, expected clientlang.ValueType) bool {
	if actual == clientlang.TypeUnknown || expected == clientlang.TypeUnknown {
		return true
	}
	return (actual == clientlang.TypeInt || actual == clientlang.TypeFloat) &&
		(expected == clientlang.TypeInt || expected == clientlang.TypeFloat)
}

func isIslandScalarLiteral(value string) bool {
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if islandNumberPattern.MatchString(value) {
		return true
	}
	if strings.HasPrefix(value, `"`) {
		_, err := strconv.Unquote(value)
		return err == nil
	}
	return false
}

func islandCallFields(call clientlang.Call) []string {
	seen := map[string]bool{}
	for _, arg := range call.Args {
		arg = strings.TrimSpace(arg)
		if islandFieldPattern.MatchString(arg) && !isIslandScalarLiteral(arg) {
			seen[arg] = true
		}
	}
	return sortedKeys(seen)
}

func islandTextBinding(value string) (string, bool) {
	match := islandTextBindingPattern.FindStringSubmatch(value)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char == '_':
		case index > 0 && char >= '0' && char <= '9':
		default:
			return false
		}
	}
	return true
}

func cloneStack(input map[string]bool) map[string]bool {
	output := map[string]bool{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneValues(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeValues(base map[string]string, overlay map[string]string) map[string]string {
	out := cloneValues(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func evalComputedValues(computeds []clientlang.Computed, values map[string]string) (map[string]string, map[string]any, error) {
	if len(computeds) == 0 {
		return nil, nil, nil
	}
	stringsOut := map[string]string{}
	valuesOut := map[string]any{}
	scope := cloneValues(values)
	for _, computed := range computeds {
		value, err := clientlang.EvalValue(computed.Expr, scope)
		if err != nil {
			return nil, nil, fmt.Errorf("computed %s: %w", computed.Name, err)
		}
		scalar, ok := scalarString(value)
		if !ok {
			return nil, nil, fmt.Errorf("computed %s must evaluate to a scalar value", computed.Name)
		}
		stringsOut[computed.Name] = scalar
		valuesOut[computed.Name] = value
		scope[computed.Name] = scalar
	}
	return stringsOut, valuesOut, nil
}

func componentStateJSON(stateJSON string, props map[string]string, computed map[string]any) string {
	if stateJSON == "" && len(props) == 0 && len(computed) == 0 {
		return ""
	}
	values := map[string]any{}
	if stateJSON != "" {
		_ = json.Unmarshal([]byte(stateJSON), &values)
	}
	for key, value := range props {
		values[key] = value
	}
	for key, value := range computed {
		values[key] = value
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return stateJSON
	}
	return string(payload)
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case json.Number:
		return typed.String(), true
	default:
		return "", false
	}
}

func keys(input map[string]string) []string {
	out := make([]string, 0, len(input))
	for key := range input {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func keysFromTypes(input map[string]clientlang.ValueType) []string {
	out := make([]string, 0, len(input))
	for key := range input {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func boolSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func stripLineComments(source string) string {
	var lines []string
	for _, rawLine := range strings.Split(source, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, rawLine)
	}
	return strings.Join(lines, "\n")
}
