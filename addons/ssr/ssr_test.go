package ssr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func TestAddonRegistersSSRFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "ssr" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureSSR) {
		t.Fatal("expected ssr feature")
	}
}

func TestLoadFuncContract(t *testing.T) {
	request, err := http.NewRequestWithContext(context.WithValue(context.Background(), "trace", "abc"), http.MethodGet, "/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	session := map[string]any{"userID": "u_123"}
	load := LoadFunc(func(ctx LoadContext) (map[string]any, error) {
		return map[string]any{
			"name":   "GOWDK",
			"trace":  ctx.Context.Value("trace"),
			"path":   ctx.Request.URL.Path,
			"userID": ctx.Session["userID"],
		}, nil
	})

	data, err := load(NewLoadContext(request, session))
	if err != nil {
		t.Fatal(err)
	}
	if data["name"] != "GOWDK" {
		t.Fatalf("unexpected load data: %#v", data)
	}
	if data["trace"] != "abc" || data["path"] != "/dashboard" || data["userID"] != "u_123" {
		t.Fatalf("expected request/session data in load context, got %#v", data)
	}
}

func TestLoadPathResolvesNestedMapsAndStructs(t *testing.T) {
	type profile struct {
		DisplayName string `json:"displayName"`
	}
	type user struct {
		Name    string
		Profile *profile
	}
	data := map[string]any{
		"user": user{
			Name:    "Ada",
			Profile: &profile{DisplayName: "Ada Lovelace"},
		},
		"account": map[string]string{
			"plan": "pro",
		},
	}

	for path, expected := range map[string]any{
		"user.name":                "Ada",
		"user.profile.displayName": "Ada Lovelace",
		"user.Profile.displayName": "Ada Lovelace",
		"account.plan":             "pro",
	} {
		value, ok := LoadPath(data, path)
		if !ok || value != expected {
			t.Fatalf("LoadPath(%q) = %#v, %v; want %#v, true", path, value, ok, expected)
		}
	}
}

func TestLoadPathRejectsMissingOrInvalidPaths(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{"name": "Ada"},
	}

	for _, path := range []string{"", "user.", "user.email", "missing.name"} {
		if value, ok := LoadPath(data, path); ok {
			t.Fatalf("LoadPath(%q) = %#v, true; want false", path, value)
		}
	}
}

func TestErrorHandlerContract(t *testing.T) {
	var captured error
	handler := ErrorHandler(func(_ http.ResponseWriter, _ *http.Request, errorValue error) {
		captured = errorValue
	})
	expected := errors.New("boom")

	handler(nil, nil, expected)

	if !errors.Is(captured, expected) {
		t.Fatalf("expected captured error, got %v", captured)
	}
}

func TestDefaultErrorHandlerWritesHTTP500(t *testing.T) {
	response := httptest.NewRecorder()
	DefaultErrorHandler(response, httptest.NewRequest(http.MethodGet, "/dashboard", nil), errors.New("load failed password=secret"))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "GOWDK SSR error") {
		t.Fatalf("unexpected error response body: %q", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "load failed") || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("internal error leaked in response body: %q", response.Body.String())
	}
	if cache := response.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store error response, got %q", cache)
	}
}

func TestRedirectToContract(t *testing.T) {
	err := RedirectTo("/login?next=/dashboard")

	url, status, ok := RedirectTarget(err)
	if !ok {
		t.Fatalf("expected redirect target, got %v", err)
	}
	if url != "/login?next=/dashboard" || status != http.StatusSeeOther {
		t.Fatalf("unexpected redirect target: url=%q status=%d", url, status)
	}
}

func TestRedirectRejectsUnsafeURL(t *testing.T) {
	// "/\evil.com" and "\\evil.com" are rejected because browsers normalize
	// "\" to "/" before navigating, turning them into protocol-relative
	// "//evil.com" redirects.
	for _, url := range []string{
		"https://example.com",
		"//example.com",
		"/login\nSet-Cookie: bad=true",
		`/\evil.com`,
		`\\evil.com`,
		`/foo\..\\evil.com`,
	} {
		if _, _, ok := RedirectTarget(Redirect(url, http.StatusSeeOther)); ok {
			t.Fatalf("expected unsafe redirect URL %q to fail", url)
		}
	}
	if _, _, ok := RedirectTarget(Redirect("/login", http.StatusOK)); ok {
		t.Fatal("expected non-3xx redirect status to fail")
	}
	// "/%5C" stays accepted: browsers do not percent-decode the Location
	// path before resolving it, so an encoded backslash never becomes a
	// host separator and is safe to pass through as an opaque path byte.
	url, _, ok := RedirectTarget(Redirect("/%5Cevil.com", http.StatusSeeOther))
	if !ok || url != "/%5Cevil.com" {
		t.Fatalf("expected percent-encoded backslash path to stay valid, got ok=%v url=%q", ok, url)
	}
}

