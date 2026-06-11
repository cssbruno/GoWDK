package guard

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func TestNewContextCarriesRequestContext(t *testing.T) {
	request, err := http.NewRequestWithContext(context.WithValue(context.Background(), "trace", "abc"), http.MethodGet, "/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := NewContext(request, map[string]any{"user": "Ada"})

	if ctx.Request != request || ctx.Context.Value("trace") != "abc" || ctx.Session["user"] != "Ada" {
		t.Fatalf("unexpected guard context: %#v", ctx)
	}
}

func TestRunGuardsExecutesRegisteredGuards(t *testing.T) {
	var calls []string
	registry := Registry{
		"auth.required": func(Context) error {
			calls = append(calls, "auth.required")
			return nil
		},
		"billing.active": func(Context) error {
			calls = append(calls, "billing.active")
			return nil
		},
	}

	if err := RunGuards(Context{}, []string{"auth.required", "billing.active"}, registry); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "auth.required,billing.active" {
		t.Fatalf("unexpected guard order: %#v", calls)
	}
}

func TestRunGuardsReportsMissingAndFailedGuards(t *testing.T) {
	if err := RunGuards(Context{}, []string{"auth.required"}, Registry{}); err == nil || !strings.Contains(err.Error(), `guard "auth.required" is not registered`) {
		t.Fatalf("expected missing guard error, got %v", err)
	}

	expected := errors.New("nope")
	err := RunGuards(Context{}, []string{"auth.required"}, Registry{
		"auth.required": func(Context) error { return expected },
	})
	if !errors.Is(err, expected) || !strings.Contains(err.Error(), `guard "auth.required" failed`) {
		t.Fatalf("expected wrapped guard error, got %v", err)
	}
}

func TestRunGuardsWithAuthExecutesNativeRBACGuards(t *testing.T) {
	provider := gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			Roles:       []string{"admin"},
			Permissions: []string{"posts.write"},
		}, nil
	})

	err := RunGuardsWithAuth(Context{}, []string{"role:admin", "permission:posts.write"}, nil, provider)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunGuardsWithAuthFailsClosedForNativeRBACGuards(t *testing.T) {
	if err := RunGuardsWithAuth(Context{}, []string{"role:admin"}, nil, nil); err == nil || !strings.Contains(err.Error(), "requires an auth provider") {
		t.Fatalf("expected missing auth provider error, got %v", err)
	}

	if err := RunGuardsWithAuth(Context{}, []string{"role:admin"}, nil, gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return nil, nil
	})); !errors.Is(err, gowdkauth.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}

	err := RunGuardsWithAuth(Context{}, []string{"permission:posts.write"}, nil, gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{}, nil
	}))
	if !errors.Is(err, gowdkauth.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
