package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"strings"
)

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
		if node.Name == "input" && node.SPAInputType("radio") {
			if _, ok, err := node.SPAAttrInterpolated(ctx, "value"); err != nil {
				return "", err
			} else if !ok {
				return "", fmt.Errorf("g:bind:value on radio <input> requires a literal value attribute")
			}
		}
		typ := ctx.stateTypes[field]
		if typ == clientlang.TypeInt || typ == clientlang.TypeFloat {
			if node.Name != "input" || !node.SPAInputType("number") {
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
		if node.Name != "input" || !node.SPAInputType("checkbox") {
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

func (node Element) SPAInputType(value string) bool {
	for _, attr := range node.Attrs {
		if attr.Name != "type" || attr.Boolean {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(attr.Value), value)
	}
	return value == "text"
}

type postDirectives struct {
	Action       string
	Route        string
	Command      string
	CommandStart int
	CommandEnd   int
	Query        string
	QueryStart   int
	QueryEnd     int
	Target       string
	Swap         string
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
		return directives, nil
	}
	if directives.Command != "" {
		return postDirectives{}, fmt.Errorf("form must not declare both g:post and g:command")
	}
	if directives.Query != "" {
		return postDirectives{}, fmt.Errorf("form must not declare both g:post and g:query")
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
		if attr.Name == "g:html" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				return postDirectives{}, fmt.Errorf("g:html requires an expression value")
			}
			continue
		}
		if attr.Name == "g:event" {
			return postDirectives{}, fmt.Errorf("frontend templates must not declare g:event; domain and integration events are backend-owned facts, use g:command for backend intent or g:on:* for local UI events")
		}
		if attr.Name != "g:post" && attr.Name != "g:command" && attr.Name != "g:query" && attr.Name != "g:target" && attr.Name != "g:swap" {
			return postDirectives{}, fmt.Errorf("unsupported directive attribute %q in SPA build", attr.Name)
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return postDirectives{}, fmt.Errorf("%s requires a value", attr.Name)
		}
		if attr.Name == "g:query" {
			if directives.Query != "" {
				return postDirectives{}, fmt.Errorf("element declares multiple g:query directives")
			}
			query := strings.TrimSpace(attr.Value)
			if !contractReferencePattern.MatchString(query) {
				return postDirectives{}, fmt.Errorf("g:query %q must be a package-qualified Go contract reference", query)
			}
			directives.Query = query
			directives.QueryStart = attr.Start
			directives.QueryEnd = attr.End
			continue
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
		case "g:command":
			if directives.Command != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:command directives")
			}
			command := strings.TrimSpace(attr.Value)
			if !contractReferencePattern.MatchString(command) {
				return postDirectives{}, fmt.Errorf("g:command %q must be a package-qualified Go contract reference", command)
			}
			directives.Command = command
			directives.CommandStart = attr.Start
			directives.CommandEnd = attr.End
		case "g:target":
			if directives.Target != "" {
				return postDirectives{}, fmt.Errorf("form declares multiple g:target directives")
			}
			target := strings.TrimSpace(attr.Value)
			if strings.ContainsAny(target, "{}") {
				return postDirectives{}, fmt.Errorf("g:target %q must be literal", target)
			}
			if !strings.HasPrefix(target, "#") || strings.TrimPrefix(target, "#") == "" || strings.ContainsAny(target, " \t\r\n") {
				return postDirectives{}, fmt.Errorf("g:target %q must be a literal id selector", target)
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
	if directives.Command != "" && directives.Query != "" {
		return postDirectives{}, fmt.Errorf("form must not declare both g:command and g:query")
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
