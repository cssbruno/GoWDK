package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

type slotContent struct {
	Nodes []Node
	Ctx   renderContext
	Lets  map[string]string
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

func renderComponentCall(node ComponentCall, ctx *renderContext, out *renderOutput) error {
	component, ok := ctx.lookupComponent(node.Name)
	if !ok {
		return fmt.Errorf("missing component %q", node.Name)
	}
	identity := component.Identity()
	if ctx.stack[identity] {
		return fmt.Errorf("recursive component %q", node.Name)
	}

	mode, err := componentCallIslandMode(node)
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
	providedProps := map[string]string{}
	var parentListeners []parentComponentListener
	for _, attr := range node.Attrs {
		if attr.Spread {
			expanded, err := componentSpreadProps(ctx, component)
			if err != nil {
				return fmt.Errorf("component %s: %w", node.Name, err)
			}
			for _, spreadAttr := range expanded {
				if err := applyComponentPropAttr(ctx, component, node.Name, spreadAttr, props, propValues, propExpressions, taintedValues, providedProps); err != nil {
					return err
				}
			}
			continue
		}
		if strings.HasPrefix(attr.Name, "g:") {
			if attr.Name == "g:island" {
				continue
			}
			if strings.HasPrefix(attr.Name, "g:on:") {
				listener, err := componentCallParentListener(node, attr, component, ctx)
				if err != nil {
					return err
				}
				merged, err := addParentListener(parentListeners, listener)
				if err != nil {
					return fmt.Errorf("component %s %w", node.Name, err)
				}
				parentListeners = merged
				continue
			}
			if attr.Name == "g:event" {
				return fmt.Errorf("component %s must not declare g:event; domain and integration events are backend-owned facts", node.Name)
			}
			if strings.HasPrefix(attr.Name, "g:bind:") {
				listener, err := applyComponentBinding(ctx, component, node.Name, attr, props, propValues, propExpressions, taintedValues, providedProps)
				if err != nil {
					return err
				}
				merged, err := addParentListener(parentListeners, listener)
				if err != nil {
					return fmt.Errorf("component %s %w", node.Name, err)
				}
				parentListeners = merged
				continue
			}
			if attr.Name == "g:bind" {
				return fmt.Errorf("component %s uses unsupported bind target %q; component bindings must use g:bind:<exportedState>", node.Name, attr.Name)
			}
			return fmt.Errorf("component %s uses unsupported directive attribute %q", node.Name, attr.Name)
		}
		if err := applyComponentPropAttr(ctx, component, node.Name, attr, props, propValues, propExpressions, taintedValues, providedProps); err != nil {
			return err
		}
	}
	for _, prop := range component.Props {
		if _, ok := props[prop]; !ok {
			return fmt.Errorf("component %s missing required prop %q", node.Name, prop)
		}
	}
	for prop := range props {
		if !component.HasProp(prop) && !component.HasStateField(prop) {
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
		renderComponentContext: renderComponentContext{
			components:             ctx.components,
			ownerPackage:           component.Package,
			uses:                   component.Uses,
			realtimeEventTypeNames: ctx.realtimeEventTypeNames,
			queryTypeNames:         ctx.queryTypeNames,
			stack:                  cloneStack(ctx.stack),
			slotHTML:               slotHTML,
			slots:                  slots,
			scopeIDs:               append([]string(nil), component.ScopeIDs...),
		},
		renderDataContext: renderDataContext{
			values:       values,
			tainted:      taintedValues,
			actions:      ctx.actions,
			actionFields: ctx.actionFields,
			propFields:   boolSet(component.Props),
			stateFields:  boolSet(keys(component.State)),
			readFields:   boolSet(keys(values)),
			bindFields:   boolSet(keys(bindValues)),
		},
		renderClientContext: renderClientContext{
			handlers:   component.Handlers,
			stateTypes: component.StateTypes,
			refs:       component.Refs,
			emits:      component.Emits,
		},
		ids: ctx.ids,
	}
	childCtx.stack[identity] = true
	var body string
	var renderErr error
	if len(component.Nodes) > 0 {
		body, renderErr = renderParsedNodes(component.Nodes, childCtx)
	} else {
		body, renderErr = render(component.Body, childCtx)
	}
	if renderErr != nil {
		return renderErr
	}
	if component.StateJSON != "" || component.HandlersJSON != "" || mode != "" || len(component.Emits) > 0 || len(component.Exports) > 0 || len(propExpressions) > 0 {
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
	if identity := island.Component.Identity(); identity != island.Component.Name {
		out.write(gowhtml.Attr("data-gowdk-component-id", identity))
	}
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

func applyComponentBinding(ctx *renderContext, component Component, callName string, attr Attr, props map[string]string, propValues map[string]any, propExpressions map[string]string, taintedValues map[string]bool, providedProps map[string]string) (parentComponentListener, error) {
	if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return parentComponentListener{}, fmt.Errorf("%s requires a parent state field", attr.Name)
	}
	target := strings.TrimPrefix(attr.Name, "g:bind:")
	if target == "" || target == attr.Name {
		return parentComponentListener{}, fmt.Errorf("component %s uses unsupported bind target %q", callName, attr.Name)
	}
	exportType, ok := component.Exports[target]
	if !ok {
		return parentComponentListener{}, fmt.Errorf("component %s bind target %q must be declared in exports", callName, target)
	}
	if !component.HasStateField(target) {
		return parentComponentListener{}, fmt.Errorf("component %s bind target %q must be declared in state", callName, target)
	}
	parentField := expressionAttrSource(attr.Value)
	parentType, err := validateIslandSymbol(parentField, ctx.writeSymbols())
	if err != nil {
		return parentComponentListener{}, fmt.Errorf("%s: %w", attr.Name, err)
	}
	if parentType != clientlang.TypeUnknown && exportType != clientlang.TypeUnknown && parentType != exportType && !compatibleNumericType(exportType, parentType) {
		return parentComponentListener{}, fmt.Errorf("component %s bind target %q exports %s but parent field %q is %s", callName, target, exportType, parentField, parentType)
	}
	if previous, exists := providedProps[target]; exists {
		return parentComponentListener{}, fmt.Errorf("component %s prop %q is provided more than once by %s and %s", callName, target, previous, attr.Name)
	}
	providedProps[target] = attr.Name
	bindingAttr := Attr{Name: target, Value: "{" + parentField + "}", Expression: true}
	value, typedValue, _, err := componentPropValue(ctx, bindingAttr, exportType)
	if err != nil {
		return parentComponentListener{}, err
	}
	props[target] = value
	propValues[target] = typedValue
	propExpressions[target] = parentField
	if ctx.tainted[parentField] {
		taintedValues[target] = true
	}
	return parentComponentListener{
		Event:      "exports",
		Expression: parentField + " = event." + target,
	}, nil
}

func applyComponentPropAttr(ctx *renderContext, component Component, callName string, attr Attr, props map[string]string, propValues map[string]any, propExpressions map[string]string, taintedValues map[string]bool, providedProps map[string]string) error {
	propName, sourceName, err := componentPropTarget(component, callName, attr)
	if err != nil {
		return err
	}
	if previous, exists := providedProps[propName]; exists {
		return fmt.Errorf("component %s prop %q is provided more than once by %s and %s", callName, propName, previous, sourceName)
	}
	providedProps[propName] = sourceName
	propType := component.PropType(propName)
	if attr.Boolean && sourceName == propName {
		if propType != clientlang.TypeBool {
			return fmt.Errorf("component %s prop %q requires a value", callName, propName)
		}
		props[propName] = "true"
		propValues[propName] = true
		return nil
	}
	if attr.Boolean {
		attr = Attr{Name: propName, Value: "{" + sourceName + "}", Expression: true}
	}
	value, typedValue, tainted, err := componentPropValue(ctx, Attr{Name: propName, Value: attr.Value, Boolean: attr.Boolean, Expression: attr.Expression}, propType)
	if err != nil {
		return err
	}
	props[propName] = value
	propValues[propName] = typedValue
	if attr.Expression {
		propExpressions[propName] = expressionAttrSource(attr.Value)
	}
	if tainted {
		taintedValues[propName] = true
	}
	return nil
}

func (ctx *renderContext) writeSymbols() map[string]clientlang.ValueType {
	if len(ctx.stateTypes) > 0 {
		return ctx.stateTypes
	}
	return boolFieldSymbols(ctx.bindFields)
}

// addParentListener appends a parent listener, merging it into an existing
// listener for the same event instead of rejecting it. This lets a component
// bind several exported state fields at once (multiple g:bind:<export>) or pair
// a g:on:exports handler with bindings: their expressions are joined and run as
// ordered statements in the browser. Different event modifiers are rejected
// because a single data-gowdk-parent-event-* attribute cannot preserve separate
// listener timing for each merged expression.
func addParentListener(listeners []parentComponentListener, next parentComponentListener) ([]parentComponentListener, error) {
	for i, existing := range listeners {
		if existing.Event != next.Event {
			continue
		}
		if existing.Modifiers != next.Modifiers {
			return listeners, fmt.Errorf("declares incompatible modifiers for parent event %q", next.Event)
		}
		existing.Expression = existing.Expression + "; " + next.Expression
		listeners[i] = existing
		return listeners, nil
	}
	return append(listeners, next), nil
}

func componentPropTarget(component Component, callName string, attr Attr) (string, string, error) {
	propName := attr.Name
	sourceName := attr.Name
	if strings.Contains(attr.Name, ":") {
		left, right, ok := strings.Cut(attr.Name, ":")
		if !ok || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" || strings.Contains(right, ":") {
			return "", "", fmt.Errorf("component %s prop rename %q must use target:alias", callName, attr.Name)
		}
		propName = left
		sourceName = right
	}
	if !component.HasProp(propName) {
		return "", "", fmt.Errorf("component %s does not declare prop %q", callName, propName)
	}
	return propName, sourceName, nil
}

func componentSpreadProps(ctx *renderContext, component Component) ([]Attr, error) {
	if len(ctx.propFields) == 0 {
		return nil, fmt.Errorf("spread source props is not available outside a component prop scope")
	}
	var attrs []Attr
	for _, prop := range component.Props {
		if !ctx.propFields[prop] {
			continue
		}
		if _, ok := ctx.values[prop]; !ok {
			continue
		}
		attrs = append(attrs, Attr{Name: prop, Value: "{" + prop + "}", Expression: true, Spread: true})
	}
	if len(attrs) == 0 {
		return nil, fmt.Errorf("spread source props has no fields matching component %s", component.Name)
	}
	return attrs, nil
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

func componentCallParentListener(node ComponentCall, attr Attr, component Component, ctx *renderContext) (parentComponentListener, error) {
	if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return parentComponentListener{}, fmt.Errorf("%s requires an expression value", attr.Name)
	}
	directive, err := ParseEventDirective(attr.Name)
	if err != nil {
		return parentComponentListener{}, err
	}
	event, ok := component.Emits[directive.Event]
	if !ok {
		if directive.Event != "exports" || len(component.Exports) == 0 {
			return parentComponentListener{}, fmt.Errorf("component %s does not emit event %q", node.Name, directive.Event)
		}
		readSymbols := mergeClientSymbols(ctx.readSymbols(), exportPayloadSymbols(component.Exports))
		if err := ValidateIslandEventExpressionTypedWithFunctions(attr.Value, readSymbols, ctx.stateTypes, ctx.handlers, nil); err != nil {
			return parentComponentListener{}, fmt.Errorf("%s: %w", attr.Name, err)
		}
		return parentComponentListener{
			Event:      directive.Event,
			Expression: attr.Value,
			Modifiers:  directive.RuntimeOptions(),
		}, nil
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

func exportPayloadSymbols(exports map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	out := map[string]clientlang.ValueType{
		"event":        clientlang.TypeObject,
		"event.active": clientlang.TypeBool,
	}
	for name, typ := range exports {
		if typ == clientlang.TypeUnknown {
			typ = clientlang.TypeString
		}
		out["event."+name] = typ
	}
	return out
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

func componentCallIslandMode(node ComponentCall) (string, error) {
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
