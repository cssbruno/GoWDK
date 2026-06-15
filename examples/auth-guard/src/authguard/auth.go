package authguard

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	gowdkauth "github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/guard"
	"github.com/cssbruno/gowdk/runtime/response"
)

const (
	sessionSecretEnv = "GOWDK_AUTH_SESSION_SECRET"
	sessionCookie    = "gowdk_auth_guard_session"
)

var passwordHasher gowdkauth.PasswordHasher = gowdkauth.PBKDF2Hasher{}

type DashboardData struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
}

var sessionState struct {
	sync.Mutex
	manager *gowdkauth.Sessions
	err     error
}

var passwordState struct {
	sync.Mutex
	hash string
	err  error
}

func Login(_ context.Context, values form.Values) (response.Response, error) {
	email := strings.TrimSpace(values.First("email"))
	password := values.First("password")
	encoded, err := demoPasswordHash()
	if err != nil {
		return response.Response{}, err
	}

	if !constantEqual(email, env("GOWDK_AUTH_GUARD_EMAIL", "demo@example.com")) ||
		!passwordHasher.VerifyPassword(password, encoded) {
		return response.RedirectTo("/?login=failed"), nil
	}

	sessions, err := Sessions()
	if err != nil {
		return response.Response{}, err
	}
	cookie, err := sessions.Cookie(gowdkauth.Principal{
		ID:          email,
		Roles:       []string{"user"},
		Permissions: []string{"dashboard.read"},
	})
	if err != nil {
		return response.Response{}, err
	}
	return response.WithCookie(response.RedirectTo("/dashboard"), cookie), nil
}

func Logout(context.Context, form.Values) (response.Response, error) {
	sessions, err := Sessions()
	if err != nil {
		return response.Response{}, err
	}
	return response.WithCookie(response.RedirectTo("/"), sessions.ClearCookie()), nil
}

func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	sessions, err := Sessions()
	if err != nil {
		return nil, err
	}
	principal, err := sessions.Principal(ctx.Request)
	if err != nil {
		return nil, err
	}
	if principal == nil {
		return nil, fmt.Errorf("dashboard load requires an authenticated session")
	}
	return map[string]any{
		"dashboard": DashboardData{
			Email:     principal.ID,
			Role:      "user",
			ExpiresAt: time.Now().Add(12 * time.Hour).Format(time.RFC822),
		},
	}, nil
}

func RequireSession(ctx guard.Context) error {
	sessions, err := Sessions()
	if err != nil {
		return err
	}
	principal, err := sessions.Principal(ctx.Request)
	if err != nil {
		return err
	}
	if principal == nil {
		return guard.RedirectTo("/?login=required")
	}
	return nil
}

func MustAuthProvider() gowdkauth.Provider {
	sessions, err := Sessions()
	if err != nil {
		panic(err)
	}
	return sessions.Provider()
}

func Sessions() (*gowdkauth.Sessions, error) {
	sessionState.Lock()
	defer sessionState.Unlock()
	if sessionState.manager != nil || sessionState.err != nil {
		return sessionState.manager, sessionState.err
	}
	sessionState.manager, sessionState.err = gowdkauth.New(gowdkauth.Options{
		SecretEnv:  sessionSecretEnv,
		CookieName: sessionCookie,
		TTL:        12 * time.Hour,
		Insecure:   !secureCookie(),
	})
	return sessionState.manager, sessionState.err
}

func demoPasswordHash() (string, error) {
	passwordState.Lock()
	defer passwordState.Unlock()
	if passwordState.hash != "" || passwordState.err != nil {
		return passwordState.hash, passwordState.err
	}
	passwordState.hash, passwordState.err = passwordHasher.HashPassword(env("GOWDK_AUTH_GUARD_PASSWORD", "demo-password"))
	return passwordState.hash, passwordState.err
}

func constantEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func secureCookie() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("GOWDK_COOKIE_SECURE")), "true")
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
