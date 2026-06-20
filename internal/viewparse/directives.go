package viewparse

import (
	"fmt"
	"sort"
	"strings"
)

// supportedDirectiveNames is the closed set of exact-name g: directives owned
// by the GOWDK view {} markup contract. Prefixed families (g:on:* and
// g:message:*) are validated separately. Any other g: attribute is rejected at
// parse time with the registered unsupported_markup_directive diagnostic
// message so unknown directives can never pass through silently.
var supportedDirectiveNames = map[string]bool{
	"g:bind:checked": true,
	"g:bind:value":   true,
	"g:command":      true,
	"g:else":         true,
	"g:else-if":      true,
	// g:event parses so the renderer can explain that domain events are
	// backend-owned facts instead of emitting a generic unknown-directive error.
	"g:event": true,
	// g:for and g:if span both lanes: over server {} request-time data they
	// render server-side (the former g:each/g:when), and over client state/store
	// they bind reactive islands. The compiler infers the lane from the operand.
	"g:for":         true,
	"g:unsafe-html": true,
	"g:if":          true,
	"g:island":      true,
	"g:key":         true,
	"g:post":        true,
	"g:query":       true,
	"g:ref":         true,
	"g:slot":        true,
	"g:subscribe":   true,
	"g:swap":        true,
	"g:target":      true,
}

// supportedMessageDirectives are the g:message:* validation-message rules.
var supportedMessageDirectives = map[string]bool{
	"g:message:required":  true,
	"g:message:minlength": true,
	"g:message:maxlength": true,
	"g:message:pattern":   true,
}

// SupportedDirectiveNames returns the sorted closed set of exact-name g:
// directives owned by the current view contract (excluding the g:on:* event
// family and the g:message:* rules, which are validated separately). It is the
// source of truth cross-checked against docs/language/stability.md.
func SupportedDirectiveNames() []string {
	names := make([]string, 0, len(supportedDirectiveNames))
	for name := range supportedDirectiveNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isSupportedDirectiveName(name string) bool {
	if supportedDirectiveNames[name] {
		return true
	}
	if strings.HasPrefix(name, "g:on:") {
		// Event names and modifiers are validated by ParseEventDirective via
		// isAttrName before this check runs.
		return true
	}
	return supportedMessageDirectives[name]
}

func isComponentBindDirective(name string) bool {
	return name == "g:bind" || strings.HasPrefix(name, "g:bind:")
}

// unsupportedDirectiveMessage is the canonical unsupported_markup_directive
// message for a g: attribute outside the owned directive contract. Deferred
// construct families get explicit guidance instead of a generic rejection.
func unsupportedDirectiveMessage(name string) string {
	switch {
	case name == "g:html":
		return "g:html was renamed to g:unsafe-html to make the raw-HTML XSS surface explicit; use g:unsafe-html={Expr} to opt into trusted raw HTML"
	case name == "g:each":
		return "g:each was unified into g:for; use g:for={item in collection} — the compiler renders it server-side when the collection is a server {} field and as a client island over state/store"
	case name == "g:when":
		return "g:when was unified into g:if; use g:if={field} (or g:if={!field}) — the compiler renders it server-side when the condition is a server {} field and as a client conditional over state/store"
	case name == "g:transition" || name == "g:animate":
		return fmt.Sprintf("unsupported g: directive %q; transitions and animations are deferred from the view {} contract — use CSS transitions or a future addon-specific contract", name)
	case name == "g:window" || name == "g:document" || name == "g:body" || name == "g:head":
		return fmt.Sprintf("unsupported g: directive %q; document, window, body, and head targets are deferred from the view {} contract — use page metadata such as title, or g:on:* on rendered elements", name)
	case name == "g:await" || name == "g:async":
		return fmt.Sprintf("unsupported g: directive %q; use a bounded {#await fetchJSON[T](urlExpr)} block inside a client island for local async placeholders", name)
	case name == "g:use" || name == "g:action" || name == "g:attach":
		return fmt.Sprintf("unsupported g: directive %q; DOM actions and attachments are deferred from the view {} contract — use component client {} blocks with g:ref", name)
	case strings.HasPrefix(name, "g:bind:") || name == "g:bind":
		return fmt.Sprintf("unsupported g: directive %q; supported g:bind targets are g:bind:value and g:bind:checked", name)
	case strings.HasPrefix(name, "g:message:") || name == "g:message":
		return fmt.Sprintf("unsupported g: directive %q; supported g:message rules are required, minlength, maxlength, and pattern", name)
	default:
		return fmt.Sprintf("unsupported g: directive %q; supported g: directives are listed in docs/language/markup.md", name)
	}
}

// validateRawHTMLDirective enforces the parse-time g:unsafe-html contract: one g:unsafe-html
// per element, expression value only, no markup children, no void elements,
// and no combination with g:for/g:key or g:bind:* directives.
func validateRawHTMLDirective(name string, attrs []Attr, children []Node) error {
	count := 0
	var raw Attr
	for _, attr := range attrs {
		if attr.Name == "g:unsafe-html" {
			count++
			raw = attr
		}
	}
	if count == 0 {
		return nil
	}
	if count > 1 {
		return fmt.Errorf("element declares multiple g:unsafe-html directives")
	}
	if voidElements[name] {
		return fmt.Errorf("g:unsafe-html is not supported on void element <%s>", name)
	}
	if raw.Boolean || strings.TrimSpace(raw.Value) == "" {
		return fmt.Errorf("g:unsafe-html requires an expression value such as g:unsafe-html={Body}")
	}
	if !raw.Expression {
		return fmt.Errorf("g:unsafe-html must use an expression value such as g:unsafe-html={Body}, not a string literal")
	}
	if len(children) > 0 {
		return fmt.Errorf("element with g:unsafe-html must not declare children; the g:unsafe-html expression provides the element content")
	}
	for _, attr := range attrs {
		if attr.Name == "g:for" || attr.Name == "g:each" || attr.Name == "g:when" || attr.Name == "g:key" || strings.HasPrefix(attr.Name, "g:bind:") {
			return fmt.Errorf("g:unsafe-html cannot combine with %s", attr.Name)
		}
	}
	return nil
}
