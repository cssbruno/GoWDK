package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
	"strconv"
	"strings"
)

type Element struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

func (node Element) render(ctx *renderContext, out *strings.Builder) error {
	if node.Name == "slot" {
		return node.renderSlot(ctx, out)
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
	if len(ctx.scopeIDs) > 0 {
		out.WriteString(` data-gowdk-scope="`)
		out.WriteString(gowhtml.Escape(strings.Join(ctx.scopeIDs, " ")))
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
			readSymbols := mergeClientSymbols(ctx.readSymbols(), domEventSymbols())
			if err := ValidateIslandEventExpressionTypedWithEvents(attr.Value, readSymbols, ctx.stateTypes, ctx.handlers, nil, ctx.emits); err != nil {
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
		if valueBinding != "" && node.SPAInputType("radio") && attr.Name == "checked" {
			continue
		}
		if isClassToggleAttr(attr.Name) {
			continue
		}
		if isStyleBindingAttr(attr.Name) {
			continue
		}
		if valueBinding != "" && attr.Name == "value" && !node.SPAInputType("radio") {
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
	if directives.Command != "" {
		out.WriteString(` data-gowdk-command="`)
		out.WriteString(gowhtml.Escape(directives.Command))
		out.WriteByte('"')
	}
	if directives.Query != "" {
		out.WriteString(` data-gowdk-query="`)
		out.WriteString(gowhtml.Escape(directives.Query))
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

// DOMEventSymbols returns the compiler-owned scalar DOM event scope exposed to
// g:on:* expressions.
func DOMEventSymbols() map[string]clientlang.ValueType {
	return map[string]clientlang.ValueType{
		"event":         clientlang.TypeObject,
		"event.value":   clientlang.TypeString,
		"event.checked": clientlang.TypeBool,
		"event.key":     clientlang.TypeString,
		"event.code":    clientlang.TypeString,
		"event.clientX": clientlang.TypeFloat,
		"event.clientY": clientlang.TypeFloat,
	}
}

func domEventSymbols() map[string]clientlang.ValueType {
	return DOMEventSymbols()
}

func (node Element) renderSlot(ctx *renderContext, out *strings.Builder) error {
	name := ""
	scopedValues := map[string]string{}
	for _, attr := range node.Attrs {
		if attr.Name == "name" {
			if attr.Boolean {
				return fmt.Errorf("slot name requires a value")
			}
			name = strings.TrimSpace(attr.Value)
			continue
		}
		if strings.HasPrefix(attr.Name, "g:") {
			return fmt.Errorf("slot uses unsupported directive attribute %q", attr.Name)
		}
		if attr.Boolean {
			return fmt.Errorf("slot prop %q requires a value", attr.Name)
		}
		value, _, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return err
		}
		scopedValues[attr.Name] = value
	}
	if slot, ok := ctx.slots[name]; ok {
		slotValues := map[string]string{}
		for key, value := range scopedValues {
			slotValues[key] = value
		}
		for prop, local := range slot.Lets {
			if value, ok := scopedValues[prop]; ok {
				slotValues[local] = value
			}
		}
		html, err := renderSlotContent(slot, slotValues)
		if err != nil {
			return err
		}
		out.WriteString(html)
		return nil
	}
	if name == "" && ctx.slotHTML != "" {
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
	if field == "" || node.Name != "input" || !node.SPAInputType("radio") {
		return false, nil
	}
	value, ok, err := node.SPAAttrInterpolated(ctx, "value")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("g:bind:value on radio <input> requires a literal value attribute")
	}
	return value == ctx.values[field], nil
}

func (node Element) SPAAttrInterpolated(ctx *renderContext, name string) (string, bool, error) {
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
