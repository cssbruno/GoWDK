package flagship

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/guard"
	"github.com/cssbruno/gowdk/runtime/response"
	"github.com/cssbruno/gowdk/runtime/ssr"
)

const sessionCookie = "gowdk_flagship_session"

func Login(_ context.Context, values form.Values) (response.Response, error) {
	email := strings.TrimSpace(values.First("email"))
	password := values.First("password")
	if !constantEqual(email, env("GOWDK_FLAGSHIP_EMAIL", "demo@example.com")) ||
		!constantEqual(password, env("GOWDK_FLAGSHIP_PASSWORD", "demo-password")) {
		return response.RedirectTo("/?login=failed"), nil
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

func RefreshSummary(_ context.Context, values form.Values) (response.Response, error) {
	topic := strings.TrimSpace(values.First("topic"))
	if topic == "" {
		topic = "compiler"
	}
	return response.FragmentFor("#summary", summaryHTML(topic, time.Now())), nil
}

func Status(_ context.Context, request *http.Request) (response.Response, error) {
	payload := map[string]any{
		"ok":        true,
		"surface":   "api",
		"method":    request.Method,
		"path":      request.URL.Path,
		"generated": "gowdk",
	}
	return response.JSONValue(http.StatusOK, payload)
}

func RequireSession(ctx guard.Context) error {
	if _, ok := currentSession(ctx.Request); ok {
		return nil
	}
	return guard.RedirectTo("/?login=required")
}

func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	current, ok := currentSession(ctx.Request)
	if !ok {
		return map[string]any{
			"dashboard": DashboardData{
				Title:      "Dashboard unavailable",
				Email:      "anonymous",
				Expires:    "not signed in",
				QueueDepth: 0,
			},
		}, nil
	}
	return map[string]any{
		"dashboard": DashboardData{
			Title:      "Protected dashboard",
			Email:      current.Email,
			Expires:    current.ExpiresAt.Format(time.RFC822),
			QueueDepth: 3,
		},
	}, nil
}

type DashboardData struct {
	Title      string `json:"title"`
	Email      string `json:"email"`
	Expires    string `json:"expires"`
	QueueDepth int    `json:"queueDepth"`
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
	if request == nil {
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

func summaryHTML(topic string, refreshed time.Time) string {
	escaped := html.EscapeString(topic)
	return fmt.Sprintf(`<section id="summary" class="summary-fragment"><h2>%s summary</h2><p>Updated by a generated partial action at %s.</p></section>`, escaped, refreshed.Format(time.Kitchen))
}

func sign(value string) string {
	return value + "." + signature(value)
}

func signature(value string) string {
	mac := hmac.New(sha256.New, []byte(env("GOWDK_FLAGSHIP_SECRET", "development-flagship-secret-change-me")))
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
