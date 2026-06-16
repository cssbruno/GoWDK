package view

import (
	"fmt"
	stdhtml "html"
	"strings"

	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

func renderText(ctx *renderContext, out *renderOutput, value string) error {
	value = decodeSourceTextEntities(value)
	text, _, err := interpolateValue(ctx, value)
	if err != nil {
		return err
	}
	text = restoreSourceTextBraces(text)
	out.write(gowhtml.Escape(text))
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
	var out renderOutput
	tainted := false
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			out.write(value)
			return out.string(), tainted, nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", false, fmt.Errorf("unterminated interpolation")
		}
		end += start
		out.write(value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if name == "" {
			return "", false, fmt.Errorf("empty interpolation")
		}
		if ctx.serverScope != nil {
			placeholder, err := ctx.serverScope.serverScopeFieldPlaceholder(name, ctx.idAllocator())
			if err != nil {
				return "", false, err
			}
			tainted = true
			out.write(placeholder)
			value = value[end+1:]
			continue
		}
		if ctx.templateLoop != nil {
			out.write(loopTemplateValue(name))
			value = value[end+1:]
			continue
		}
		if param, ok := routeParamExpression(name); ok {
			resolved, ok := ctx.values[param]
			if !ok {
				return "", false, fmt.Errorf("unknown route param %q", param)
			}
			tainted = true
			out.write(resolved)
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
		out.write(resolved)
		value = value[end+1:]
	}
}

func unsafeRouteParamAttr(name string) bool {
	if inlineEventHandlerAttr(name) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(name), "style") || strings.EqualFold(strings.TrimSpace(name), "srcdoc") {
		return true
	}
	return urlBearingAttr(name)
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

const (
	escapedOpenBrace  = "\x00GOWDK_OPEN_BRACE\x00"
	escapedCloseBrace = "\x00GOWDK_CLOSE_BRACE\x00"
)

func decodeSourceTextEntities(value string) string {
	if !strings.Contains(value, "&") {
		return value
	}
	value = strings.ReplaceAll(value, "&#123;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#x7b;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#X7B;", escapedOpenBrace)
	value = strings.ReplaceAll(value, "&#125;", escapedCloseBrace)
	value = strings.ReplaceAll(value, "&#x7d;", escapedCloseBrace)
	value = strings.ReplaceAll(value, "&#X7D;", escapedCloseBrace)
	return stdhtml.UnescapeString(value)
}

func restoreSourceTextBraces(value string) string {
	value = strings.ReplaceAll(value, escapedOpenBrace, "{")
	value = strings.ReplaceAll(value, escapedCloseBrace, "}")
	return value
}
