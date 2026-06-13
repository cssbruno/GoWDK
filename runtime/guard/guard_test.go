package guard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
	gowdkresponse "github.com/cssbruno/gowdk/runtime/response"
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

func TestGuardRedirectResponseHelpers(t *testing.T) {
	err := RunGuards(Context{}, []string{"auth.required"}, Registry{
		"auth.required": func(Context) error { return RedirectTo("/login?next=/dashboard") },
	})
	result, ok := ResponseResult(err)
	if !ok || result.Kind != gowdkresponse.Redirect || result.Status != http.StatusSeeOther || result.URL != "/login?next=/dashboard" {
		t.Fatalf("unexpected guard redirect response: %#v ok=%v err=%v", result, ok, err)
	}

	for _, url := range []string{"https://example.com/login", "//example.com/login", "/\\evil.com", "/login\nSet-Cookie: bad=1"} {
		if _, ok := ResponseResult(Redirect(url, http.StatusSeeOther)); ok {
			t.Fatalf("unsafe guard redirect %q should not produce a response", url)
		}
		if _, ok := ResponseResult(RedirectError{URL: url, Status: http.StatusFound}); ok {
			t.Fatalf("unsafe manually constructed guard redirect %q should not produce a response", url)
		}
		if _, ok := ResponseResult(&RedirectError{URL: url, Status: http.StatusFound}); ok {
			t.Fatalf("unsafe manually constructed pointer guard redirect %q should not produce a response", url)
		}
	}
	if _, ok := ResponseResult(Redirect("/login", http.StatusOK)); ok {
		t.Fatal("non-redirect guard status should not produce a response")
	}
	if _, ok := ResponseResult(RedirectError{URL: "/login", Status: http.StatusOK}); ok {
		t.Fatal("manually constructed non-redirect guard status should not produce a response")
	}

	err = RunGuards(Context{}, []string{"auth.required"}, Registry{
		"auth.required": func(Context) error {
			return Respond(gowdkresponse.JSONBody(http.StatusUnauthorized, `{"error":"login required"}`))
		},
	})
	result, ok = ResponseResult(err)
	if !ok || result.Kind != gowdkresponse.JSON || result.Status != http.StatusUnauthorized {
		t.Fatalf("unexpected guard custom response: %#v ok=%v err=%v", result, ok, err)
	}

	result, ok = ResponseResult(&ResponseError{Result: gowdkresponse.JSONBody(http.StatusUnauthorized, `{"error":"login required"}`)})
	if !ok || result.Kind != gowdkresponse.JSON || result.Status != http.StatusUnauthorized {
		t.Fatalf("unexpected pointer guard custom response: %#v ok=%v", result, ok)
	}
}

func TestWriteNoStoreFailure(t *testing.T) {
	redirect := httptest.NewRecorder()
	WriteNoStoreFailure(redirect, RedirectTo("/login"))
	if redirect.Code != http.StatusSeeOther || redirect.Header().Get("Location") != "/login" || redirect.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected redirect failure response: status=%d headers=%v", redirect.Code, redirect.Header())
	}

	jsonResponse := httptest.NewRecorder()
	WriteNoStoreFailure(jsonResponse, Respond(gowdkresponse.JSONBody(http.StatusUnauthorized, `{"error":"login required"}`)))
	if jsonResponse.Code != http.StatusUnauthorized || jsonResponse.Header().Get("Cache-Control") != "no-store" || !strings.Contains(jsonResponse.Body.String(), "login required") {
		t.Fatalf("unexpected JSON failure response: status=%d headers=%v body=%q", jsonResponse.Code, jsonResponse.Header(), jsonResponse.Body.String())
	}

	ordinary := httptest.NewRecorder()
	WriteNoStoreFailure(ordinary, errors.New("guard failed"))
	if ordinary.Code != http.StatusForbidden || ordinary.Header().Get("Cache-Control") != "no-store" || !strings.Contains(ordinary.Body.String(), "guard failed") {
		t.Fatalf("unexpected ordinary failure response: status=%d headers=%v body=%q", ordinary.Code, ordinary.Header(), ordinary.Body.String())
	}
}
