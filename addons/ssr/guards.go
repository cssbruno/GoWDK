package ssr

import "fmt"

// GuardFunc authorizes one request-time SSR page access check.
type GuardFunc func(LoadContext) error

// GuardRegistry resolves @guard IDs to executable guard functions.
type GuardRegistry map[string]GuardFunc

// RunGuards executes guard IDs in declaration order.
func RunGuards(ctx LoadContext, names []string, registry GuardRegistry) error {
	for _, name := range names {
		guard := registry[name]
		if guard == nil {
			return fmt.Errorf("SSR guard %q is not registered", name)
		}
		if err := guard(ctx); err != nil {
			return fmt.Errorf("SSR guard %q failed: %w", name, err)
		}
	}
	return nil
}
