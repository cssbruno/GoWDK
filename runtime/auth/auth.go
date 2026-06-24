package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	// PublicGuard marks a generated page or route as intentionally public.
	PublicGuard = "public"
	// RoleGuardPrefix marks a native RBAC guard that requires a principal role.
	RoleGuardPrefix = "role:"
	// PermissionGuardPrefix marks a native RBAC guard that requires a principal permission.
	PermissionGuardPrefix = "permission:"
)

var (
	// ErrUnauthenticated reports that a native RBAC guard has no principal.
	ErrUnauthenticated = errors.New("gowdk principal is unauthenticated")
	// ErrForbidden reports that a native RBAC guard rejected the principal.
	ErrForbidden = errors.New("gowdk principal is forbidden")
)

// Principal is the current application-owned identity visible to native RBAC
// guards. AuthorizationVersion is optional metadata that revocable session
// providers can compare against current server-side authorization state. GOWDK
// does not own users, sessions, OAuth, tenants, or persistence.
type Principal struct {
	ID                   string
	Roles                []string
	Permissions          []string
	AuthorizationVersion string
}

// HasRole reports whether the principal has role.
func (p Principal) HasRole(role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	for _, candidate := range p.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

// HasPermission reports whether the principal has permission.
func (p Principal) HasPermission(permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, candidate := range p.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

// Provider returns the current application-owned principal for a request. A nil
// principal means unauthenticated.
type Provider interface {
	Principal(*http.Request) (*Principal, error)
}

// ProviderFunc adapts a function into a Provider.
type ProviderFunc func(*http.Request) (*Principal, error)

// Principal returns the current application-owned principal.
func (fn ProviderFunc) Principal(request *http.Request) (*Principal, error) {
	return fn(request)
}

// IsNativeGuard reports whether name is a built-in role or permission guard.
func IsNativeGuard(name string) bool {
	return strings.HasPrefix(name, RoleGuardPrefix) || strings.HasPrefix(name, PermissionGuardPrefix)
}

// IsPublicGuard reports whether name is the explicit public access marker.
func IsPublicGuard(name string) bool {
	return strings.TrimSpace(name) == PublicGuard
}
