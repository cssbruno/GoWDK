package auth

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
)

func TestAddonEnablesAuthFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "auth" {
		t.Fatalf("addon.Name() = %q, want auth", addon.Name())
	}
	config := gowdk.Config{Addons: []gowdk.Addon{addon}}
	if !config.HasFeature(gowdk.FeatureAuth) {
		t.Fatal("expected auth feature to be enabled")
	}
}

func TestAddonExposesGeneratedSessionOptions(t *testing.T) {
	addon := Addon(Options{
		SecretEnv:  "GOWDK_TEST_AUTH_SECRET",
		CookieName: "site_session",
		TTL:        2 * time.Hour,
		Insecure:   true,
	})
	provider, ok := addon.(gowdk.AuthSessionProvider)
	if !ok {
		t.Fatalf("expected auth addon to implement AuthSessionProvider, got %T", addon)
	}
	options := provider.AuthSessionOptions()
	if options.SecretEnv != "GOWDK_TEST_AUTH_SECRET" || options.CookieName != "site_session" || options.TTL != 2*time.Hour || !options.Insecure {
		t.Fatalf("unexpected auth session options: %#v", options)
	}
}

func TestAddonDefaultSessionOptionsUseDefaultSecretEnv(t *testing.T) {
	provider, ok := Addon().(gowdk.AuthSessionProvider)
	if !ok {
		t.Fatalf("expected auth addon to implement AuthSessionProvider")
	}
	options := provider.AuthSessionOptions()
	if options.SecretEnv != DefaultSessionSecretEnv {
		t.Fatalf("expected default secret env %q, got %#v", DefaultSessionSecretEnv, options)
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword("correct horse battery staple", encoded) {
		t.Fatal("VerifyPassword rejected the correct password")
	}
	if VerifyPassword("wrong password", encoded) {
		t.Fatal("VerifyPassword accepted a wrong password")
	}
}

func TestPBKDF2HasherWithCustomIterations(t *testing.T) {
	iterations := DefaultIterations + 1
	hasher := PBKDF2Hasher{Iterations: iterations}
	encoded, err := hasher.HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "pbkdf2-sha256$"+strconv.Itoa(iterations)+"$") {
		t.Fatalf("expected encoded iterations, got %q", encoded)
	}
	if !hasher.VerifyPassword("same", encoded) {
		t.Fatal("VerifyPassword rejected the correct password")
	}
}

func TestHashPasswordWithIterationsRejectsInvalidIterations(t *testing.T) {
	for _, iterations := range []int{0, -1, MinIterations - 1, MaxIterations + 1} {
		if _, err := HashPasswordWithIterations("same", iterations); err == nil {
			t.Fatalf("HashPasswordWithIterations accepted iterations=%d", iterations)
		}
	}
	if _, err := (PBKDF2Hasher{Iterations: MaxIterations + 1}).HashPassword("same"); err == nil {
		t.Fatalf("PBKDF2Hasher accepted iterations=%d", MaxIterations+1)
	}
}

func TestPBKDF2KnownVector(t *testing.T) {
	key, err := pbkdf2SHA256("password", []byte("salt"), 1, 32)
	if err != nil {
		t.Fatalf("pbkdf2SHA256: %v", err)
	}
	if got, want := hex.EncodeToString(key), "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"; got != want {
		t.Fatalf("unexpected PBKDF2 key: got %s want %s", got, want)
	}
}

type fixedPasswordHasher struct {
	hash string
}

func (hasher fixedPasswordHasher) HashPassword(string) (string, error) {
	return hasher.hash, nil
}

func (hasher fixedPasswordHasher) VerifyPassword(_, encoded string) bool {
	return encoded == hasher.hash
}

func TestCustomPasswordHasherSatisfiesInterface(t *testing.T) {
	var hasher PasswordHasher = fixedPasswordHasher{hash: "custom"}
	encoded, err := hasher.HashPassword("ignored")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !hasher.VerifyPassword("ignored", encoded) {
		t.Fatal("custom hasher did not verify its encoded hash")
	}
}

