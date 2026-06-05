package view

import (
	"fmt"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
	"strings"
)

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
