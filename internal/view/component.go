package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

// ComponentCall invokes a parsed component with literal string props.
type ComponentCall struct {
	Name     string
	Attrs    []Attr
	Children []Node
	Start    int
	End      int
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

func (node ComponentCall) render(ctx *renderContext, out *renderOutput) error {
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
	if mode == "" {
		mode = component.DefaultIsland
	}
	props := cloneValues(component.PropDefaults)
	propValues := map[string]any{}
	for prop, value := range props {
		typed, err := typedComponentPropString(prop, value, component.PropType(prop))
		if err != nil {
			return err
		}
		propValues[prop] = typed
	}
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
			if attr.Name == "g:event" {
				return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", node.Name)
			}
			return fmt.Errorf("component %s uses unsupported directive attribute %q", node.Name, attr.Name)
		}
		if strings.Contains(attr.Name, ":") {
			return fmt.Errorf("component %s prop renaming is not supported; pass declared prop %q directly", node.Name, strings.Split(attr.Name, ":")[0])
		}
		if !component.HasProp(attr.Name) {
			return fmt.Errorf("component %s does not declare prop %q", node.Name, attr.Name)
		}
		propType := component.PropType(attr.Name)
		if attr.Boolean {
			if propType != clientlang.TypeBool {
				return fmt.Errorf("component %s prop %q requires a value", node.Name, attr.Name)
			}
			props[attr.Name] = "true"
			propValues[attr.Name] = true
			continue
		}
		value, typedValue, tainted, err := componentPropValue(ctx, attr, propType)
		if err != nil {
			return err
		}
		props[attr.Name] = value
		propValues[attr.Name] = typedValue
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
		scopeIDs:     append([]string(nil), component.ScopeIDs...),
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
		return renderComponentIsland(out, componentIslandRender{
			Component:       component,
			IslandID:        ctx.nextIslandID(),
			Mode:            mode,
			Body:            body,
			Props:           props,
			PropValues:      propValues,
			PropExpressions: propExpressions,
			ComputedValues:  computedValues,
			ParentListeners: parentListeners,
		})
	}
	out.write(body)
	return nil
}

type componentIslandRender struct {
	Component       Component
	IslandID        string
	Mode            string
	Body            string
	Props           map[string]string
	PropValues      map[string]any
	PropExpressions map[string]string
	ComputedValues  map[string]any
	ParentListeners []parentComponentListener
}

func renderComponentIsland(out *renderOutput, island componentIslandRender) error {
	propsJSON, err := componentPropExpressionsJSON(island.PropExpressions)
	if err != nil {
		return err
	}
	stateJSON, err := componentStateJSON(island.Component.StateJSON, island.PropValues, island.ComputedValues)
	if err != nil {
		return err
	}
	if stateJSON == "" {
		stateJSON = "{}"
	}

	out.write("<gowdk-island")
	out.write(gowhtml.Attr("data-gowdk-component", island.Component.Name))
	out.write(gowhtml.Attr("data-gowdk-island", island.IslandID))
	out.write(gowhtml.Attr("data-gowdk-runtime", island.Mode))
	out.write(gowhtml.Attr("data-gowdk-state", stateJSON))
	out.write(gowhtml.Attr("data-gowdk-client", island.Component.HandlersJSON))
	out.write(gowhtml.Attr("data-gowdk-props", propsJSON))
	for _, listener := range island.ParentListeners {
		writeParentComponentListenerAttrs(out, listener)
	}
	out.write(">")
	out.write(island.Body)
	out.write("</gowdk-island>")
	return nil
}

func componentPropExpressionsJSON(propExpressions map[string]string) (string, error) {
	if len(propExpressions) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(propExpressions)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func componentPropValue(ctx *renderContext, attr Attr, propType clientlang.ValueType) (string, any, bool, error) {
	if propType == clientlang.TypeUnknown {
		propType = clientlang.TypeString
	}
	if propType == clientlang.TypeString {
		value, tainted, err := interpolateValue(ctx, attr.Value)
		if err != nil {
			return "", nil, false, err
		}
		return value, value, tainted, nil
	}
	if !attr.Expression && strings.Contains(attr.Value, "{") {
		return "", nil, false, fmt.Errorf("prop %q with type %s requires a scalar literal or expression", attr.Name, propType)
	}
	source := strings.TrimSpace(attr.Value)
	if attr.Expression {
		source = expressionAttrSource(source)
	}
	if source == "" {
		return "", nil, false, fmt.Errorf("prop %q with type %s requires a scalar literal or expression", attr.Name, propType)
	}
	value, err := clientlang.EvalValue(source, ctx.values)
	if err != nil {
		return "", nil, false, fmt.Errorf("prop %q: %w", attr.Name, err)
	}
	typed, err := coerceComponentPropValue(attr.Name, value, propType)
	if err != nil {
		return "", nil, false, err
	}
	scalar, ok := scalarString(typed)
	if !ok {
		return "", nil, false, fmt.Errorf("prop %q with type %s must resolve to a scalar value", attr.Name, propType)
	}
	return scalar, typed, false, nil
}

func coerceComponentPropValue(name string, value any, propType clientlang.ValueType) (any, error) {
	switch propType {
	case clientlang.TypeBool:
		typed, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("prop %q expects bool, got %T", name, value)
		}
		return typed, nil
	case clientlang.TypeInt:
		switch typed := value.(type) {
		case int:
			return typed, nil
		case float64:
			asInt := int(typed)
			if float64(asInt) == typed {
				return asInt, nil
			}
		}
		return nil, fmt.Errorf("prop %q expects int, got %T", name, value)
	case clientlang.TypeFloat:
		switch typed := value.(type) {
		case int:
			return float64(typed), nil
		case float64:
			return typed, nil
		}
		return nil, fmt.Errorf("prop %q expects float, got %T", name, value)
	default:
		return nil, fmt.Errorf("prop %q uses unsupported type %s", name, propType)
	}
}

func typedComponentPropString(name string, value string, propType clientlang.ValueType) (any, error) {
	if propType == clientlang.TypeUnknown {
		propType = clientlang.TypeString
	}
	if propType == clientlang.TypeString {
		return value, nil
	}
	typed, err := clientlang.EvalValue(value, nil)
	if err != nil {
		return nil, fmt.Errorf("prop %q default: %w", name, err)
	}
	return coerceComponentPropValue(name, typed, propType)
}

func writeParentComponentListenerAttrs(out *renderOutput, listener parentComponentListener) {
	out.write(gowhtml.Attr("data-gowdk-parent-on-"+listener.Event, listener.Expression))
	out.write(gowhtml.Attr("data-gowdk-parent-event-"+listener.Event, listener.Modifiers))
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
