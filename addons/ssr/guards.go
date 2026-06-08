package ssr

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/runtime/auth"
)

// GuardFunc authorizes one generated request-time route access check.
type GuardFunc func(LoadContext) error

// GuardRegistry resolves @guard IDs to executable guard functions.
type GuardRegistry map[string]GuardFunc

// RunGuards executes guard IDs in declaration order.
func RunGuards(ctx LoadContext, names []string, registry GuardRegistry) error {
	return RunGuardsWithAuth(ctx, names, registry, nil)
}

// RunGuardsWithAuth executes guard IDs in declaration order and resolves native
// RBAC guard IDs such as role:admin and permission:posts.write through provider.
func RunGuardsWithAuth(ctx LoadContext, names []string, registry GuardRegistry, provider auth.Provider) error {
	for _, name := range names {
		guard := registry[name]
		if guard == nil && IsNativeRBACGuard(name) {
			guard = NativeRBACGuard(name, provider)
		}
		if guard == nil {
			return fmt.Errorf("SSR guard %q is not registered", name)
		}
		if err := guard(ctx); err != nil {
			return fmt.Errorf("SSR guard %q failed: %w", name, err)
		}
	}
	return nil
}

// IsNativeRBACGuard reports whether name is a built-in role or permission guard.
func IsNativeRBACGuard(name string) bool {
	return auth.IsNativeGuard(name)
}

// NativeRBACGuard returns a guard for a native role or permission guard ID.
func NativeRBACGuard(name string, provider auth.Provider) GuardFunc {
	return func(ctx LoadContext) error {
		if provider == nil {
			return fmt.Errorf("native RBAC guard %q requires an auth provider", name)
		}
		principal, err := provider.Principal(ctx.Request)
		if err != nil {
			return err
		}
		if principal == nil {
			return auth.ErrUnauthenticated
		}
		if role, ok := strings.CutPrefix(name, auth.RoleGuardPrefix); ok {
			role = strings.TrimSpace(role)
			if role == "" {
				return fmt.Errorf("native RBAC guard %q requires a role", name)
			}
			if !principal.HasRole(role) {
				return auth.ErrForbidden
			}
			return nil
		}
		if permission, ok := strings.CutPrefix(name, auth.PermissionGuardPrefix); ok {
			permission = strings.TrimSpace(permission)
			if permission == "" {
				return fmt.Errorf("native RBAC guard %q requires a permission", name)
			}
			if !principal.HasPermission(permission) {
				return auth.ErrForbidden
			}
			return nil
		}
		return fmt.Errorf("native RBAC guard %q is unsupported", name)
	}
}
