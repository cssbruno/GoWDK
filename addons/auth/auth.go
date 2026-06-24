// Package auth is the batteries-included GOWDK authentication addon. It enables
// the auth feature and ships a working, dependency-free identity implementation:
// PBKDF2 password hashing, a PasswordHasher replacement point, and signed-cookie
// or revocable sessions, all on the Go standard library. It builds on the native
// RBAC guard machinery in runtime/auth. Pages and routes protected with guard
// role:... or guard permission:... resolve through a session-backed Provider;
// guard public remains intentionally unauthenticated.
//
// GOWDK still does not own your user store. Look users up however you like, then
// hand the addon a Principal to issue a session for; the addon owns default
// hashing helpers, session signing, and request-time principal resolution.
// Revocable sessions require an application-owned SessionStore.
package auth

import (
	"github.com/cssbruno/gowdk"
	runtimeauth "github.com/cssbruno/gowdk/runtime/auth"
	"github.com/cssbruno/gowdk/runtime/guard"
)

// ImportPath is the canonical Go import path for the auth addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/auth"

type addon struct {
	options Options
}

// Addon enables session-backed authentication and native RBAC guards.
func Addon(options ...Options) gowdk.Addon {
	var selected Options
	if len(options) > 0 {
		selected = options[0]
	}
	return addon{options: selected}
}

func (addon) Name() string {
	return "auth"
}

func (addon) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureAuth}
}

func (a addon) AuthSessionOptions() gowdk.AuthSessionOptions {
	secretEnv := a.options.SecretEnv
	if secretEnv == "" {
		secretEnv = DefaultSessionSecretEnv
	}
	return gowdk.AuthSessionOptions{
		SecretEnv:  secretEnv,
		CookieName: a.options.CookieName,
		TTL:        a.options.TTL,
		Insecure:   a.options.Insecure,
	}
}

// Principal is the application identity visible to native RBAC guards. It is
// re-exported from runtime/auth so callers of this addon need only one import.
type Principal = runtimeauth.Principal

// Provider resolves the current principal for a request. Register the value
// returned by Sessions.Provider with the generated RegisterAuthProvider hook.
type Provider = runtimeauth.Provider

// ProviderFunc adapts a function into a Provider.
type ProviderFunc = runtimeauth.ProviderFunc

// RequireAuthenticated returns an auth.required guard backed by provider. When
// provider is nil, it falls back to the addon-level Sessions configured by the
// generated app.
func RequireAuthenticated(provider Provider) guard.Func {
	return func(ctx guard.Context) error {
		current := provider
		if current == nil {
			sessions, err := DefaultSessions()
			if err != nil {
				return err
			}
			current = sessions.Provider()
		}
		principal, err := current.Principal(ctx.Request)
		if err != nil {
			return err
		}
		if principal == nil {
			return runtimeauth.ErrUnauthenticated
		}
		return nil
	}
}
