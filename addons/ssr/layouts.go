package ssr

import "fmt"

// LayoutStack is the ordered set of request-time layouts for an SSR page.
type LayoutStack []string

// LayoutFunc wraps already-rendered child HTML with request-aware layout HTML.
type LayoutFunc func(LoadContext, string) (string, error)

// LayoutRegistry maps layout IDs to request-aware layout functions.
type LayoutRegistry map[string]LayoutFunc

// ComposeLayouts wraps body with the declared layout stack. Layouts are listed
// from outermost to innermost, matching @layout root, dashboard semantics.
func ComposeLayouts(ctx LoadContext, stack LayoutStack, registry LayoutRegistry, body string) (string, error) {
	out := body
	for index := len(stack) - 1; index >= 0; index-- {
		name := stack[index]
		layout, ok := registry[name]
		if !ok || layout == nil {
			return "", fmt.Errorf("SSR layout %q is not registered", name)
		}
		next, err := layout(ctx, out)
		if err != nil {
			return "", fmt.Errorf("SSR layout %q failed: %w", name, err)
		}
		out = next
	}
	return out, nil
}
