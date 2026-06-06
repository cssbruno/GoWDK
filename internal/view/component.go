package view

import (
	"encoding/json"
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
	"strings"
)

// ComponentCall invokes a parsed component with literal string props.
type ComponentCall struct {
	Name     string
	Attrs    []Attr
	Children []Node
}

type slotContent struct {
	Nodes []Node
	Ctx   renderContext
	Lets  map[string]string
}

// Identity is the package-qualified component identity used for compiler-time
// resolution and recursion checks. The public call name can be an import alias.
func (component Component) Identity() string {
	if component.Package == "" {
		return component.Name
	}
	return component.Package + "." + component.Name
}

func (ctx *renderContext) lookupComponent(name string) (Component, bool) {
	if strings.Contains(name, ".") {
		component, ok := ctx.components[name]
		if ok && component.Name == "" {
			_, component.Name, _ = strings.Cut(name, ".")
		}
		if ok {
			return component, true
		}
		alias, componentName, _ := strings.Cut(name, ".")
		packageName := ctx.uses[alias]
		if packageName == "" {
			return Component{}, false
		}
		for _, component := range ctx.components {
			if component.Package == packageName && component.Name == componentName {
				return component, true
			}
		}
		return Component{}, false
	}
	if ctx.ownerPackage != "" {
		for _, component := range ctx.components {
			if component.Package == ctx.ownerPackage && component.Name == name {
				return component, true
			}
		}
		return Component{}, false
	}
	component, ok := ctx.components[name]
	if ok && component.Name == "" {
		component.Name = name
	}
	return component, ok
}

func (node ComponentCall) render(ctx *renderContext, out *strings.Builder) error {
	component, ok := ctx.lookupComponent(node.Name)
	if !ok {
		return fmt.Errorf("missing component %q", node.Name)
	}
	identity := component.Identity()
	if ctx.stack[identity] {
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
	slots, err := componentSlots(node.Children, ctx)
	if err != nil {
		return err
	}
	slotHTML := ""
	if slot, ok := slots[""]; ok {
		var err error
		slotHTML, err = renderSlotContent(slot, nil)
		if err != nil {
			return err
		}
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
		components:   ctx.components,
		ownerPackage: component.Package,
		uses:         component.Uses,
		values:       values,
		tainted:      taintedValues,
		actions:      ctx.actions,
		stack:        cloneStack(ctx.stack),
		slotHTML:     slotHTML,
		slots:        slots,
		stateFields:  boolSet(keys(component.State)),
		readFields:   boolSet(keys(values)),
		bindFields:   boolSet(keys(bindValues)),
		handlers:     component.Handlers,
		stateTypes:   component.StateTypes,
		refs:         component.Refs,
		emits:        component.Emits,
		bindingSeq:   ctx.bindingSeq,
		islandSeq:    ctx.islandSeq,
	}
	childCtx.stack[identity] = true
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
		out.WriteString(gowhtml.Escape(component.Name))
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

func componentSlots(children []Node, ctx *renderContext) (map[string]slotContent, error) {
	slots := map[string]slotContent{}
	var defaultNodes []Node
	for _, child := range children {
		element, ok := child.(Element)
		if !ok || element.Name != "template" {
			defaultNodes = append(defaultNodes, child)
			continue
		}
		name, lets, isSlot, err := templateSlot(element)
		if err != nil {
			return nil, err
		}
		if !isSlot {
			defaultNodes = append(defaultNodes, child)
			continue
		}
		if _, exists := slots[name]; exists {
			return nil, fmt.Errorf("duplicate slot %q", name)
		}
		slots[name] = slotContent{Nodes: element.Children, Ctx: *ctx, Lets: lets}
	}
	if len(defaultNodes) > 0 {
		if _, exists := slots[""]; exists {
			return nil, fmt.Errorf("duplicate default slot")
		}
		slots[""] = slotContent{Nodes: defaultNodes, Ctx: *ctx}
	}
	return slots, nil
}

func templateSlot(node Element) (string, map[string]string, bool, error) {
	name := ""
	isSlot := false
	lets := map[string]string{}
	for _, attr := range node.Attrs {
		switch {
		case attr.Name == "g:slot":
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return "", nil, false, fmt.Errorf("g:slot requires a slot name")
			}
			name = strings.TrimSpace(attr.Value)
			isSlot = true
		case strings.HasPrefix(attr.Name, "let:"):
			prop := strings.TrimPrefix(attr.Name, "let:")
			if prop == "" {
				return "", nil, false, fmt.Errorf("slot let binding requires a name")
			}
			local := prop
			if !attr.Boolean && strings.TrimSpace(attr.Value) != "" {
				local = strings.TrimSpace(attr.Value)
			}
			lets[prop] = local
		}
	}
	return name, lets, isSlot, nil
}

func renderSlotContent(slot slotContent, scopedValues map[string]string) (string, error) {
	slotCtx := slot.Ctx
	if len(scopedValues) > 0 {
		slotCtx.values = mergeValues(slotCtx.values, scopedValues)
		slotCtx.readFields = mergeBoolSets(slotCtx.readFields, boolSet(keys(scopedValues)))
	}
	return renderNodes(slot.Nodes, &slotCtx)
}

func mergeBoolSets(base, next map[string]bool) map[string]bool {
	out := map[string]bool{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range next {
		out[key] = value
	}
	return out
}
