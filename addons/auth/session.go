package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultSessionCookie is the cookie name used for signed sessions.
	DefaultSessionCookie = "gowdk_session"
	// DefaultSessionTTL is how long an issued session remains valid.
	DefaultSessionTTL = 24 * time.Hour
	// DefaultSessionSecretEnv is the recommended runtime environment variable
	// for session signing secrets.
	DefaultSessionSecretEnv = "GOWDK_AUTH_SESSION_SECRET"
	// MinSessionSecretBytes is the minimum accepted session signing secret
	// length.
	MinSessionSecretBytes = 32
)

// ErrNoSession reports that a request carries no readable session cookie.
var ErrNoSession = errors.New("gowdk auth: no session")

// errBadSession reports a present-but-invalid (tampered, malformed, or expired)
// session. It is intentionally unexported and collapses to a nil principal so
// callers treat any unreadable cookie as simply unauthenticated.
var errBadSession = errors.New("gowdk auth: invalid session")

// Options configures a Sessions manager. Secret or SecretEnv is required;
// everything else has a working default.
type Options struct {
	// Secret signs session payloads with HMAC-SHA256. It must be non-empty and
	// should be high-entropy and stable across instances.
	Secret []byte
	// SecretEnv names the environment variable to read instead of Secret. Error
	// messages include this name, never the secret value.
	SecretEnv string
	// CookieName overrides DefaultSessionCookie.
	CookieName string
	// TTL overrides DefaultSessionTTL.
	TTL time.Duration
	// Insecure drops the Secure cookie flag for local HTTP development. Leave
	// false in production so the cookie is only sent over HTTPS.
	Insecure bool
	// Now overrides the clock, for tests.
	Now func() time.Time
}

// Sessions issues and reads signed-cookie sessions and resolves the current
// Principal for a request. The zero value is not usable; construct one with
// New. Sessions implements Provider.
type Sessions struct {
	secret   []byte
	cookie   string
	ttl      time.Duration
	insecure bool
	now      func() time.Time
}

// sessionPayload is the JSON body carried inside the signed cookie.
type sessionPayload struct {
	ID          string   `json:"id"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"perms,omitempty"`
	Expires     int64    `json:"exp"`
}

