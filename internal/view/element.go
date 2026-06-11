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
	Start    int
	End      int
}

func (node Element) render(ctx *renderContext, out *renderOutput) error {
	if node.Name == "slot" {
		return node.renderSlot(ctx, out)
	}
	if loop, keyExpr, ok, err := node.forDirective(ctx); err != nil {
		return err
	} else if ok {
		return node.renderFor(ctx, out, loop, keyExpr)
	}
	if ctx.conditional != nil {
		out.write(`<!--gowdk-if:`)
		out.write(gowhtml.Escape(ctx.conditional.Marker()))
		out.write(`:start-->`)
	}
	out.writeByte('<')
	out.write(node.Name)
	directives, err := node.postDirectives(ctx)
	if err != nil {
		return err
	}
	valueBinding, err := node.valueBinding(ctx)
	if err != nil {
		return err
	}
	rawHTML, hasRawHTML, err := node.rawHTMLContent(ctx)
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
		out.write(` data-gowdk-style-`)
		out.write(binding.Property)
		out.write(`="`)
		out.write(gowhtml.Escape(binding.Expression))
		out.write(`" data-gowdk-binding-style-`)
		out.write(binding.Property)
		out.write(`="`)
		out.write(ctx.nextBindingID())
		out.writeByte('"')
		if binding.Unit != "" {
			out.write(` data-gowdk-style-unit-`)
			out.write(binding.Property)
			out.write(`="`)
			out.write(gowhtml.Escape(binding.Unit))
			out.writeByte('"')
		}
	}
	for _, toggle := range classToggles {
		out.write(` data-gowdk-class-`)
		out.write(toggle.Name)
		out.write(`="`)
		out.write(gowhtml.Escape(toggle.Expression))
		out.write(`" data-gowdk-binding-class-`)
		out.write(toggle.Name)
		out.write(`="`)
		out.write(ctx.nextBindingID())
		out.writeByte('"')
	}
	if classValue := node.initialClassValue(ctx, classToggles); classValue != "" {
		out.write(` class="`)
		out.write(gowhtml.Escape(classValue))
		out.writeByte('"')
	}
	styleValue, err := node.initialStyleValue(ctx, styleBindings)
	if err != nil {
		return err
	}
	if styleValue != "" {
		out.write(` style="`)
		out.write(gowhtml.Escape(styleValue))
		out.writeByte('"')
	}
	if ctx.loopItem != nil {
		out.write(` data-gowdk-for-item="`)
		out.write(gowhtml.Escape(ctx.loopItem.Group))
		out.write(`" data-gowdk-key-value="`)
		out.write(gowhtml.Escape(ctx.loopKeyValue(ctx.loopItem.KeyExpr)))
		out.writeByte('"')
	}
	if len(ctx.scopeIDs) > 0 {
		out.write(` data-gowdk-scope="`)
		out.write(gowhtml.Escape(strings.Join(ctx.scopeIDs, " ")))
		out.writeByte('"')
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
			out.write(` data-gowdk-on-`)
			out.write(eventDirective.Event)
			out.write(`="`)
			out.write(gowhtml.Escape(attr.Value))
			out.write(`" data-gowdk-binding-on-`)
			out.write(eventDirective.Event)
			out.write(`="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
			if options := eventDirective.RuntimeOptions(); options != "" {
				out.write(` data-gowdk-event-`)
				out.write(eventDirective.Event)
				out.write(`="`)
				out.write(gowhtml.Escape(options))
				out.writeByte('"')
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
			out.write(` data-gowdk-if="`)
			out.write(gowhtml.Escape(attr.Value))
			out.write(`" data-gowdk-binding-if="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
			if visible, err := clientlang.EvalBool(attr.Value, ctx.values); err == nil && !visible {
				out.write(` hidden`)
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
			out.write(` data-gowdk-bind-value="`)
			out.write(gowhtml.Escape(valueBinding))
			out.write(`" data-gowdk-binding-value="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
			if bindingType := valueBindingRuntimeType(valueBinding, ctx.stateTypes); bindingType != "" {
				out.write(` data-gowdk-bind-type="`)
				out.write(gowhtml.Escape(bindingType))
				out.writeByte('"')
			}
			if node.Name == "input" {
				out.write(` value="`)
				out.write(gowhtml.Escape(ctx.values[valueBinding]))
				out.writeByte('"')
			}
			continue
		}
		if attr.Name == "g:bind:checked" {
			out.write(` data-gowdk-bind-checked="`)
			out.write(gowhtml.Escape(checkedBinding))
			out.write(`" data-gowdk-binding-checked="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
			if ctx.values[checkedBinding] == "true" {
				out.write(` checked`)
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
			out.write(` data-gowdk-ref="`)
			out.write(gowhtml.Escape(refName))
			out.write(`" data-gowdk-binding-ref="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
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
			out.write(` data-gowdk-attr-`)
			out.write(attr.Name)
			out.write(`="`)
			out.write(gowhtml.Escape(expr))
			out.write(`" data-gowdk-binding-attr-`)
			out.write(attr.Name)
			out.write(`="`)
			out.write(ctx.nextBindingID())
			out.writeByte('"')
			value, ok, err := reactiveAttrValue(attr.Name, expr, ctx.values)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			out.writeByte(' ')
			out.write(attr.Name)
			if !isBooleanHTMLAttr(attr.Name) {
				out.write(`="`)
				out.write(gowhtml.Escape(value))
				out.writeByte('"')
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
		if err := validateRenderedHTMLAttrSafety(attr.Name, value); err != nil {
			return err
		}
		out.writeByte(' ')
		out.write(attr.Name)
		if attr.Value != "" || !attr.Boolean {
			out.write(`="`)
			out.write(gowhtml.Escape(value))
			out.writeByte('"')
		}
	}
	if selected, err := node.optionSelected(ctx); err != nil {
		return err
	} else if selected {
		out.write(` selected`)
	}
	if checked, err := node.radioChecked(ctx, valueBinding); err != nil {
		return err
	} else if checked {
		out.write(` checked`)
	}
	if directives.Route != "" {
		out.write(` method="post" action="`)
		out.write(gowhtml.Escape(directives.Route))
		out.writeByte('"')
	}
	if directives.Command != "" {
		out.write(` data-gowdk-command="`)
		out.write(gowhtml.Escape(directives.Command))
		out.writeByte('"')
	}
	if directives.Query != "" {
		out.write(` data-gowdk-query="`)
		out.write(gowhtml.Escape(directives.Query))
		out.writeByte('"')
	}
	if directives.Target != "" {
		out.write(` data-gowdk-target="`)
		out.write(gowhtml.Escape(directives.Target))
		out.writeByte('"')
	}
	if directives.Swap != "" {
		out.write(` data-gowdk-swap="`)
		out.write(gowhtml.Escape(directives.Swap))
		out.writeByte('"')
	}
	if ctx.conditional != nil {
		out.write(` data-gowdk-if-group="`)
		out.write(gowhtml.Escape(ctx.conditional.Group))
		out.write(`" data-gowdk-if-index="`)
		out.write(strconv.Itoa(ctx.conditional.Index))
		out.write(`" data-gowdk-binding-if="`)
		out.write(ctx.nextBindingID())
		out.writeByte('"')
		if ctx.conditional.Condition != "" {
			out.write(` data-gowdk-if="`)
			out.write(gowhtml.Escape(ctx.conditional.Condition))
			out.writeByte('"')
		} else {
			out.write(` data-gowdk-else`)
		}
		if !ctx.conditional.Visible {
			out.write(` hidden`)
		}
	}
	out.writeByte('>')
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
	if hasRawHTML {
		out.write(rawHTML)
	} else if node.Name == "textarea" && valueBinding != "" {
		out.write(gowhtml.Escape(ctx.values[valueBinding]))
	} else {
		for _, child := range node.Children {
			if err := child.render(childCtx, out); err != nil {
				return err
			}
		}
	}
	if !voidElements[node.Name] {
		out.write("</")
		out.write(node.Name)
		out.writeByte('>')
	}
	if ctx.conditional != nil {
		out.write(`<!--gowdk-if:`)
		out.write(gowhtml.Escape(ctx.conditional.Marker()))
		out.write(`:end-->`)
	}
	return nil
}

// rawHTMLContent evaluates the explicit g:html raw HTML escape hatch for one
// element. The expression resolves through the same render-data lookup as text
// interpolation, and the resulting string is written without escaping. Raw
// HTML is rejected inside stateful component views and g:for loops because the
// island runtime re-renders bound content as escaped text and cannot honor
// raw HTML.
func (node Element) rawHTMLContent(ctx *renderContext) (string, bool, error) {
	expr := ""
	for _, attr := range node.Attrs {
		if attr.Name == "g:html" {
			expr = strings.TrimSpace(attr.Value)
		}
	}
	if expr == "" {
		return "", false, nil
	}
	if ctx.templateLoop != nil || ctx.loopItem != nil {
		return "", false, fmt.Errorf("g:html is not supported inside g:for loops; the island loop runtime re-renders rows as escaped text")
	}
	if len(ctx.stateFields) > 0 || len(ctx.stateTypes) > 0 || len(ctx.handlers) > 0 || len(ctx.emits) > 0 {
		return "", false, fmt.Errorf("g:html is not supported inside stateful component views; the island runtime re-renders bound content as escaped text and cannot honor raw HTML")
	}
	if ctx.bindFields[expr] {
		return "", false, fmt.Errorf("g:html cannot reference reactive field %q; the island runtime re-renders bound content as escaped text", expr)
	}
	value, tainted, err := interpolateValue(ctx, "{"+expr+"}")
	if err != nil {
		return "", false, fmt.Errorf("g:html: %w", err)
	}
	if tainted {
		return "", false, fmt.Errorf("route param interpolation is not allowed in g:html")
	}
	return value, true, nil
}

// voidElements are HTML elements that have no end tag; emitting one (for
// example </br>) makes browsers treat it as a second start tag.
var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"source": true,
	"track":  true,
	"wbr":    true,
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

func (node Element) renderSlot(ctx *renderContext, out *renderOutput) error {
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
		out.write(html)
		return nil
	}
	if name == "" && ctx.slotHTML != "" {
		out.write(ctx.slotHTML)
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
	var text renderOutput
	for _, child := range node.Children {
		typed, ok := child.(Text)
		if !ok {
			return "", nil
		}
		value, _, err := interpolateValue(ctx, typed.Value)
		if err != nil {
			return "", err
		}
		text.write(value)
	}
	return strings.TrimSpace(text.string()), nil
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
