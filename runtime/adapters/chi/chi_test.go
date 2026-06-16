package chi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	"github.com/cssbruno/gowdk/runtime/adapters/internal/conformance"
	chiframework "github.com/go-chi/chi/v5"
)

const openAPISpec = `{
  "openapi": "3.1.0",
  "servers": [{"url": "/app"}],
  "paths": {
    "/": {"get": {"responses": {"200": {"description": "OK"}}}},
    "/api/status": {"get": {"responses": {"200": {"description": "OK"}}}},
    "/patients": {"post": {"responses": {"200": {"description": "OK"}}}},
    "/patients/{id}": {"get": {"responses": {"200": {"description": "OK"}}}}
  }
}`

const emptyOpenAPISpec = `{
  "openapi": "3.1.0",
  "servers": [{"url": "/app"}],
  "paths": {}
}`

func TestHandlerWrapsHTTPHandler(t *testing.T) {
	router := chiframework.NewRouter()
	Mount(router, "/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		context, ok := Context(request.Context())
		if !ok {
			t.Fatal("expected chi context")
		}
		writer.Header().Set("X-GOWDK-Test", "chi")
		writer.Header().Set("X-GOWDK-Route", context.RoutePattern())
		_, _ = writer.Write([]byte(request.Method + " " + request.URL.Path))
	}))

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/docs"},
		{method: http.MethodPost, path: "/forms/signup"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, nil)
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK || recorder.Body.String() != test.method+" "+test.path {
			t.Fatalf("unexpected response for %s %s: %d %q", test.method, test.path, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("X-GOWDK-Test") != "chi" {
			t.Fatalf("expected wrapped handler header, got %#v", recorder.Header())
		}
	}
}

func TestMountOpenAPIConformance(t *testing.T) {
	conformance.AssertOpenAPIConformance(t, []byte(openAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		router := chiframework.NewRouter()
		err := MountOpenAPI(router, []byte(openAPISpec), handler, WithPrefix(prefix))
		return router, err
	})
}

func TestMountOpenAPIFallbackConformance(t *testing.T) {
	conformance.AssertEmptyOpenAPIFallback(t, []byte(emptyOpenAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		router := chiframework.NewRouter()
		err := MountOpenAPI(router, []byte(emptyOpenAPISpec), handler, WithPrefix(prefix))
		return router, err
	})
}

func TestMountOpenAPIRestRouteConformance(t *testing.T) {
	conformance.AssertOpenAPIRestRoute(t, "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		router := chiframework.NewRouter()
		err := MountRoutes(router, routes, handler, WithPrefix(prefix))
		return router, err
	})
}

func TestMountRoutesRebasesPrefixedRedirect(t *testing.T) {
	conformance.AssertPrefixedRedirect(t, "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		router := chiframework.NewRouter()
		err := MountRoutes(router, routes, handler, WithPrefix(prefix))
		return router, err
	})
}

func TestMountRoutesMatchesRestParam(t *testing.T) {
	router := chiframework.NewRouter()
	err := MountRoutes(router, []gowdkadapters.Route{
		{Method: http.MethodGet, Path: "/files/{path...}"},
	}, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("matched"))
	}))
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/files/one/two", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "matched" {
		t.Fatalf("unexpected response for rest route: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
