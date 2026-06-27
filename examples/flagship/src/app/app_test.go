package flagship

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestLoginRequiresExplicitSessionSecret(t *testing.T) {
	resetTestSessions(t)
	t.Setenv("GOWDK_FLAGSHIP_EMAIL", "demo@example.com")
	t.Setenv("GOWDK_FLAGSHIP_SECRET", "")
	t.Setenv("GOWDK_FLAGSHIP_PASSWORD", "demo-password")

	result := loginForTest(t, "demo@example.com", "demo-password")

	assertLoginFailed(t, result)
}

func TestLoginRequiresExplicitPassword(t *testing.T) {
	resetTestSessions(t)
	t.Setenv("GOWDK_FLAGSHIP_EMAIL", "demo@example.com")
	t.Setenv("GOWDK_FLAGSHIP_SECRET", "development-flagship-session-secret-32b")
	t.Setenv("GOWDK_FLAGSHIP_PASSWORD", "")

	result := loginForTest(t, "demo@example.com", "demo-password")

	assertLoginFailed(t, result)
}

func TestLoginCreatesSignedSessionWithExplicitCredentials(t *testing.T) {
	resetTestSessions(t)
	t.Setenv("GOWDK_FLAGSHIP_EMAIL", "demo@example.com")
	t.Setenv("GOWDK_FLAGSHIP_SECRET", "development-flagship-session-secret-32b")
	t.Setenv("GOWDK_FLAGSHIP_PASSWORD", "demo-password")

	result := loginForTest(t, "demo@example.com", "demo-password")

	if result.Kind != response.Redirect || result.URL != "/dashboard" {
		t.Fatalf("login result = %#v, want redirect to dashboard", result)
	}
	if len(result.Cookies) != 1 {
		t.Fatalf("cookies = %#v, want one session cookie", result.Cookies)
	}
	cookie := result.Cookies[0]
	if cookie.Name != sessionCookie || !cookie.HttpOnly || cookie.Value == "" || !strings.Contains(cookie.Value, ".") {
		t.Fatalf("session cookie = %#v", cookie)
	}

	request, err := http.NewRequest(http.MethodGet, "/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&cookie)
	if current, ok := currentSession(request); !ok || current.Email != "demo@example.com" {
		t.Fatalf("current session = %#v ok=%v", current, ok)
	}
}

func loginForTest(t *testing.T, email string, password string) response.Response {
	t.Helper()
	result, err := Login(context.Background(), form.Values{
		"email":    {email},
		"password": {password},
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	return result
}

func assertLoginFailed(t *testing.T, result response.Response) {
	t.Helper()
	if result.Kind != response.Redirect || result.URL != "/?login=failed" {
		t.Fatalf("login result = %#v, want failed redirect", result)
	}
	if len(result.Cookies) != 0 {
		t.Fatalf("failed login set cookies: %#v", result.Cookies)
	}
}

func resetTestSessions(t *testing.T) {
	t.Helper()
	sessions.Lock()
	defer sessions.Unlock()
	sessions.Values = map[string]session{}
}
