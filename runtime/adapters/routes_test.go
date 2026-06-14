package adapters

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesFromOpenAPI(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.1.0",
		"paths": {
			"/docs/{path}": {
				"get": {
					"x-gowdk": {"route": "/docs/{path...}"},
					"responses": {"200": {"description": "OK"}}
				}
			},
			"/patients/{id}": {
				"get": {"responses": {"200": {"description": "OK"}}},
				"parameters": []
			},
			"/patients": {
				"post": {"responses": {"200": {"description": "OK"}}}
			}
		}
	}`)

	routes, err := RoutesFromOpenAPI(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []Route{
		{Method: http.MethodGet, Path: "/docs/{path...}"},
		{Method: http.MethodGet, Path: "/patients/{id}"},
		{Method: http.MethodPost, Path: "/patients"},
	}
	if len(routes) != len(want) {
		t.Fatalf("routes = %#v, want %#v", routes, want)
	}
	for index := range want {
		if routes[index] != want[index] {
			t.Fatalf("routes[%d] = %#v, want %#v", index, routes[index], want[index])
		}
	}
}

func TestTranslatePattern(t *testing.T) {
	tests := []struct {
		name  string
		route string
		style PatternStyle
		want  string
	}{
		{name: "chi typed param", route: "/patients/{id:int}", style: PatternChi, want: "/patients/{id}"},
		{name: "echo param", route: "/patients/{id}", style: PatternEcho, want: "/patients/:id"},
		{name: "gin param", route: "/patients/{id}", style: PatternGin, want: "/patients/:id"},
		{name: "chi rest", route: "/files/{path...}", style: PatternChi, want: "/files/*"},
		{name: "echo rest", route: "/files/{path...}", style: PatternEcho, want: "/files/*"},
		{name: "gin rest", route: "/files/{path...}", style: PatternGin, want: "/files/*path"},
		{name: "root", route: "/", style: PatternGin, want: "/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := TranslatePattern(test.route, test.style)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("TranslatePattern(%q, %q) = %q, want %q", test.route, test.style, got, test.want)
			}
		})
	}
}

func TestTranslatePatternRejectsInvalidRestPosition(t *testing.T) {
	_, err := TranslatePattern("/files/{path...}/edit", PatternGin)
	if err == nil || !strings.Contains(err.Error(), "must be the final segment") {
		t.Fatalf("expected invalid rest segment error, got %v", err)
	}
}

func TestJoinPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		route  string
		want   string
	}{
		{prefix: "", route: "/", want: "/"},
		{prefix: "/app", route: "/", want: "/app"},
		{prefix: "app/", route: "/patients/{id}", want: "/app/patients/{id}"},
	}
	for _, test := range tests {
		got, err := JoinPrefix(test.prefix, test.route)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("JoinPrefix(%q, %q) = %q, want %q", test.prefix, test.route, got, test.want)
		}
	}
}

func TestNormalizeMountPrefixRejectsAmbiguousRootPrefix(t *testing.T) {
	for _, prefix := range []string{"//evil.example", `/\evil.example`} {
		t.Run(prefix, func(t *testing.T) {
			if got, err := NormalizeMountPrefix(prefix); err == nil {
				t.Fatalf("NormalizeMountPrefix(%q) = %q, want error", prefix, got)
			}
		})
	}
}

func TestCleanRoutePathRejectsAmbiguousRootPath(t *testing.T) {
	if got, err := CleanRoutePath("//evil.example"); err == nil {
		t.Fatalf("CleanRoutePath returned %q, want error", got)
	}
}

func TestHandlerWithPrefixStripsMountPrefix(t *testing.T) {
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(request.URL.Path))
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app/patients/123", nil))

	if recorder.Body.String() != "/patients/123" {
		t.Fatalf("handler saw path %q", recorder.Body.String())
	}
}

func TestHandlerWithPrefixStripsExactMountPrefixToRoot(t *testing.T) {
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(request.URL.Path))
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app", nil))

	if recorder.Body.String() != "/" {
		t.Fatalf("handler saw path %q", recorder.Body.String())
	}
}

func TestHandlerWithPrefixRejectsNonBoundaryPrefix(t *testing.T) {
	called := false
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		_, _ = writer.Write([]byte(request.URL.Path))
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/application/status", nil))

	if called {
		t.Fatal("handler was called for path outside mount prefix")
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandlerWithPrefixRebasesLocation(t *testing.T) {
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/login?next=/dashboard", http.StatusSeeOther)
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app/private", nil))

	if location := recorder.Header().Get("Location"); location != "/app/login?next=/dashboard" {
		t.Fatalf("Location = %q, want %q", location, "/app/login?next=/dashboard")
	}
}

func TestHandlerWithPrefixRebasesHTMLRootURLs(t *testing.T) {
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(`<a href="/login">Login</a><img src="/assets/logo.png" srcset="/small.png 1x, /app/large.png 2x, https://cdn.example.com/x.png 3x"><form action="/signup"></form><button formaction="/save"></button><blockquote cite="/quote"></blockquote><object data="/media.svg"></object><a longdesc="/details">Details</a><html manifest="/app.webmanifest"><svg><use xlink:href="/icons.svg#logo"></use></svg><meta property="og:image" content="/social.png"><a href="/app/already">Already</a><style>@import "/theme.css";.hero{background:url('/assets/hero.png')}</style>`))
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app", nil))

	body := recorder.Body.String()
	for _, expected := range []string{
		`href="/app/login"`,
		`src="/app/assets/logo.png"`,
		`srcset="/app/small.png 1x, /app/large.png 2x, https://cdn.example.com/x.png 3x"`,
		`action="/app/signup"`,
		`formaction="/app/save"`,
		`cite="/app/quote"`,
		`data="/app/media.svg"`,
		`longdesc="/app/details"`,
		`manifest="/app/app.webmanifest"`,
		`xlink:href="/app/icons.svg#logo"`,
		`content="/app/social.png"`,
		`href="/app/already"`,
		`@import "/app/theme.css"`,
		`url('/app/assets/hero.png')`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in rebased HTML:\n%s", expected, body)
		}
	}
	if strings.Contains(body, `/app/app/already`) {
		t.Fatalf("double-prefixed already rebased URL:\n%s", body)
	}
}

func TestHandlerWithPrefixRebasesCSSRootURLs(t *testing.T) {
	handler, err := HandlerWithPrefix("/app", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/css; charset=utf-8")
		writer.Header().Set("Content-Length", "999")
		_, _ = writer.Write([]byte(`@import "/theme.css";@import '/app/base.css';@import url("/fonts.css");.hero{background:url("/hero.png")}.icon{mask:url('/icons.svg')}.already{background:url('/app/existing.png')}.cdn{background:url(//cdn.example.com/hero.png)}`))
	}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app/assets/site.css", nil))

	body := recorder.Body.String()
	for _, expected := range []string{
		`@import "/app/theme.css"`,
		`@import '/app/base.css'`,
		`@import url("/app/fonts.css")`,
		`url("/app/hero.png")`,
		`url('/app/icons.svg')`,
		`url('/app/existing.png')`,
		`url(//cdn.example.com/hero.png)`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in rebased CSS:\n%s", expected, body)
		}
	}
	if strings.Contains(body, `/app/app/existing.png`) {
		t.Fatalf("double-prefixed already rebased CSS URL:\n%s", body)
	}
	if length := recorder.Header().Get("Content-Length"); length != "" {
		t.Fatalf("Content-Length = %q, want cleared after rebasing", length)
	}
}

func TestRebaseLocalURLLeavesUnsafeRootURLUnchanged(t *testing.T) {
	for _, value := range []string{"//evil.example/login", `/\evil.example\login`} {
		t.Run(value, func(t *testing.T) {
			if got := RebaseLocalURL("/app", value); got != value {
				t.Fatalf("RebaseLocalURL returned %q, want %q", got, value)
			}
		})
	}
}

func TestOpenAPIWithServerURL(t *testing.T) {
	spec := []byte(`{"openapi":"3.1.0","servers":[{"url":"/"}],"paths":{}}`)
	rewritten, err := OpenAPIWithServerURL(spec, "/app/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rewritten), `"servers": [
    {
      "url": "/app"
    }
  ]`) {
		t.Fatalf("server URL was not rewritten:\n%s", rewritten)
	}
}
