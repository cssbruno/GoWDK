// Package auth is the batteries-included GOWDK authentication addon. It enables
// the auth feature and ships a working, dependency-free identity implementation:
// PBKDF2 password hashing, a PasswordHasher replacement point, and signed-cookie
// sessions, all on the Go standard library. It builds on the native RBAC guard
// machinery in runtime/auth, so pages and routes protected with guard role:... /
// guard permission:... / guard public resolve through a session-backed Provider.
//
// GOWDK still does not own your user store. Look users up however you like, then
// hand the addon a Principal to issue a session for; the addon owns default
// hashing helpers, session signing, and request-time principal resolution.
package auth

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/auth"
)

// ImportPath is the canonical Go import path for the auth addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/auth"

// Addon enables session-backed authentication and native RBAC guards.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("auth", gowdk.FeatureAuth)
}

// Principal is the application identity visible to native RBAC guards. It is
// re-exported from runtime/auth so callers of this addon need only one import.
type Principal = auth.Principal

// Provider resolves the current principal for a request. Register the value
// returned by Sessions.Provider with the generated RegisterAuthProvider hook.
type Provider = auth.Provider

// ProviderFunc adapts a function into a Provider.
type ProviderFunc = auth.ProviderFunc
