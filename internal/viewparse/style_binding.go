package viewparse

import (
	"fmt"
	"strings"
)

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