func TestHashPasswordUsesFreshSalt(t *testing.T) {
	first, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if first == second {
		t.Fatal("expected distinct hashes for the same password (salt not random)")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	canonicalSalt := base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("s", pbkdf2SaltLength)))
	canonicalKey := base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("k", pbkdf2KeyLength)))
	shortSalt := base64.RawStdEncoding.EncodeToString([]byte("s"))
	shortKey := base64.RawStdEncoding.EncodeToString([]byte("k"))
	for _, encoded := range []string{
		"",
		"plain",
		"pbkdf2-sha256$notanumber$c2FsdA$a2V5",
		"bcrypt$10$salt$key",
		"pbkdf2-sha256$600000$$a2V5",
		"pbkdf2-sha256$1$" + canonicalSalt + "$" + canonicalKey,
		"pbkdf2-sha256$" + strconv.Itoa(MaxIterations+1) + "$" + canonicalSalt + "$" + canonicalKey,
		"pbkdf2-sha256$600000$" + shortSalt + "$" + canonicalKey,
		"pbkdf2-sha256$600000$" + canonicalSalt + "$" + shortKey,
	} {
		if VerifyPassword("anything", encoded) {
			t.Fatalf("VerifyPassword accepted malformed hash %q", encoded)
		}
	}
}

// fixedClock returns a Now function pinned to t.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func newTestSessions(t *testing.T, now time.Time) *Sessions {
	t.Helper()
	sessions, err := New(Options{
		Secret:   []byte(strings.Repeat("s", MinSessionSecretBytes)),
		Insecure: true,
		Now:      fixedClock(now),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return sessions
}

func TestSessionIssueAndResolve(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	sessions := newTestSessions(t, now)

	recorder := httptest.NewRecorder()
	want := Principal{ID: "user-1", Roles: []string{"admin"}, Permissions: []string{"posts.write"}}
	if err := sessions.Issue(recorder, want); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}

	got, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if got == nil {
		t.Fatal("expected a principal, got nil")
	}
	if got.ID != want.ID || !got.HasRole("admin") || !got.HasPermission("posts.write") {
		t.Fatalf("resolved principal = %+v, want %+v", got, want)
	}
}

