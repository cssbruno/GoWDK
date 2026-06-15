package viewvalidation

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

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
	return clientlang.ValidateIslandBoolExpressionTyped(expr, symbols)
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

func classToggleName(name string) string {
	return strings.TrimSpace(strings.TrimPrefix(name, "class:"))
}

type styleBinding struct {
	Property string
	Unit     string
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
		return len(name) > 2 && isStyleCustomProperty(name)
	}
	return isStyleProperty(name)
}

func isSupportedStyleUnit(unit string) bool {
	switch unit {
	case "px", "rem", "em", "vh", "vw", "vmin", "vmax", "ch", "ex", "lh", "rlh", "ms", "s", "%":
		return true
	default:
		return false
	}
}

func isStyleProperty(source string) bool {
	if source == "" || source[0] < 'a' || source[0] > 'z' {
		return false
	}
	for index := 1; index < len(source); index++ {
		char := source[index]
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isStyleCustomProperty(source string) bool {
	body, ok := strings.CutPrefix(source, "--")
	if !ok || body == "" {
		return false
	}
	for index := 0; index < len(body); index++ {
		char := body[index]
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
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
