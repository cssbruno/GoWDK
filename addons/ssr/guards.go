package ssr

import (
	"github.com/cssbruno/gowdk/runtime/auth"
	"github.com/cssbruno/gowdk/runtime/guard"
)

// GuardFunc authorizes one generated request-time route access check.
type GuardFunc = guard.Func

// GuardRegistry resolves guard IDs to executable guard functions.
type GuardRegistry = guard.Registry

// RunGuards executes guard IDs in declaration order.
func RunGuards(ctx LoadContext, names []string, registry GuardRegistry) error {
	return guard.RunGuards(ctx, names, registry)
}

// RunGuardsWithAuth executes guard IDs in declaration order and resolves native
// RBAC guard IDs such as role:admin and permission:posts.write through provider.
func RunGuardsWithAuth(ctx LoadContext, names []string, registry GuardRegistry, provider auth.Provider) error {
	return guard.RunGuardsWithAuth(ctx, names, registry, provider)
}

// IsNativeRBACGuard reports whether name is a built-in role or permission guard.
func IsNativeRBACGuard(name string) bool {
	return guard.IsNativeRBACGuard(name)
}

// NativeRBACGuard returns a guard for a native role or permission guard ID.
func NativeRBACGuard(name string, provider auth.Provider) GuardFunc {
	return guard.NativeRBACGuard(name, provider)
}
