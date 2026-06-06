package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cssbruno/gowdk/runtime/asset"
	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestHandlerServesAppIndexAndIdentityHeaders(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte("<main>Home</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "<main>Home</main>" {
		t.Fatalf("unexpected body: %s", body)
	}
	if recorder.Header().Get("X-GOWDK-App") != "clinic" {
		t.Fatalf("unexpected app header: %q", recorder.Header().Get("X-GOWDK-App"))
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-cache" {
		t.Fatalf("expected generated static HTML to revalidate, got %q", cache)
	}
}

func TestHandlerHealth(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets: asset.Manifest{Version: 1, Files: map[string]string{
			"assets/app.css": "assets/app.css",
		}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/_gowdk/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	for _, expected := range []string{`"status":"ok"`, `"app":"clinic"`, `"assets":"1"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("expected health response to contain %q, got %s", expected, recorder.Body.String())
		}
	}
}

func TestHandlerDelegatesAction(t *testing.T) {
	called := false
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		Action: func(response http.ResponseWriter, request *http.Request) bool {
			called = true
			response.WriteHeader(http.StatusNoContent)
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/submit", nil)

	handler.ServeHTTP(recorder, request)

	if !called {
		t.Fatal("expected action hook to run")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerDelegatesBackendBeforeLegacyHooks(t *testing.T) {
	called := false
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		Backend: func(response http.ResponseWriter, request *http.Request) bool {
			called = true
			response.WriteHeader(http.StatusAccepted)
			return true
		},
		Action: func(response http.ResponseWriter, request *http.Request) bool {
			t.Fatal("legacy action hook should not run after backend handled request")
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/submit", nil)

	handler.ServeHTTP(recorder, request)

	if !called {
		t.Fatal("expected backend hook to run")
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerAcknowledgesCookie(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://gowdk.test/_gowdk/cookie-ack/", nil)
	request.Header.Set("Referer", "https://gowdk.test/docs/?tab=deploy")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/docs/?tab=deploy" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
	setCookie := recorder.Header().Get("Set-Cookie")
	for _, expected := range []string{"gowdk_cookie_ack=accepted", "Path=/", "Max-Age=31536000", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(setCookie, expected) {
			t.Fatalf("expected Set-Cookie to contain %q, got %q", expected, setCookie)
		}
	}
}

func TestHandlerHidesAcknowledgedCookieNotice(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte(`<main>Home</main><form data-cookie-notice method="post"></form>`)},
		},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "gowdk_cookie_ack", Value: "accepted"})

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "data-cookie-notice hidden") {
		t.Fatalf("expected hidden cookie notice, got %s", body)
	}
}

type fakeCSRFTokenSource struct {
	field string
	token string
	err   error
	calls int
}

func (source *fakeCSRFTokenSource) Token(http.ResponseWriter) (string, error) {
	source.calls++
	return source.token, source.err
}

func (source *fakeCSRFTokenSource) FieldName() string {
	return source.field
}

func TestHandlerInjectsCSRFHiddenInputsIntoPOSTForms(t *testing.T) {
	csrf := &fakeCSRFTokenSource{field: "_csrf", token: `token"&<>`}
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte(`<main><form class="signup" method="post" action="/signup"><input name="email"></form><form method="get" action="/search"></form></main>`)},
		},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		CSRF:     csrf,
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := recorder.Body.String()
	expected := `<form class="signup" method="post" action="/signup"><input type="hidden" name="_csrf" value="token&#34;&amp;&lt;&gt;">`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected hidden csrf input after POST form tag, got %s", body)
	}
	if count := strings.Count(body, `name="_csrf"`); count != 1 {
		t.Fatalf("expected one csrf input, got %d: %s", count, body)
	}
	if csrf.calls != 1 {
		t.Fatalf("expected one token generation call, got %d", csrf.calls)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store for csrf-personalized HTML, got %q", cache)
	}
}

