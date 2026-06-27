package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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

const (
	// SessionModeSignedCookie stores the full principal in the signed cookie.
	// It has no server-side revocation lookup and remains intended for bounded
	// development, tests, and simple deployments that accept that tradeoff.
	SessionModeSignedCookie SessionMode = "signed-cookie"
	// SessionModeRevocable stores only a signed session pointer in the cookie and
	// resolves the current principal from SessionStore on every request.
	SessionModeRevocable SessionMode = "revocable"
)

// ErrNoSession reports that a request carries no readable session cookie.
var ErrNoSession = errors.New("gowdk auth: no session")

var (
	defaultSessionsMu sync.RWMutex
	defaultSessions   *Sessions
	defaultOptions    Options
)

// errBadSession reports a present-but-invalid (tampered, malformed, or expired)
// session. It is intentionally unexported and collapses to a nil principal so
// callers treat any unreadable cookie as simply unauthenticated.
var errBadSession = errors.New("gowdk auth: invalid session")

// SessionMode describes how session cookies are interpreted.
type SessionMode string

// SigningKey configures a session signing key. Previous keys are accepted only
// until AcceptUntil, when set.
type SigningKey struct {
	ID          string
	Secret      []byte
	AcceptUntil time.Time
}

// Options configures a Sessions manager. Secret or SecretEnv is required;
// everything else has a working default.
type Options struct {
	// Secret signs session payloads with HMAC-SHA256. It must be non-empty and
	// should be high-entropy and stable across instances.
	Secret []byte
	// SecretEnv names the environment variable to read instead of Secret. Error
	// messages include this name, never the secret value.
	SecretEnv string
	// KeyID names the current signing key. When set, issued cookies include this
	// ID so future managers can accept it through PreviousKeys during rotation.
	KeyID string
	// PreviousKeys are accepted for verification only. Use them for bounded
	// signing-key rotation, then remove them after AcceptUntil passes.
	PreviousKeys []SigningKey
	// Mode selects the session strategy. The zero value is signed-cookie mode.
	Mode SessionMode
	// Store is required in revocable mode and is consulted on every protected
	// request before returning a principal.
	Store SessionStore
	// CookieName overrides DefaultSessionCookie.
	CookieName string
	// TTL overrides DefaultSessionTTL.
	TTL time.Duration
	// IdleTTL optionally expires revocable sessions when they are unused for this
	// duration. The absolute TTL still applies.
	IdleTTL time.Duration
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
	currentKey  SigningKey
	previousKey map[string]SigningKey
	mode        SessionMode
	store       SessionStore
	cookie      string
	ttl         time.Duration
	idleTTL     time.Duration
	insecure    bool
	now         func() time.Time
}

// sessionPayload is the JSON body carried inside the signed cookie.
type sessionPayload struct {
	ID                   string   `json:"id,omitempty"`
	SessionID            string   `json:"sid,omitempty"`
	Roles                []string `json:"roles,omitempty"`
	Permissions          []string `json:"perms,omitempty"`
	AuthorizationVersion string   `json:"av,omitempty"`
	Expires              int64    `json:"exp"`
}

