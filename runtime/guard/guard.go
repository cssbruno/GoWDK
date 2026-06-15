package guard

import (
	"context"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/runtime/auth"
	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

// Func authorizes one generated request-time route or endpoint access check.
type Func func(Context) error

// Registry resolves guard IDs to executable guard functions.
type Registry map[string]Func

// RunGuards executes guard IDs in declaration order.
func RunGuards(ctx Context, names []string, registry Registry) error {
	return RunGuardsWithAuth(ctx, names, registry, nil)
}

// RunGuardsWithAuth executes guard IDs in declaration order and resolves native
// RBAC guard IDs such as role:admin and permission:posts.write through provider.
func RunGuardsWithAuth(ctx Context, names []string, registry Registry, provider auth.Provider) error {
	for _, name := range names {
		guard := registry[name]
		if guard == nil && IsNativeRBACGuard(name) {
			guard = NativeRBACGuard(name, provider)
		}
		if guard == nil {
			return fmt.Errorf("guard %q is not registered", name)
		}
		guardCtx, span := startGuardTrace(ctx, name)
		ctx.Context = guardCtx
		if err := guard(ctx); err != nil {
			span.SetStatus(gowdktrace.StatusError, err.Error())
			span.End()
			return fmt.Errorf("guard %q failed: %w", name, err)
		}
		span.SetStatus(gowdktrace.StatusOK, "")
		span.End()
	}
	return nil
}

func startGuardTrace(ctx Context, name string) (context.Context, *gowdktrace.Span) {
	if _, ok := gowdktrace.TracerFromContext(ctx.Context); !ok {
		return ctx.Context, nil
	}
	return gowdktrace.Start(ctx.Context, "guard "+name,
		gowdktrace.WithSurface(gowdktrace.SurfaceBackend),
		gowdktrace.WithLane(gowdktrace.LaneGuard),
		gowdktrace.WithAttributes(map[string]any{"gowdk.guard": name}),
	)
}

// IsNativeRBACGuard reports whether name is a built-in role or permission guard.
func IsNativeRBACGuard(name string) bool {
	return auth.IsNativeGuard(name)
}

// NativeRBACGuard returns a guard for a native role or permission guard ID.
func NativeRBACGuard(name string, provider auth.Provider) Func {
	return func(ctx Context) error {
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
