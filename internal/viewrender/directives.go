package viewrender

import "sort"

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
	"g:event":         true,
	"g:for":           true,
	"g:unsafe-html":   true,
	"g:if":            true,
	"g:island":        true,
	"g:key":           true,
	"g:max-file-size": true,
	"g:max-files":     true,
	"g:transition":    true,
	"g:animate":       true,
	"g:post":          true,
	"g:query":         true,
	"g:ref":           true,
	"g:slot":          true,
	"g:subscribe":     true,
	"g:swap":          true,
	"g:target":        true,
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