func TestHandlerReturnsNoStoreErrorWhenCSRFTokenGenerationFails(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte(`<form method="post" action="/signup"></form>`)},
		},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		CSRF:     &fakeCSRFTokenSource{field: "_csrf", err: errors.New("entropy unavailable")},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store for csrf error, got %q", cache)
	}
	if !strings.Contains(recorder.Body.String(), "csrf token unavailable") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestHandlerUsesDynamicSSRAfterAppMiss(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		SSRDynamic: func(response http.ResponseWriter, request *http.Request) bool {
			_, _ = response.Write([]byte("<main>SSR</main>"))
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/post/hello", nil)

	handler.ServeHTTP(recorder, request)

	payload, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "<main>SSR</main>" {
		t.Fatalf("unexpected body: %s", payload)
	}
}

func TestLoadAssetManifest(t *testing.T) {
	manifest := LoadAssetManifest(fstest.MapFS{
		"gowdk-assets.json": {Data: []byte(`{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)},
	})
	if manifest.Resolve("assets/app.css") != "assets/app.css" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestBackendRouterDispatchesNormalizedRoutes(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Method: http.MethodGet,
		Path:   "/api/session/",
		Handler: APIHandler(func(ctx context.Context, request *http.Request) (response.Response, error) {
			attached, ok := Request(ctx)
			if !ok || attached.URL.Path != request.URL.Path {
				t.Fatal("expected request in context")
			}
			return response.JSONBody(http.StatusOK, `{"ok":true}`), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)

	if !router.Dispatch(recorder, request) {
		t.Fatal("expected route to dispatch")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store, got %q", cache)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestBackendRouterRejectsDuplicateRoutes(t *testing.T) {
	handler := NotImplemented("missing")
	_, err := NewBackendRouter(
		BackendRoute{Method: http.MethodPost, Path: "/login", Handler: handler},
		BackendRoute{Method: "post", Path: "login/", Handler: handler},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate backend route POST /login") {
		t.Fatalf("expected duplicate route error, got %v", err)
	}
}

func TestActionFormDecodesStructAndWritesResponse(t *testing.T) {
	type loginInput struct {
		Email string `form:"email"`
		Stay  bool   `form:"stay"`
	}
	decode := func(values form.Values) (loginInput, error) {
		stay, _, err := form.Bool(values, "stay")
		if err != nil {
			return loginInput{}, err
		}
		email, _, err := form.String(values, "email")
		if err != nil {
			return loginInput{}, err
		}
		return loginInput{Email: email, Stay: stay}, nil
	}
	handler := ActionForm(decode, func(ctx context.Context, input loginInput) (response.Response, error) {
		if _, ok := Request(ctx); !ok {
			t.Fatal("expected request in context")
		}
		if input.Email != "reader@example.com" || !input.Stay {
			t.Fatalf("unexpected input: %#v", input)
		}
		return response.RedirectTo("/dashboard"), nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{
		"email":       {"reader@example.com"},
		"stay":        {"on"},
		"_gowdk_csrf": {"token"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if !handler(recorder, request) {
		t.Fatal("expected action handler to handle request")
	}
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/dashboard" {
		t.Fatalf("unexpected location: %q", location)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store, got %q", cache)
	}
}

func TestActionFormPtrDecodesStructPointer(t *testing.T) {
	type updateInput struct {
		Name string `form:"name"`
	}
	decode := func(values form.Values) (updateInput, error) {
		name, _, err := form.String(values, "name")
		return updateInput{Name: name}, err
	}
	handler := ActionFormPtr(decode, func(ctx context.Context, input *updateInput) (response.Response, error) {
		if input == nil || input.Name != "Bruno" {
			t.Fatalf("unexpected input: %#v", input)
		}
		return response.HTMLBody(http.StatusOK, "<p>updated</p>"), nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(url.Values{
		"name": {"Bruno"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "<p>updated</p>" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestActionValuesRejectsTooLargeBody(t *testing.T) {
	handler := ActionValues(func(context.Context, form.Values) (response.Response, error) {
		t.Fatal("handler should not run for oversized body")
		return response.Response{}, nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("field="+strings.Repeat("a", int(DefaultActionBodyLimit)+1)))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestActionFormRejectsInvalidForm(t *testing.T) {
	decode := func(values form.Values) (struct {
		Email string `form:"email"`
	}, error) {
		email, _, err := form.String(values, "email")
		return struct {
			Email string `form:"email"`
		}{Email: email}, err
	}
	handler := ActionForm(decode, func(context.Context, struct {
		Email string `form:"email"`
	}) (response.Response, error) {
		t.Fatal("handler should not run for invalid form")
		return response.Response{}, nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(url.Values{
		"email": {"reader@example.com", "other@example.com"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "other@example.com") {
		t.Fatalf("response leaked submitted value: %s", recorder.Body.String())
	}
}

func TestContextHelpersCopyParams(t *testing.T) {
	ctx := WithParams(context.Background(), map[string]string{"slug": "hello"})
	params := Params(ctx)
	params["slug"] = "changed"
	if got := Params(ctx)["slug"]; got != "hello" {
		t.Fatalf("expected params copy, got %q", got)
	}
	ctx = WithCSRF(ctx, "token")
	if token := CSRF(ctx); token != "token" {
		t.Fatalf("unexpected csrf token: %q", token)
	}
	session := struct{ User string }{User: "bruno"}
	ctx = WithSession(ctx, session)
	if got := Session(ctx); got != session {
		t.Fatalf("unexpected session: %#v", got)
	}
	ctx = WithRoute(ctx, RouteMetadata{
		Kind:   "ssr",
		PageID: "blog.post",
		Method: http.MethodGet,
		Path:   "/blog/{slug}",
	})
	route, ok := Route(ctx)
	if !ok {
		t.Fatal("expected route metadata")
	}
	if route.Kind != "ssr" || route.PageID != "blog.post" || route.Method != http.MethodGet || route.Path != "/blog/{slug}" {
		t.Fatalf("unexpected route metadata: %#v", route)
	}
	ctx = WithEndpoint(ctx, EndpointMetadata{
		Kind:   "action",
		PageID: "login",
		Name:   "Login",
		Method: http.MethodPost,
		Path:   "/login",
	})
	endpoint, ok := Endpoint(ctx)
	if !ok {
		t.Fatal("expected endpoint metadata")
	}
	if endpoint.Kind != "action" || endpoint.PageID != "login" || endpoint.Name != "Login" || endpoint.Method != http.MethodPost || endpoint.Path != "/login" {
		t.Fatalf("unexpected endpoint metadata: %#v", endpoint)
	}
}