func TestRunGuardsExecutesInDeclarationOrder(t *testing.T) {
	var calls []string
	registry := GuardRegistry{
		"auth.required": func(LoadContext) error {
			calls = append(calls, "auth.required")
			return nil
		},
		"billing.active": func(LoadContext) error {
			calls = append(calls, "billing.active")
			return nil
		},
	}

	if err := RunGuards(LoadContext{}, []string{"auth.required", "billing.active"}, registry); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "auth.required,billing.active" {
		t.Fatalf("unexpected guard order: %#v", calls)
	}
}

func TestRunGuardsReportsMissingOrFailedGuard(t *testing.T) {
	if err := RunGuards(LoadContext{}, []string{"auth.required"}, GuardRegistry{}); err == nil || !strings.Contains(err.Error(), `guard "auth.required" is not registered`) {
		t.Fatalf("expected missing guard error, got %v", err)
	}

	expected := errors.New("forbidden")
	err := RunGuards(LoadContext{}, []string{"auth.required"}, GuardRegistry{
		"auth.required": func(LoadContext) error { return expected },
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped guard error, got %v", err)
	}
}

func TestRunGuardsWithAuthExecutesNativeRBACGuards(t *testing.T) {
	provider := gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			ID:          "user-1",
			Roles:       []string{"admin"},
			Permissions: []string{"posts.write"},
		}, nil
	})

	err := RunGuardsWithAuth(LoadContext{}, []string{"role:admin", "permission:posts.write"}, nil, provider)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunGuardsWithAuthFailsClosedForNativeRBACGuards(t *testing.T) {
	if err := RunGuardsWithAuth(LoadContext{}, []string{"role:admin"}, nil, nil); err == nil || !strings.Contains(err.Error(), "requires an auth provider") {
		t.Fatalf("expected missing auth provider error, got %v", err)
	}

	if err := RunGuardsWithAuth(LoadContext{}, []string{"role:admin"}, nil, gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return nil, nil
	})); !errors.Is(err, gowdkauth.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}

	err := RunGuardsWithAuth(LoadContext{}, []string{"permission:posts.write"}, nil, gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{ID: "user-1", Permissions: []string{"posts.read"}}, nil
	}))
	if !errors.Is(err, gowdkauth.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestRegisterAddsGeneratedRoutes(t *testing.T) {
	router := &recordingRouter{}
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	Register(router, []Route{{Pattern: "GET /dashboard", Handler: handler}})

	if len(router.routes) != 1 {
		t.Fatalf("expected one route, got %#v", router.routes)
	}
	if router.routes[0].pattern != "GET /dashboard" || router.routes[0].handler == nil {
		t.Fatalf("unexpected route registration: %#v", router.routes)
	}
}

func TestLayoutStackIsOrdered(t *testing.T) {
	stack := LayoutStack{"root", "dashboard"}
	if len(stack) != 2 || stack[0] != "root" || stack[1] != "dashboard" {
		t.Fatalf("unexpected layout stack: %#v", stack)
	}
}

func TestComposeLayoutsWrapsBodyFromInnermostToOutermost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	ctx := NewLoadContext(request, map[string]any{"user": "Ada"})

	body, err := ComposeLayouts(ctx, LayoutStack{"root", "dashboard"}, LayoutRegistry{
		"root": func(ctx LoadContext, child string) (string, error) {
			if ctx.Request != request || ctx.Session["user"] != "Ada" {
				t.Fatalf("layout did not receive request-aware context: %#v", ctx)
			}
			return "<html>" + child + "</html>", nil
		},
		"dashboard": func(ctx LoadContext, child string) (string, error) {
			return `<section data-path="` + ctx.Request.URL.Path + `">` + child + "</section>", nil
		},
	}, "<main>Dashboard</main>")
	if err != nil {
		t.Fatal(err)
	}
	want := `<html><section data-path="/dashboard"><main>Dashboard</main></section></html>`
	if body != want {
		t.Fatalf("unexpected composed layout\nwant: %s\n got: %s", want, body)
	}
}

func TestComposeLayoutsReportsMissingLayout(t *testing.T) {
	_, err := ComposeLayouts(LoadContext{}, LayoutStack{"root"}, LayoutRegistry{}, "<main>Dashboard</main>")
	if err == nil || !strings.Contains(err.Error(), `SSR layout "root" is not registered`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposeLayoutsWrapsLayoutFailures(t *testing.T) {
	expected := errors.New("session expired")
	_, err := ComposeLayouts(LoadContext{}, LayoutStack{"root"}, LayoutRegistry{
		"root": func(LoadContext, string) (string, error) {
			return "", expected
		},
	}, "<main>Dashboard</main>")
	if !errors.Is(err, expected) || !strings.Contains(err.Error(), `SSR layout "root" failed`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

type recordingRouter struct {
	routes []recordedRoute
}

type recordedRoute struct {
	pattern string
	handler http.Handler
}

func (router *recordingRouter) Handle(pattern string, handler http.Handler) {
	router.routes = append(router.routes, recordedRoute{pattern: pattern, handler: handler})
}
