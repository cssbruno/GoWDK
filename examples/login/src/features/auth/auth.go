package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultBackendOrigin  = "http://127.0.0.1:8091"
	defaultFrontendOrigin = "http://127.0.0.1:8090"
	sessionCookie         = "gowdk_login_session"
)

type LoginViewState struct {
	BackendOrigin string
	LoginAction   string
	SessionAPI    string
	LogoutAction  string
	DemoEmail     string
}

func NewLoginViewState() LoginViewState {
	backendOrigin := env("GOWDK_BACKEND_ORIGIN", defaultBackendOrigin)
	return LoginViewState{
		BackendOrigin: backendOrigin,
		LoginAction:   backendURL(backendOrigin, "/api/login"),
		SessionAPI:    backendURL(backendOrigin, "/api/session"),
		LogoutAction:  backendURL(backendOrigin, "/api/logout"),
		DemoEmail:     env("GOWDK_LOGIN_EMAIL", "demo@example.com"),
	}
}

type BackendConfig struct {
	Addr            string
	FrontendOrigin  string
	Email           string
	Password        string
	SessionSecret   string
	SessionDuration time.Duration
	CookieSecure    bool
}

func BackendConfigFromEnv() BackendConfig {
	return BackendConfig{
		Addr:            env("GOWDK_BACKEND_ADDR", "127.0.0.1:8091"),
		FrontendOrigin:  env("GOWDK_FRONTEND_ORIGIN", defaultFrontendOrigin),
		Email:           env("GOWDK_LOGIN_EMAIL", "demo@example.com"),
		Password:        env("GOWDK_LOGIN_PASSWORD", "demo-password"),
		SessionSecret:   env("GOWDK_LOGIN_SECRET", "development-login-secret-change-me"),
		SessionDuration: 12 * time.Hour,
		CookieSecure:    env("GOWDK_COOKIE_SECURE", "false") == "true",
	}
}

func RunBackendFromEnv() error {
	cfg := BackendConfigFromEnv()
	handler := NewBackendHandler(cfg)

	log.Printf("serving login backend API at http://%s", cfg.Addr)
	log.Printf("allowing frontend origin %s", cfg.FrontendOrigin)
	return http.ListenAndServe(cfg.Addr, handler)
}

func NewBackendHandler(cfg BackendConfig) http.Handler {
	app := &backendServer{
		config:   cfg,
		sessions: map[string]session{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /_backend/health", app.health)
	mux.HandleFunc("GET /api/session", app.sessionAPI)
	mux.HandleFunc("POST /api/login", app.loginAPI)
	mux.HandleFunc("POST /api/logout", app.logoutAPI)
	return app.middleware(mux)
}

type backendServer struct {
	config BackendConfig

	mu       sync.Mutex
	sessions map[string]session
}

type session struct {
	Email     string
	ExpiresAt time.Time
}

func (app *backendServer) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "login-backend"})
}

func (app *backendServer) loginAPI(w http.ResponseWriter, r *http.Request) {
	if !app.validOrigin(r) {
		app.loginFailed(w, r, http.StatusForbidden, "Request origin rejected.")
		return
	}
	if err := r.ParseForm(); err != nil {
		app.loginFailed(w, r, http.StatusBadRequest, "Invalid login request.")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	if !constantEqual(email, app.config.Email) || !constantEqual(password, app.config.Password) {
		app.loginFailed(w, r, http.StatusUnauthorized, "Invalid email or password.")
		return
	}

	sessionID := randomToken()
	app.mu.Lock()
	app.sessions[sessionID] = session{
		Email:     email,
		ExpiresAt: time.Now().Add(app.config.SessionDuration),
	}
	app.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    app.sign(sessionID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.config.CookieSecure || r.TLS != nil,
		MaxAge:   int(app.config.SessionDuration.Seconds()),
	})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]string{"redirect": app.frontendURL("/dashboard")})
		return
	}
	http.Redirect(w, r, app.frontendURL("/dashboard"), http.StatusSeeOther)
}

func (app *backendServer) logoutAPI(w http.ResponseWriter, r *http.Request) {
	if !app.validOrigin(r) {
		http.Error(w, "request origin rejected", http.StatusForbidden)
		return
	}
	if id, ok := app.sessionID(r); ok {
		app.mu.Lock()
		delete(app.sessions, id)
		app.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.config.CookieSecure || r.TLS != nil,
	})
	http.Redirect(w, r, app.frontendURL("/"), http.StatusSeeOther)
}

func (app *backendServer) sessionAPI(w http.ResponseWriter, r *http.Request) {
	current, ok := app.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         current.Email,
		"expires_at":    current.ExpiresAt.Format(time.RFC3339),
	})
}

func (app *backendServer) loginFailed(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsJSON(r) {
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	http.Redirect(w, r, app.frontendURL("/login/error"), http.StatusSeeOther)
}

func (app *backendServer) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host) || strings.EqualFold(originOf(parsed), app.config.FrontendOrigin)
}

func (app *backendServer) currentSession(r *http.Request) (session, bool) {
	id, ok := app.sessionID(r)
	if !ok {
		return session{}, false
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	current, ok := app.sessions[id]
	if !ok || time.Now().After(current.ExpiresAt) {
		delete(app.sessions, id)
		return session{}, false
	}
	return current, true
}

func (app *backendServer) sessionID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	id, sig, ok := strings.Cut(cookie.Value, ".")
	if !ok || id == "" || sig == "" || !constantEqual(sig, app.signature(id)) {
		return "", false
	}
	return id, true
}

func (app *backendServer) sign(value string) string {
	return value + "." + app.signature(value)
}

func (app *backendServer) signature(value string) string {
	mac := hmac.New(sha256.New, []byte(app.config.SessionSecret))
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (app *backendServer) frontendURL(targetPath string) string {
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	return strings.TrimRight(app.config.FrontendOrigin, "/") + targetPath
}

func (app *backendServer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Access-Control-Allow-Origin", app.config.FrontendOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func backendURL(origin string, targetPath string) string {
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	return strings.TrimRight(origin, "/") + targetPath
}

func originOf(parsed *url.URL) string {
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
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
