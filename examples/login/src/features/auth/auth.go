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
	email := strings.TrimSpace(values.First("email"))
	password := values.First("password")
	if !constantEqual(email, env("GOWDK_LOGIN_EMAIL", "demo@example.com")) ||
		!constantEqual(password, env("GOWDK_LOGIN_PASSWORD", "demo-password")) {
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
		Secure:   env("GOWDK_COOKIE_SECURE", "false") == "true",
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
		Secure:   env("GOWDK_COOKIE_SECURE", "false") == "true",
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
	mac := hmac.New(sha256.New, []byte(env("GOWDK_LOGIN_SECRET", "development-login-secret-change-me")))
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
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