// New creates a Sessions manager. It returns an error when no secret is set.
func New(options Options) (*Sessions, error) {
	currentKey, previousKeys, err := sessionSigningKeys(options)
	if err != nil {
		return nil, err
	}
	mode := options.Mode
	if mode == "" {
		mode = SessionModeSignedCookie
	}
	switch mode {
	case SessionModeSignedCookie:
	case SessionModeRevocable:
		if options.Store == nil {
			return nil, fmt.Errorf("gowdk auth: revocable session mode requires Store")
		}
	default:
		return nil, fmt.Errorf("gowdk auth: unsupported session mode %q", mode)
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
	idleTTL := options.IdleTTL
	if idleTTL < 0 {
		return nil, fmt.Errorf("gowdk auth: idle session ttl must be non-negative")
	}
	if idleTTL > 0 && idleTTL < time.Second {
		return nil, fmt.Errorf("gowdk auth: idle session ttl must be at least 1s")
	}
	if mode == SessionModeRevocable && idleTTL > 0 {
		if _, ok := options.Store.(SessionToucher); !ok {
			return nil, fmt.Errorf("gowdk auth: idle session ttl requires Store to implement SessionToucher")
		}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Sessions{
		currentKey:  currentKey,
		previousKey: previousKeys,
		mode:        mode,
		store:       options.Store,
		cookie:      cookie,
		ttl:         ttl,
		idleTTL:     idleTTL,
		insecure:    options.Insecure,
		now:         now,
	}, nil
}

// Configure initializes the process-wide default session manager used by
// DefaultSessions and generated auth wiring.
func Configure(options Options) (*Sessions, error) {
	sessions, err := New(options)
	if err != nil {
		return nil, err
	}
	defaultSessionsMu.Lock()
	defer defaultSessionsMu.Unlock()
	defaultSessions = sessions
	defaultOptions = options
	return sessions, nil
}

// DefaultSessions returns the process-wide session manager. When Configure has
// not been called yet, it creates one from DefaultSessionSecretEnv.
func DefaultSessions() (*Sessions, error) {
	defaultSessionsMu.RLock()
	sessions := defaultSessions
	defaultSessionsMu.RUnlock()
	if sessions != nil {
		return sessions, nil
	}
	defaultSessionsMu.Lock()
	defer defaultSessionsMu.Unlock()
	if defaultSessions != nil {
		return defaultSessions, nil
	}
	options := defaultOptions
	if options.SecretEnv == "" && len(options.Secret) == 0 {
		options.SecretEnv = DefaultSessionSecretEnv
	}
	var err error
	defaultSessions, err = New(options)
	if err != nil {
		return nil, err
	}
	defaultOptions = options
	return defaultSessions, nil
}

func sessionSigningKeys(options Options) (SigningKey, map[string]SigningKey, error) {
	secret, err := sessionSecret(options)
	if err != nil {
		return SigningKey{}, nil, err
	}
	current := SigningKey{
		ID:     strings.TrimSpace(options.KeyID),
		Secret: secret,
	}
	if err := validateSigningKeyID("signing key", current.ID); err != nil {
		return SigningKey{}, nil, err
	}
	previous := map[string]SigningKey{}
	for _, key := range options.PreviousKeys {
		key.ID = strings.TrimSpace(key.ID)
		if err := validateSigningKeyID("previous signing key", key.ID); err != nil {
			return SigningKey{}, nil, err
		}
		if len(key.Secret) < MinSessionSecretBytes {
			return SigningKey{}, nil, fmt.Errorf("gowdk auth: previous signing key %q must be at least %d bytes", key.ID, MinSessionSecretBytes)
		}
		copied := make([]byte, len(key.Secret))
		copy(copied, key.Secret)
		key.Secret = copied
		previous[key.ID] = key
	}
	return current, previous, nil
}

func validateSigningKeyID(label string, id string) error {
	if strings.Contains(id, ".") {
		return fmt.Errorf("gowdk auth: %s id %q must not contain dot", label, id)
	}
	return nil
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
	now := sessions.now()
	expires := sessionExpiry(now, sessions.ttl)
	payload := sessionPayload{
		ID:                   principal.ID,
		Roles:                principal.Roles,
		Permissions:          principal.Permissions,
		AuthorizationVersion: principal.AuthorizationVersion,
		Expires:              expires.Unix(),
	}
	if sessions.mode == SessionModeRevocable {
		sessionID, err := randomSessionID()
		if err != nil {
			return http.Cookie{}, err
		}
		payload.SessionID = sessionID
		payload.ID = ""
		payload.Roles = nil
		payload.Permissions = nil
		record := SessionRecord{
			ID:                   sessionID,
			Principal:            clonePrincipal(principal),
			AuthorizationVersion: principal.AuthorizationVersion,
			CreatedAt:            now,
			ExpiresAt:            expires,
			LastSeenAt:           now,
		}
		if sessions.idleTTL > 0 {
			record.IdleExpiresAt = sessionExpiry(now, sessions.idleTTL)
		}
		if err := sessions.store.CreateSession(context.Background(), record); err != nil {
			return http.Cookie{}, err
		}
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

// ClearRequest revokes the current server-side session in revocable mode, then
// writes an expired cookie. Signed-cookie mode only writes the expired cookie.
func (sessions *Sessions) ClearRequest(writer http.ResponseWriter, request *http.Request) error {
	var revokeErr error
	if request != nil && sessions.mode == SessionModeRevocable {
		revokeErr = sessions.Revoke(request.Context(), request)
	}
	sessions.Clear(writer)
	return revokeErr
}

// Rotate revokes the current revocable session, then issues a fresh cookie for
// principal. Applications should call this after authentication and sensitive
// authorization changes.
func (sessions *Sessions) Rotate(writer http.ResponseWriter, request *http.Request, principal Principal) error {
	if request != nil && sessions.mode == SessionModeRevocable {
		if err := sessions.Revoke(request.Context(), request); err != nil {
			return err
		}
	}
	return sessions.Issue(writer, principal)
}

// Revoke invalidates the session presented by request in revocable mode.
func (sessions *Sessions) Revoke(ctx context.Context, request *http.Request) error {
	if sessions.mode != SessionModeRevocable || request == nil {
		return nil
	}
	cookie, ok := sessions.requestCookie(request)
	if !ok {
		return nil
	}
	payload, ok := sessions.verifiedPayload(cookie.Value)
	if !ok {
		return nil
	}
	if strings.TrimSpace(payload.SessionID) == "" {
		return nil
	}
	return sessions.store.RevokeSession(sessionContext(ctx), payload.SessionID)
}

// RevokeSession invalidates one revocable session ID.
func (sessions *Sessions) RevokeSession(ctx context.Context, sessionID string) error {
	if sessions.mode != SessionModeRevocable {
		return nil
	}
	return sessions.store.RevokeSession(sessionContext(ctx), sessionID)
}

// RevokePrincipal invalidates all sessions for principalID in the configured
// revocable store.
func (sessions *Sessions) RevokePrincipal(ctx context.Context, principalID string) error {
	if sessions.mode != SessionModeRevocable {
		return nil
	}
	return sessions.store.RevokePrincipal(sessionContext(ctx), principalID)
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
		return unauthenticatedPrincipal()
	}
	cookie, ok := sessions.requestCookie(request)
	if !ok {
		return unauthenticatedPrincipal()
	}
	payload, ok := sessions.verifiedPayload(cookie.Value)
	if !ok {
		return unauthenticatedPrincipal()
	}
	if sessions.now().Unix() >= payload.Expires {
		return unauthenticatedPrincipal()
	}
	if sessions.mode == SessionModeRevocable {
		return sessions.revocablePrincipal(request.Context(), payload)
	}
	if strings.TrimSpace(payload.ID) == "" {
		return unauthenticatedPrincipal()
	}
	return &Principal{
		ID:                   payload.ID,
		Roles:                payload.Roles,
		Permissions:          payload.Permissions,
		AuthorizationVersion: payload.AuthorizationVersion,
	}, nil
}

func (sessions *Sessions) revocablePrincipal(ctx context.Context, payload sessionPayload) (*Principal, error) {
	if strings.TrimSpace(payload.SessionID) == "" {
		return unauthenticatedPrincipal()
	}
	record, err := sessions.store.LookupSession(ctx, payload.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return unauthenticatedPrincipal()
		}
		return nil, err
	}
	now := sessions.now()
	if record.Revoked || record.expired(now) || strings.TrimSpace(record.Principal.ID) == "" {
		return unauthenticatedPrincipal()
	}
	if sessionRecordAuthorizationVersion(record) != payload.AuthorizationVersion {
		return unauthenticatedPrincipal()
	}
	if sessions.idleTTL > 0 {
		toucher, ok := sessions.store.(SessionToucher)
		if ok {
			if err := toucher.TouchSession(ctx, record.ID, now, sessionExpiry(now, sessions.idleTTL)); err != nil {
				return nil, err
			}
		}
	}
	principal := clonePrincipal(record.Principal)
	if principal.AuthorizationVersion == "" {
		principal.AuthorizationVersion = record.AuthorizationVersion
	}
	return &principal, nil
}

func unauthenticatedPrincipal() (*Principal, error) {
	var principal *Principal
	return principal, nil
}

func (sessions *Sessions) requestCookie(request *http.Request) (*http.Cookie, bool) {
	cookie, err := request.Cookie(sessions.cookie)
	if err != nil {
		return nil, false
	}
	return cookie, true
}

func (sessions *Sessions) verifiedPayload(token string) (sessionPayload, bool) {
	payload, err := sessions.verify(token)
	if err != nil {
		return sessionPayload{}, false
	}
	return payload, true
}

func sessionRecordAuthorizationVersion(record SessionRecord) string {
	if record.Principal.AuthorizationVersion != "" {
		return record.Principal.AuthorizationVersion
	}
	return record.AuthorizationVersion
}

// Provider returns sessions typed as a Provider for registration with the
// generated RegisterAuthProvider hook.
func (sessions *Sessions) Provider() Provider {
	return sessions
}

// Mode reports the configured session strategy.
func (sessions *Sessions) Mode() SessionMode {
	return sessions.mode
}

// sign encodes payload and appends an HMAC-SHA256 tag:
// <b64payload>.<b64tag> for legacy unnamed keys, or
// <key-id>.<b64payload>.<b64tag> when KeyID is configured.
func (sessions *Sessions) sign(payload []byte) string {
	key := sessions.currentKey
	mac := hmac.New(sha256.New, key.Secret)
	mac.Write(payload)
	tag := mac.Sum(nil)
	encoded := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(tag)
	if key.ID == "" {
		return encoded
	}
	return key.ID + "." + encoded
}

// verify checks the HMAC tag in constant time and returns the decoded payload.
func (sessions *Sessions) verify(token string) (sessionPayload, error) {
	parts := strings.Split(token, ".")
	var body string
	var tag string
	var keys []SigningKey
	switch len(parts) {
	case 2:
		body = parts[0]
		tag = parts[1]
		keys = sessions.unnamedSigningKeys()
	case 3:
		keyID := strings.TrimSpace(parts[0])
		key, ok := sessions.namedSigningKey(keyID)
		if !ok {
			return sessionPayload{}, errBadSession
		}
		body = parts[1]
		tag = parts[2]
		keys = []SigningKey{key}
	default:
		return sessionPayload{}, errBadSession
	}
	if len(keys) == 0 {
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
	for _, key := range keys {
		mac := hmac.New(sha256.New, key.Secret)
		mac.Write(payload)
		wantTag := mac.Sum(nil)
		if subtle.ConstantTimeCompare(gotTag, wantTag) != 1 {
			continue
		}
		var decoded sessionPayload
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return sessionPayload{}, errBadSession
		}
		return decoded, nil
	}
	return sessionPayload{}, errBadSession
}

func (sessions *Sessions) namedSigningKey(keyID string) (SigningKey, bool) {
	if keyID == "" {
		return SigningKey{}, false
	}
	if keyID == sessions.currentKey.ID {
		return sessions.currentKey, true
	}
	key, ok := sessions.previousKey[keyID]
	if !ok || !sessions.previousSigningKeyAccepted(key) {
		return SigningKey{}, false
	}
	return key, true
}

func (sessions *Sessions) unnamedSigningKeys() []SigningKey {
	var keys []SigningKey
	if sessions.currentKey.ID == "" {
		keys = append(keys, sessions.currentKey)
	}
	if key, ok := sessions.previousKey[""]; ok && sessions.previousSigningKeyAccepted(key) {
		keys = append(keys, key)
	}
	return keys
}

func (sessions *Sessions) previousSigningKeyAccepted(key SigningKey) bool {
	return key.AcceptUntil.IsZero() || sessions.now().Before(key.AcceptUntil)
}

func randomSessionID() (string, error) {
	var payload [32]byte
	if _, err := rand.Read(payload[:]); err != nil {
		return "", fmt.Errorf("gowdk auth: generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload[:]), nil
}

func sessionContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
