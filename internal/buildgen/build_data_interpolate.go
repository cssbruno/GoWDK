package buildgen

import (
	"fmt"
	"strings"
)

func interpolateBuildValue(value string, routeParams map[string]string, data map[string]string) (string, error) {
	if !strings.Contains(value, "{") {
		return value, nil
	}
	parts := make([]string, 0, 4)
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			parts = append(parts, value)
			return strings.Join(parts, ""), nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated interpolation")
		}
		end += start
		parts = append(parts, value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if param, ok := buildRouteParamExpression(name); ok {
			name = param
			resolved, ok := routeParams[name]
			if !ok {
				return "", fmt.Errorf("unknown route param %q", name)
			}
			parts = append(parts, resolved)
			value = value[end+1:]
			continue
		}
		wasField := false
		if field, ok := buildFieldExpression(name); ok {
			name = field
			wasField = true
		}
		resolved, ok := data[name]
		if !ok {
			resolved, ok = routeParams[name]
		}
		if !ok {
			if wasField {
				return "", fmt.Errorf("unknown build data field %q", name)
			}
			return "", fmt.Errorf("unknown build data field or route param %q", name)
		}
		parts = append(parts, resolved)
		value = value[end+1:]
	}
}

func buildFieldExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `field("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `field("`)
	if !isLiteralName(name) {
		return "", false
	}
	return name, true
}

func buildRouteParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !isLiteralName(name) {
		return "", false
	}
	return name, true
}
