package authguard

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/form"
)

func TestLoginRequiresSessionSecret(t *testing.T) {
	resetTestState()
	t.Setenv("GOWDK_AUTH_SESSION_SECRET", "")
	_, err := Login(context.Background(), form.Values{"email": {"demo@example.com"}, "password": {"demo-password"}})
	if err == nil || !strings.Contains(err.Error(), "GOWDK_AUTH_SESSION_SECRET") {
		t.Fatalf("expected session secret error, got %v", err)
	}
}

func TestLoginIssuesSessionCookie(t *testing.T) {
	resetTestState()
	t.Setenv("GOWDK_AUTH_SESSION_SECRET", strings.Repeat("s", 32))
	result, err := Login(context.Background(), form.Values{"email": {"demo@example.com"}, "password": {"demo-password"}})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.Status != http.StatusSeeOther || result.URL != "/dashboard" {
		t.Fatalf("unexpected login response: %#v", result)
	}
	if len(result.Cookies) != 1 || result.Cookies[0].Name != sessionCookie {
		t.Fatalf("expected session cookie, got %#v", result.Cookies)
	}
	if result.Cookies[0].Secure {
		t.Fatal("expected local example cookie to be insecure by default")
	}
}

func resetTestState() {
	sessionState.Lock()
	sessionState.manager = nil
	sessionState.err = nil
	sessionState.Unlock()

	passwordState.Lock()
	passwordState.hash = ""
	passwordState.err = nil
	passwordState.Unlock()
}