// New creates a Sessions manager. It returns an error when no secret is set.
func New(options Options) (*Sessions, error) {
	secret, err := sessionSecret(options)
	if err != nil {
		return nil, err
	}
	cookie := options.CookieName
	if cookie == "" {
		cookie = DefaultSessionCookie
	} else {
		probe := http.Cookie{Name: cookie, Value: "session"}
		if err := probe.Valid(); err != nil {
			return nil, fmt.Errorf("gowdk auth: invalid session cookie name %q: %w", cookie, err)
		}
	}
	ttl := options.TTL
	if ttl < 0 {
		return nil, fmt.Errorf("gowdk auth: session ttl must be non-negative")
	}
	if ttl > 0 && ttl < time.Second {
		return nil, fmt.Errorf("gowdk auth: session ttl must be at least 1s")
	}
	if ttl == 0 {
		ttl = DefaultSessionTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Sessions{
		secret:   secret,
		cookie:   cookie,
		ttl:      ttl,
		insecure: options.Insecure,
		now:      now,
	}, nil
}

func sessionSecret(options Options) ([]byte, error) {
	envName := strings.TrimSpace(options.SecretEnv)
	if len(options.Secret) > 0 && envName != "" {
		return nil, fmt.Errorf("gowdk auth: set Secret or SecretEnv, not both")
	}
	if len(options.Secret) > 0 {
		if len(options.Secret) < MinSessionSecretBytes {
			return nil, fmt.Errorf("gowdk auth: session secret must be at least %d bytes", MinSessionSecretBytes)
		}
		secret := make([]byte, len(options.Secret))
		copy(secret, options.Secret)
		return secret, nil
	}
	if envName == "" {
		return nil, fmt.Errorf("gowdk auth: session secret is required")
	}
	value := os.Getenv(envName)
	if value == "" {
		return nil, fmt.Errorf("gowdk auth: %s is required for session signing", envName)
	}
	if len([]byte(value)) < MinSessionSecretBytes {
		return nil, fmt.Errorf("gowdk auth: %s must be at least %d bytes", envName, MinSessionSecretBytes)
	}
	return []byte(value), nil
}

// Issue writes a signed session cookie for principal to the response.
func (sessions *Sessions) Issue(writer http.ResponseWriter, principal Principal) error {
	if writer == nil {
		return fmt.Errorf("gowdk auth: response writer is required")
	}
	cookie, err := sessions.Cookie(principal)
	if err != nil {
		return err
	}
	http.SetCookie(writer, &cookie)
	return nil
}

// Cookie creates a signed session cookie for principal.
func (sessions *Sessions) Cookie(principal Principal) (http.Cookie, error) {
	if strings.TrimSpace(principal.ID) == "" {
		return http.Cookie{}, fmt.Errorf("gowdk auth: principal id is required")
	}
	expires := sessionExpiry(sessions.now(), sessions.ttl)
	payload := sessionPayload{
		ID:          principal.ID,
		Roles:       principal.Roles,
		Permissions: principal.Permissions,
		Expires:     expires.Unix(),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return http.Cookie{}, fmt.Errorf("gowdk auth: encode session: %w", err)
	}
	token := sessions.sign(encoded)
	return http.Cookie{
		Name:     sessions.cookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   !sessions.insecure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

func sessionExpiry(now time.Time, ttl time.Duration) time.Time {
	expires := now.Add(ttl)
	if expires.Nanosecond() == 0 {
		return expires
	}
	return expires.Truncate(time.Second).Add(time.Second)
}

// Clear writes an immediately-expired session cookie, logging the request out.
func (sessions *Sessions) Clear(writer http.ResponseWriter) {
	if writer == nil {
		return
	}
	cookie := sessions.ClearCookie()
	http.SetCookie(writer, &cookie)
}

// ClearCookie creates an immediately-expired session cookie.
func (sessions *Sessions) ClearCookie() http.Cookie {
	return http.Cookie{
		Name:     sessions.cookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   !sessions.insecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Principal resolves the current principal from the request's session cookie.
// A request with no cookie, or a tampered or expired one, yields a nil
// principal and no error, meaning unauthenticated.
func (sessions *Sessions) Principal(request *http.Request) (*Principal, error) {
	if request == nil {
		return nil, nil
	}
	cookie, err := request.Cookie(sessions.cookie)
	if err != nil {
		return nil, nil
	}
	payload, err := sessions.verify(cookie.Value)
	if err != nil {
		return nil, nil
	}
	if sessions.now().Unix() >= payload.Expires {
		return nil, nil
	}
	if strings.TrimSpace(payload.ID) == "" {
		return nil, nil
	}
	return &Principal{
		ID:          payload.ID,
		Roles:       payload.Roles,
		Permissions: payload.Permissions,
	}, nil
}

// Provider returns sessions typed as a Provider for registration with the
// generated RegisterAuthProvider hook.
func (sessions *Sessions) Provider() Provider {
	return sessions
}

// sign encodes payload and appends an HMAC-SHA256 tag: <b64payload>.<b64tag>.
func (sessions *Sessions) sign(payload []byte) string {
	mac := hmac.New(sha256.New, sessions.secret)
	mac.Write(payload)
	tag := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(tag)
}

// verify checks the HMAC tag in constant time and returns the decoded payload.
func (sessions *Sessions) verify(token string) (sessionPayload, error) {
	body, tag, found := strings.Cut(token, ".")
	if !found {
		return sessionPayload{}, errBadSession
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return sessionPayload{}, errBadSession
	}
	gotTag, err := base64.RawURLEncoding.DecodeString(tag)
	if err != nil {
		return sessionPayload{}, errBadSession
	}
	mac := hmac.New(sha256.New, sessions.secret)
	mac.Write(payload)
	wantTag := mac.Sum(nil)
	if subtle.ConstantTimeCompare(gotTag, wantTag) != 1 {
		return sessionPayload{}, errBadSession
	}
	var decoded sessionPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return sessionPayload{}, errBadSession
	}
	return decoded, nil
}
