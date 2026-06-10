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
	"time"

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

func TestHandlerDeniesGuardlessRouteWith403(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html":           {Data: []byte("<main>Home</main>")},
			"dashboard/index.html": {Data: []byte("<main>Secret</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
		Denied:   map[string]bool{"/dashboard": true},
	}

	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("guardless route should be denied, got status %d", denied.Code)
	}

	// A route absent from Denied (intentionally public) still serves.
	served := httptest.NewRecorder()
	handler.ServeHTTP(served, httptest.NewRequest(http.MethodGet, "/", nil))
	if served.Code != http.StatusOK {
		t.Fatalf("public route should serve, got status %d", served.Code)
	}
}

func TestHandlerDeniesIndexArtifactAndDirectoryForms(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html":           {Data: []byte("<main>Home</main>")},
			"dashboard/index.html": {Data: []byte("<main>Secret</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
		Denied:   map[string]bool{"/dashboard": true},
	}

	// The index artifact path resolves to the same denied page file, so a direct
	// fetch by file path must also be denied rather than served.
	for _, requestPath := range []string{"/dashboard", "/dashboard/index.html"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s should be denied, got status %d", requestPath, recorder.Code)
		}
	}
}

func TestHandlerDeniesRootIndexArtifact(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{"index.html": {Data: []byte("<main>Home</main>")}},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
		Denied:   map[string]bool{"/": true},
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("/index.html should be denied for a guardless root, got status %d", recorder.Code)
	}
}

func TestHandlerDeniesDynamicSPAPatternArtifacts(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"blog/hello/index.html": {Data: []byte("<main>Hello</main>")},
			"blog/world/index.html": {Data: []byte("<main>World</main>")},
		},
		Identity:       Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:         asset.Manifest{Version: 1, Files: map[string]string{}},
		DeniedPatterns: []string{"/blog/{slug}"},
	}

	// Every concrete artifact expanded from the guardless dynamic page must be
	// denied, whether requested by canonical route or by index artifact path.
	for _, requestPath := range []string{"/blog/hello", "/blog/world", "/blog/hello/index.html"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s should be denied by pattern, got status %d", requestPath, recorder.Code)
		}
	}
}

