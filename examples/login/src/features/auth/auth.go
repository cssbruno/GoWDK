package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

const sessionCookie = "gowdk_login_session"

type LoginViewState struct {
	DemoEmail string
}

func NewLoginViewState() LoginViewState {
	return LoginViewState{DemoEmail: env("GOWDK_LOGIN_EMAIL", "demo@example.com")}
}

func Login(_ context.Context, values form.Values) (response.Response, error) {
	// Fail closed: refuse to authenticate unless the signing secret and the
	// demo password are configured. Without a configured secret an attacker
	// could forge session cookies; without a configured password an empty
	// submitted password would match an empty fallback.
	if len(loginSecret()) == 0 {
		return response.RedirectTo("/login/error"), nil
	}
	wantEmail, wantPassword, ok := configuredCredentials()
	if !ok {
		return response.RedirectTo("/login/error"), nil
	}

	email := strings.TrimSpace(values.First("email"))
	password := values.First("password")
	if !constantEqual(email, wantEmail) || !constantEqual(password, wantPassword) {
		return response.RedirectTo("/login/error"), nil
	}

	sessionID := randomToken()
	sessions.Lock()
	sessions.Values[sessionID] = session{
		Email:     email,
		ExpiresAt: time.Now().Add(sessionDuration()),
	}
	sessions.Unlock()

	return response.WithCookie(response.RedirectTo("/dashboard"), http.Cookie{
		Name:     sessionCookie,
		Value:    sign(sessionID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookies(),
		MaxAge:   int(sessionDuration().Seconds()),
	}), nil
}

func Logout(context.Context, form.Values) (response.Response, error) {
	return response.WithCookie(response.RedirectTo("/"), http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookies(),
	}), nil
}

func Session(_ context.Context, request *http.Request) (response.Response, error) {
	current, ok := currentSession(request)
	if !ok {
		return response.JSONValue(http.StatusUnauthorized, map[string]any{"authenticated": false})
	}
	return response.JSONValue(http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         current.Email,
		"expires_at":    current.ExpiresAt.Format(time.RFC3339),
	})
}

type session struct {
	Email     string
	ExpiresAt time.Time
}

var sessions = struct {
	sync.Mutex
	Values map[string]session
}{Values: map[string]session{}}

func currentSession(request *http.Request) (session, bool) {
	if len(loginSecret()) == 0 {
		return session{}, false
	}
	cookie, err := request.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	id, sig, ok := strings.Cut(cookie.Value, ".")
	if !ok || id == "" || sig == "" || !constantEqual(sig, signature(id)) {
		return session{}, false
	}
	sessions.Lock()
	defer sessions.Unlock()
	current, ok := sessions.Values[id]
	if !ok || time.Now().After(current.ExpiresAt) {
		delete(sessions.Values, id)
		return session{}, false
	}
	return current, true
}

func sign(value string) string {
	return value + "." + signature(value)
}

func signature(value string) string {
	mac := hmac.New(sha256.New, loginSecret())
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// loginSecret returns the HMAC signing key from the environment with no
// literal fallback. An unset secret means the example refuses to issue or
// accept sessions, so a publicly known key can never sign forgeable cookies.
func loginSecret() []byte {
	return []byte(strings.TrimSpace(os.Getenv("GOWDK_LOGIN_SECRET")))
}

// configuredCredentials returns the demo login email and password. The email
// keeps a non-secret default for the demo UI, but the password must be set
// explicitly so there is no hardcoded credential and an empty submitted
// password cannot match an empty fallback.
func configuredCredentials() (email, password string, ok bool) {
	email = env("GOWDK_LOGIN_EMAIL", "demo@example.com")
	password = strings.TrimSpace(os.Getenv("GOWDK_LOGIN_PASSWORD"))
	return email, password, password != ""
}

// secureCookies reports whether the session cookie should carry the Secure
// flag. It defaults to true; set GOWDK_COOKIE_INSECURE=true only for local
// HTTP development.
func secureCookies() bool {
	return strings.TrimSpace(os.Getenv("GOWDK_COOKIE_INSECURE")) != "true"
}

func sessionDuration() time.Duration {
	return 12 * time.Hour
}

func constantEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func randomToken() string {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("random token: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw[:])
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
