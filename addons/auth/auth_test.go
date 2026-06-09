package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
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
	for _, encoded := range []string{
		"",
		"plain",
		"pbkdf2-sha256$notanumber$c2FsdA$a2V5",
		"bcrypt$10$salt$key",
		"pbkdf2-sha256$600000$$a2V5",
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
		Secret:   []byte("test-secret-value"),
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

	other, err := New(Options{Secret: []byte("a-different-secret"), Insecure: true, Now: fixedClock(now)})
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
	sessions := newTestSessions(t, issuedAt)
	sessions.ttl = time.Hour

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

func TestNewRequiresSecret(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected New to fail without a secret")
	}
}

// Sessions must satisfy the Provider interface so it can be registered with the
// generated RegisterAuthProvider hook.
var _ Provider = (*Sessions)(nil)