func TestConfigureExposesSessionsAndRequiredGuard(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	configured, err := Configure(Options{
		Secret:   []byte(strings.Repeat("s", MinSessionSecretBytes)),
		Insecure: true,
		Now:      fixedClock(now),
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	sessions, err := DefaultSessions()
	if err != nil {
		t.Fatalf("DefaultSessions: %v", err)
	}
	if sessions != configured {
		t.Fatalf("Sessions returned a different manager")
	}

	recorder := httptest.NewRecorder()
	if err := sessions.Issue(recorder, Principal{ID: "user-1"}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	authed := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		authed.AddCookie(cookie)
	}
	if err := RequireAuthenticated(nil)(gowdkguard.NewContext(authed, nil)); err != nil {
		t.Fatalf("RequireAuthenticated rejected signed session: %v", err)
	}

	anonymous := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := RequireAuthenticated(sessions.Provider())(gowdkguard.NewContext(anonymous, nil)); err == nil {
		t.Fatal("RequireAuthenticated accepted anonymous request")
	}
}

func TestSessionResolvesNilWithoutCookie(t *testing.T) {
	sessions := newTestSessions(t, time.Unix(1_700_000_000, 0))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	principal, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal != nil {
		t.Fatalf("expected nil principal without a cookie, got %+v", principal)
	}
}

func TestSessionRejectsTamperedCookie(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	sessions := newTestSessions(t, now)

	recorder := httptest.NewRecorder()
	if err := sessions.Issue(recorder, Principal{ID: "user-1", Roles: []string{"user"}}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := recorder.Result().Cookies()[0]
	// Flip the role inside the signed payload without re-signing.
	cookie.Value = cookie.Value + "x"

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)

	principal, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal != nil {
		t.Fatal("expected nil principal for a tampered cookie")
	}
}

func TestSessionRejectsForeignSecret(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	issuer := newTestSessions(t, now)

	recorder := httptest.NewRecorder()
	if err := issuer.Issue(recorder, Principal{ID: "user-1"}); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	other, err := New(Options{Secret: []byte(strings.Repeat("x", MinSessionSecretBytes)), Insecure: true, Now: fixedClock(now)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	principal, err := other.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal != nil {
		t.Fatal("expected nil principal when verifying with a foreign secret")
	}
}

func TestSessionExpires(t *testing.T) {
	issuedAt := time.Unix(1_700_000_000, 0)
	sessions, err := New(Options{
		Secret:   []byte(strings.Repeat("s", MinSessionSecretBytes)),
		TTL:      time.Hour,
		Insecure: true,
		Now:      fixedClock(issuedAt),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := sessions.Issue(recorder, Principal{ID: "user-1"}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}

	// Advance the clock past the TTL.
	sessions.now = fixedClock(issuedAt.Add(2 * time.Hour))
	principal, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal != nil {
		t.Fatal("expected nil principal after the session expired")
	}
}

func TestSessionOneSecondTTLDoesNotExpireEarlyFromSubsecondIssueTime(t *testing.T) {
	issuedAt := time.Unix(1_700_000_000, int64(900*time.Millisecond))
	sessions, err := New(Options{
		Secret:   []byte(strings.Repeat("s", MinSessionSecretBytes)),
		TTL:      time.Second,
		Insecure: true,
		Now:      fixedClock(issuedAt),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	recorder := httptest.NewRecorder()
	if err := sessions.Issue(recorder, Principal{ID: "user-1"}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}

	sessions.now = fixedClock(issuedAt.Add(200 * time.Millisecond))
	principal, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal == nil {
		t.Fatal("expected one-second session to remain valid after 200ms")
	}
}

func TestSessionClearLogsOut(t *testing.T) {
	sessions := newTestSessions(t, time.Unix(1_700_000_000, 0))
	recorder := httptest.NewRecorder()
	sessions.Clear(recorder)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Fatalf("expected a negative MaxAge to clear the cookie, got %d", cookies[0].MaxAge)
	}
}

func TestSessionCookieDefaultsToSecureAttributes(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	sessions, err := New(Options{
		Secret: []byte(strings.Repeat("s", MinSessionSecretBytes)),
		Now:    fixedClock(now),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cookie, err := sessions.Cookie(Principal{ID: "user-1"})
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if cookie.Name != DefaultSessionCookie || cookie.Path != "/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected default cookie attributes: %#v", cookie)
	}
	if !cookie.Expires.Equal(now.Add(DefaultSessionTTL)) {
		t.Fatalf("cookie expires at %v, want %v", cookie.Expires, now.Add(DefaultSessionTTL))
	}
	clear := sessions.ClearCookie()
	if clear.Name != DefaultSessionCookie || clear.Path != "/" || !clear.HttpOnly || !clear.Secure || clear.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected default clear-cookie attributes: %#v", clear)
	}
}

func TestSessionCookieAllowsLocalInsecure(t *testing.T) {
	sessions := newTestSessions(t, time.Unix(1_700_000_000, 0))
	cookie, err := sessions.Cookie(Principal{ID: "user-1"})
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if cookie.Secure {
		t.Fatal("expected Insecure option to disable the Secure cookie flag")
	}
}

func TestSessionCookieHelpers(t *testing.T) {
	sessions := newTestSessions(t, time.Unix(1_700_000_000, 0))
	cookie, err := sessions.Cookie(Principal{ID: "user-1"})
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if cookie.Name != DefaultSessionCookie || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("unexpected issued cookie: %#v", cookie)
	}
	clear := sessions.ClearCookie()
	if clear.Name != DefaultSessionCookie || clear.MaxAge >= 0 {
		t.Fatalf("unexpected clear cookie: %#v", clear)
	}
}

func TestSessionCustomCookieNameRoundTrip(t *testing.T) {
	sessions, err := New(Options{
		Secret:     []byte(strings.Repeat("s", MinSessionSecretBytes)),
		CookieName: "custom_session",
		Insecure:   true,
		Now:        fixedClock(time.Unix(1_700_000_000, 0)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cookie, err := sessions.Cookie(Principal{ID: "user-1"})
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if cookie.Name != "custom_session" {
		t.Fatalf("cookie.Name = %q, want custom_session", cookie.Name)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&cookie)
	principal, err := sessions.Principal(request)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}
	if principal == nil || principal.ID != "user-1" {
		t.Fatalf("resolved principal = %+v", principal)
	}
}

func TestSessionCookieRejectsEmptyPrincipalID(t *testing.T) {
	sessions := newTestSessions(t, time.Unix(1_700_000_000, 0))
	for _, principal := range []Principal{
		{},
		{ID: " ", Roles: []string{"admin"}},
	} {
		if _, err := sessions.Cookie(principal); err == nil {
			t.Fatalf("Cookie accepted principal with ID %q", principal.ID)
		}
	}
}

func TestNewRequiresSecret(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected New to fail without a secret")
	}
}

func TestNewRejectsBothSecretAndSecretEnv(t *testing.T) {
	t.Setenv("GOWDK_TEST_AUTH_SECRET", strings.Repeat("e", MinSessionSecretBytes))
	_, err := New(Options{
		Secret:    []byte(strings.Repeat("s", MinSessionSecretBytes)),
		SecretEnv: "GOWDK_TEST_AUTH_SECRET",
	})
	if err == nil || !strings.Contains(err.Error(), "Secret or SecretEnv") {
		t.Fatalf("expected mutually exclusive secret error, got %v", err)
	}
}

func TestNewRejectsInvalidSessionTTL(t *testing.T) {
	_, err := New(Options{
		Secret: []byte(strings.Repeat("s", MinSessionSecretBytes)),
		TTL:    -time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "ttl must be non-negative") {
		t.Fatalf("expected negative TTL error, got %v", err)
	}
}

func TestNewRejectsSubsecondSessionTTL(t *testing.T) {
	_, err := New(Options{
		Secret: []byte(strings.Repeat("s", MinSessionSecretBytes)),
		TTL:    time.Nanosecond,
	})
	if err == nil || !strings.Contains(err.Error(), "ttl must be at least 1s") {
		t.Fatalf("expected subsecond TTL error, got %v", err)
	}
}

func TestNewRejectsInvalidCookieName(t *testing.T) {
	_, err := New(Options{
		Secret:     []byte(strings.Repeat("s", MinSessionSecretBytes)),
		CookieName: "bad cookie",
	})
	if err == nil || !strings.Contains(err.Error(), "cookie name") {
		t.Fatalf("expected invalid cookie name error, got %v", err)
	}
}

func TestNewRejectsUnsafeDirectSecret(t *testing.T) {
	if _, err := New(Options{Secret: []byte("short")}); err == nil || !strings.Contains(err.Error(), "at least 32 bytes") {
		t.Fatalf("expected short-secret error, got %v", err)
	}
}

func TestNewReadsSecretFromEnv(t *testing.T) {
	t.Setenv("GOWDK_TEST_AUTH_SECRET", strings.Repeat("s", MinSessionSecretBytes))
	sessions, err := New(Options{SecretEnv: "GOWDK_TEST_AUTH_SECRET", Insecure: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if sessions == nil {
		t.Fatal("expected sessions")
	}
}

func TestNewPreservesEnvSecretBytes(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	envSecret := " " + strings.Repeat("s", MinSessionSecretBytes) + " "
	t.Setenv("GOWDK_TEST_AUTH_SECRET", envSecret)
	issuer, err := New(Options{SecretEnv: "GOWDK_TEST_AUTH_SECRET", Insecure: true, Now: fixedClock(now)})
	if err != nil {
		t.Fatalf("New env issuer: %v", err)
	}
	cookie, err := issuer.Cookie(Principal{ID: "user-1"})
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&cookie)

	exact, err := New(Options{Secret: []byte(envSecret), Insecure: true, Now: fixedClock(now)})
	if err != nil {
		t.Fatalf("New exact verifier: %v", err)
	}
	if principal, err := exact.Principal(request); err != nil || principal == nil {
		t.Fatalf("exact secret did not verify env-issued cookie: principal=%+v err=%v", principal, err)
	}

	trimmed, err := New(Options{Secret: []byte(strings.TrimSpace(envSecret)), Insecure: true, Now: fixedClock(now)})
	if err != nil {
		t.Fatalf("New trimmed verifier: %v", err)
	}
	if principal, err := trimmed.Principal(request); err != nil || principal != nil {
		t.Fatalf("trimmed secret verified env-issued cookie: principal=%+v err=%v", principal, err)
	}
}

func TestNewReportsEnvNameWithoutLeakingSecret(t *testing.T) {
	t.Setenv("GOWDK_TEST_AUTH_SECRET", "too-short-secret-value")
	_, err := New(Options{SecretEnv: "GOWDK_TEST_AUTH_SECRET"})
	if err == nil {
		t.Fatal("expected short env secret error")
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_AUTH_SECRET") {
		t.Fatalf("expected env name in error, got %v", err)
	}
	if strings.Contains(err.Error(), "too-short-secret-value") {
		t.Fatalf("secret value leaked in error: %v", err)
	}
}

// Sessions must satisfy the Provider interface so it can be registered with the
// generated RegisterAuthProvider hook.
var _ Provider = (*Sessions)(nil)
var _ PasswordHasher = PBKDF2Hasher{}