func TestHandlerAppliesPageHTMLCachePolicy(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte("<main>Home</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets: asset.Manifest{
			Version: 1,
			Files:   map[string]string{},
			Cache:   map[string]string{"index.html": "public, max-age=120"},
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "public, max-age=120" {
		t.Fatalf("expected generated page cache policy, got %q", cache)
	}
}

func TestHandlerAppliesAssetManifestCachePolicy(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"assets/app.css": {Data: []byte("body{}")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets: asset.Manifest{
			Version: 1,
			Files:   map[string]string{"assets/app.css": "assets/app.css"},
			Cache:   map[string]string{"assets/app.css": "public, max-age=31536000, immutable"},
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "public, max-age=31536000, immutable" {
		t.Fatalf("expected manifest cache policy, got %q", cache)
	}
}

func TestHandlerRedirectsTrailingSlashGETToCanonicalPath(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{"blog/hello/index.html": {Data: []byte("<main>Hello</main>")}},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/blog/hello/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected 308 redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/blog/hello" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandlerRedirectsTrailingSlashPreservingQuery(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/blog/hello/?page=2&sort=asc", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected 308 redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/blog/hello?page=2&sort=asc" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandlerDoesNotRedirectRootPath(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{"index.html": {Data: []byte("<main>Home</main>")}},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected root path to serve directly, got %d", recorder.Code)
	}
}

func TestHandlerDoesNotRedirectTrailingSlashPOST(t *testing.T) {
	called := false
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
		Action: func(response http.ResponseWriter, request *http.Request) bool {
			called = true
			response.WriteHeader(http.StatusNoContent)
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/subscribe/", nil)

	handler.ServeHTTP(recorder, request)

	if !called {
		t.Fatal("expected POST with trailing slash to reach the action hook")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerHealth(t *testing.T) {
	metrics := &Metrics{}
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets: asset.Manifest{Version: 1, Files: map[string]string{
			"assets/app.css": "assets/app.css",
		}},
		Metrics: metrics,
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
	if !strings.Contains(recorder.Body.String(), `"metrics"`) || !strings.Contains(recorder.Body.String(), `"requests":1`) {
		t.Fatalf("expected health metrics, got %s", recorder.Body.String())
	}
	if snapshot := metrics.Snapshot(); snapshot.Requests != 1 || snapshot.Health != 1 {
		t.Fatalf("unexpected metrics snapshot: %#v", snapshot)
	}
}

func TestHandlerMetricsRecordDispatchOutcomes(t *testing.T) {
	metrics := &Metrics{}
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte("<main>Home</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Metrics:  metrics,
		Backend: func(response http.ResponseWriter, request *http.Request) bool {
			if request.URL.Path != "/api" {
				return false
			}
			response.WriteHeader(http.StatusOK)
			return true
		},
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/missing", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	snapshot := metrics.Snapshot()
	if snapshot.Requests != 4 || snapshot.Static != 1 || snapshot.Backend != 1 || snapshot.MethodNotAllow != 1 || snapshot.NotFound != 1 {
		t.Fatalf("unexpected metrics snapshot: %#v", snapshot)
	}
}

func TestHandlerServesGenerated404Page(t *testing.T) {
	root := fstest.MapFS{
		"404.html": {Data: []byte("<main>Missing</main>")},
	}
	handler := Handler{
		Root:       root,
		Identity:   Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		ErrorPages: LoadErrorPages(root),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Missing</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store error page, got %q", cache)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected HTML content type, got %q", contentType)
	}
}

func TestWriteErrorPageServesGenerated500Page(t *testing.T) {
	root := fstest.MapFS{
		"500.html": {Data: []byte("<main>Server Error</main>")},
	}
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request = request.WithContext(withErrorPages(request.Context(), LoadErrorPages(root)))
	recorder := httptest.NewRecorder()

	WriteErrorPage(recorder, request, http.StatusInternalServerError, "load failed")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Server Error</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store error page, got %q", cache)
	}
}

func TestWriteErrorPagePrefersRouteErrorPage(t *testing.T) {
	root := fstest.MapFS{
		"500.html":                    {Data: []byte("<main>Global Server Error</main>")},
		"errors/dashboard.html":       {Data: []byte("<main>Dashboard Error</main>")},
		"errors/other-dashboard.html": {Data: []byte("<main>Other Dashboard Error</main>")},
	}
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	ctx := withErrorPages(request.Context(), LoadErrorPagesWith(root, ErrorPage{Path: "/errors/dashboard.html"}))
	ctx = WithRoute(ctx, RouteMetadata{Kind: "ssr", PageID: "dashboard", Path: "/dashboard", ErrorPage: "errors/dashboard.html"})
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	WriteErrorPage(recorder, request, http.StatusInternalServerError, "load failed")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Dashboard Error</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store error page, got %q", cache)
	}
}

func TestHandlerRecoversSSRExactPanicWithGenerated500Page(t *testing.T) {
	root := fstest.MapFS{
		"500.html": {Data: []byte("<main>Server Error</main>")},
	}
	handler := Handler{
		Root:       root,
		Identity:   Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		ErrorPages: LoadErrorPages(root),
		SSRExact: func(http.ResponseWriter, *http.Request) bool {
			panic("database password leaked")
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Server Error</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "database password leaked") {
		t.Fatalf("panic value leaked in response: %s", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store boundary response, got %q", cache)
	}
}

func TestRecoverSSRRoutePanicUsesRouteErrorPage(t *testing.T) {
	root := fstest.MapFS{
		"500.html":                    {Data: []byte("<main>Server Error</main>")},
		"errors/dashboard.html":       {Data: []byte("<main>Dashboard Error</main>")},
		"errors/other-dashboard.html": {Data: []byte("<main>Other Dashboard Error</main>")},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	ctx := withErrorPages(request.Context(), LoadErrorPagesWith(root, ErrorPage{Path: "errors/dashboard.html"}))
	ctx = WithRoute(ctx, RouteMetadata{Kind: "ssr", PageID: "dashboard", Path: "/dashboard", ErrorPage: "errors/dashboard.html"})
	request = request.WithContext(ctx)

	RecoverSSRRoutePanic(recorder, request, "secret panic detail")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Dashboard Error</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret panic detail") {
		t.Fatalf("panic value leaked in response: %s", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store boundary response, got %q", cache)
	}
}

func TestRecoverSSRRoutePanicAbortsAfterHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &boundaryResponseWriter{ResponseWriter: recorder}
	writer.WriteHeader(http.StatusAccepted)

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recovered := func() (value any) {
		defer func() { value = recover() }()
		RecoverSSRRoutePanic(writer, request, "secret panic detail")
		return nil
	}()

	if recovered != http.ErrAbortHandler {
		t.Fatalf("expected started response to abort the connection, got %v", recovered)
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("unexpected body after headers started: %q", recorder.Body.String())
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

func (source *fakeCSRFTokenSource) Token(http.ResponseWriter, *http.Request) (string, error) {
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

func TestBackendRouterOnlyDispatchesQueryRoutesForJSONRequests(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Method: http.MethodGet,
		Path:   "/patients",
		Kind:   "query",
		Handler: func(writer http.ResponseWriter, request *http.Request) bool {
			writer.WriteHeader(http.StatusAccepted)
			return true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/patients", nil),
		httptest.NewRequest(http.MethodGet, "/patients", nil),
	}
	requests[1].Header.Set("Accept", "text/html")
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		if router.Dispatch(recorder, request) {
			t.Fatalf("expected document request not to dispatch query route, got status %d", recorder.Code)
		}
	}

	jsonRequest := httptest.NewRequest(http.MethodGet, "/patients", nil)
	jsonRequest.Header.Set("Accept", "application/json")
	jsonRecorder := httptest.NewRecorder()
	if !router.Dispatch(jsonRecorder, jsonRequest) {
		t.Fatal("expected JSON request to dispatch query route")
	}
	if jsonRecorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected JSON query status: %d", jsonRecorder.Code)
	}

	headerRequest := httptest.NewRequest(http.MethodGet, "/patients", nil)
	headerRequest.Header.Set("X-GOWDK-Query", "true")
	headerRecorder := httptest.NewRecorder()
	if !router.Dispatch(headerRecorder, headerRequest) {
		t.Fatal("expected X-GOWDK-Query request to dispatch query route")
	}
}

func TestBackendRouterRecoversActionPanic(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Method: http.MethodPost,
		Path:   "/login",
		Kind:   "action",
		Handler: func(http.ResponseWriter, *http.Request) bool {
			panic("secret token")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)

	if !router.Dispatch(recorder, request) {
		t.Fatal("expected route to dispatch")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "GOWDK action handler failed") || strings.Contains(body, "secret token") {
		t.Fatalf("unexpected boundary body: %q", body)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store boundary response, got %q", cache)
	}
}

func TestBackendRouterRecoversAPIPanic(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Method: http.MethodGet,
		Path:   "/api/session",
		Kind:   "api",
		Handler: func(http.ResponseWriter, *http.Request) bool {
			panic("secret token")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)

	if !router.Dispatch(recorder, request) {
		t.Fatal("expected route to dispatch")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "GOWDK API handler failed") || strings.Contains(body, "secret token") {
		t.Fatalf("unexpected boundary body: %q", body)
	}
}

func TestBoundaryLogsRecoveredPanicWithRedactedSecret(t *testing.T) {
	previous := BoundaryLogger
	t.Cleanup(func() { BoundaryLogger = previous })
	var logged string
	BoundaryLogger = func(message string) { logged = message }

	handler := Boundary("action", func(http.ResponseWriter, *http.Request) bool {
		panic("connect failed: password=hunter2")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)

	if !handler(recorder, request) {
		t.Fatal("expected boundary to handle panic")
	}
	if logged == "" {
		t.Fatal("expected recovered panic to be logged")
	}
	if !strings.Contains(logged, "recovered panic in action handler") {
		t.Fatalf("log missing panic context: %q", logged)
	}
	if strings.Contains(logged, "hunter2") {
		t.Fatalf("secret leaked into log: %q", logged)
	}
	if !strings.Contains(logged, "password=[REDACTED]") {
		t.Fatalf("expected redacted secret in log: %q", logged)
	}
	if strings.Contains(recorder.Body.String(), "hunter2") {
		t.Fatalf("secret leaked into response: %q", recorder.Body.String())
	}
}

func TestBoundaryLoggerNilSilencesPanicLog(t *testing.T) {
	previous := BoundaryLogger
	t.Cleanup(func() { BoundaryLogger = previous })
	BoundaryLogger = nil

	handler := Boundary("api", func(http.ResponseWriter, *http.Request) bool {
		panic("boom")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/x", nil)

	if !handler(recorder, request) {
		t.Fatal("expected boundary to handle panic")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
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

func TestAPIHandlerCapsRequestBody(t *testing.T) {
	var readErr error
	handler := APIHandler(func(_ context.Context, request *http.Request) (response.Response, error) {
		_, readErr = io.ReadAll(request.Body)
		if readErr != nil {
			return response.Response{}, readErr
		}
		return response.JSONBody(http.StatusOK, `{"ok":true}`), nil
	})
	recorder := httptest.NewRecorder()
	oversized := strings.Repeat("a", int(DefaultAPIBodyLimit)+1)
	request := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(oversized))

	handler(recorder, request)

	if readErr == nil {
		t.Fatal("expected oversized API body read to fail")
	}
	if !strings.Contains(readErr.Error(), "request body too large") {
		t.Fatalf("expected body-too-large read error, got %v", readErr)
	}
}

func TestAPIHandlerAllowsBodyWithinLimit(t *testing.T) {
	handler := APIHandler(func(_ context.Context, request *http.Request) (response.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return response.Response{}, err
		}
		return response.JSONBody(http.StatusOK, string(body)), nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(`{"ok":true}`))

	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestHandlerAppliesRequestTimeoutDeadline(t *testing.T) {
	var hadDeadline bool
	handler := Handler{
		RequestTimeout: 50 * time.Millisecond,
		Backend: func(_ http.ResponseWriter, request *http.Request) bool {
			_, hadDeadline = request.Context().Deadline()
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)

	handler.ServeHTTP(recorder, request)

	if !hadDeadline {
		t.Fatal("expected request context to carry a deadline when RequestTimeout is set")
	}
}

func TestHandlerWithoutRequestTimeoutHasNoDeadline(t *testing.T) {
	var hadDeadline bool
	handler := Handler{
		Backend: func(_ http.ResponseWriter, request *http.Request) bool {
			_, hadDeadline = request.Context().Deadline()
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)

	handler.ServeHTTP(recorder, request)

	if hadDeadline {
		t.Fatal("expected no deadline when RequestTimeout is zero")
	}
}

func TestHandlerRequestTimeoutCancelsSlowHandler(t *testing.T) {
	var ctxErr error
	handler := Handler{
		RequestTimeout: 20 * time.Millisecond,
		Backend: func(_ http.ResponseWriter, request *http.Request) bool {
			<-request.Context().Done()
			ctxErr = request.Context().Err()
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/slow", nil)

	handler.ServeHTTP(recorder, request)

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", ctxErr)
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
		Kind:          "ssr",
		PageID:        "blog.post",
		Method:        http.MethodGet,
		Path:          "/blog/{slug}",
		Render:        "ssr",
		DynamicParams: []string{"slug"},
		RouteParams:   []RouteParamMetadata{{Name: "slug", Type: "string"}},
		Guards:        []string{"auth.required"},
	})
	route, ok := Route(ctx)
	if !ok {
		t.Fatal("expected route metadata")
	}
	if route.Kind != "ssr" || route.PageID != "blog.post" || route.Method != http.MethodGet || route.Path != "/blog/{slug}" || route.Render != "ssr" {
		t.Fatalf("unexpected route metadata: %#v", route)
	}
	route.DynamicParams[0] = "changed"
	route.RouteParams[0].Name = "changed"
	route.Guards[0] = "changed"
	route, _ = Route(ctx)
	if route.DynamicParams[0] != "slug" || route.RouteParams[0].Name != "slug" || route.Guards[0] != "auth.required" {
		t.Fatalf("expected route metadata slices to be copied, got %#v", route)
	}
	ctx = WithTypedParams(ctx, map[string]any{"id": 42})
	typed := TypedParams(ctx)
	typed["id"] = 7
	if got := TypedParams(ctx)["id"]; got != 42 {
		t.Fatalf("expected typed params copy, got %#v", got)
	}
	ctx = WithEndpoint(ctx, EndpointMetadata{
		Kind:      "action",
		PageID:    "login",
		Name:      "Login",
		Method:    http.MethodPost,
		Path:      "/login",
		ErrorPage: "errors/login.html",
	})
	endpoint, ok := Endpoint(ctx)
	if !ok {
		t.Fatal("expected endpoint metadata")
	}
	if endpoint.Kind != "action" || endpoint.PageID != "login" || endpoint.Name != "Login" || endpoint.Method != http.MethodPost || endpoint.Path != "/login" || endpoint.ErrorPage != "errors/login.html" {
		t.Fatalf("unexpected endpoint metadata: %#v", endpoint)
	}
}

func TestRecoverEndpointPanicUsesEndpointErrorPage(t *testing.T) {
	root := fstest.MapFS{
		"500.html":            {Data: []byte("<main>Server Error</main>")},
		"errors/session.html": {Data: []byte("<main>Session Error</main>")},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	ctx := withErrorPages(request.Context(), LoadErrorPagesWith(root, ErrorPage{Path: "errors/session.html"}))
	ctx = WithEndpoint(ctx, EndpointMetadata{Kind: "api", PageID: "status", Name: "Session", Method: "GET", Path: "/api/session", ErrorPage: "errors/session.html"})
	request = request.WithContext(ctx)

	RecoverEndpointPanic(recorder, request, "secret panic detail")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "<main>Session Error</main>" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret panic detail") {
		t.Fatalf("panic value leaked in response: %s", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store boundary response, got %q", cache)
	}
}

func TestBoundaryRepanicsErrAbortHandler(t *testing.T) {
	previous := BoundaryLogger
	t.Cleanup(func() { BoundaryLogger = previous })
	var logged string
	BoundaryLogger = func(message string) { logged = message }

	handler := Boundary("api", func(http.ResponseWriter, *http.Request) bool {
		panic(http.ErrAbortHandler)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/x", nil)

	recovered := func() (value any) {
		defer func() { value = recover() }()
		handler(recorder, request)
		return nil
	}()
	if recovered != http.ErrAbortHandler {
		t.Fatalf("expected http.ErrAbortHandler to propagate, got %v", recovered)
	}
	if logged != "" {
		t.Fatalf("deliberate abort should not be logged as a failure: %q", logged)
	}
}

func TestBoundaryAbortsConnectionAfterResponseStarted(t *testing.T) {
	previous := BoundaryLogger
	t.Cleanup(func() { BoundaryLogger = previous })
	var logged string
	BoundaryLogger = func(message string) { logged = message }

	handler := Boundary("api", func(writer http.ResponseWriter, _ *http.Request) bool {
		if _, err := writer.Write([]byte("partial")); err != nil {
			t.Fatal(err)
		}
		panic("boom mid-stream")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/x", nil)

	recovered := func() (value any) {
		defer func() { value = recover() }()
		handler(recorder, request)
		return nil
	}()
	if recovered != http.ErrAbortHandler {
		t.Fatalf("expected started response to abort the connection, got %v", recovered)
	}
	if logged == "" {
		t.Fatal("expected mid-stream panic to be logged")
	}
	if body := recorder.Body.String(); body != "partial" {
		t.Fatalf("expected truncated body to stay as written, got %q", body)
	}
}
