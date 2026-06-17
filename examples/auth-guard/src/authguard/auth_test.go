package authguard

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gowdkauth "github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/runtime/form"
)

func TestLoginIssuesSessionCookie(t *testing.T) {
	resetTestState()
	_, err := gowdkauth.Configure(gowdkauth.Options{
		Secret:     []byte(strings.Repeat("s", gowdkauth.MinSessionSecretBytes)),
		CookieName: sessionCookie,
		TTL:        12 * time.Hour,
		Insecure:   true,
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
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
	passwordState.Lock()
	passwordState.hash = ""
	passwordState.err = nil
	passwordState.Unlock()
}
