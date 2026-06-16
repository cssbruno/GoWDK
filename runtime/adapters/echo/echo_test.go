package echo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	"github.com/cssbruno/gowdk/runtime/adapters/internal/conformance"
	echoframework "github.com/labstack/echo/v5"
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
	engine := echoframework.New()
	Mount(engine, "/*", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		context, ok := Context(request.Context())
		if !ok {
			t.Fatal("expected Echo context")
		}
		writer.Header().Set("X-GOWDK-Test", "echo")
		writer.Header().Set("X-GOWDK-Route", context.Path())
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
		engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK || recorder.Body.String() != test.method+" "+test.path {
			t.Fatalf("unexpected response for %s %s: %d %q", test.method, test.path, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("X-GOWDK-Test") != "echo" {
			t.Fatalf("expected wrapped handler header, got %#v", recorder.Header())
		}
		if recorder.Header().Get("X-GOWDK-Route") != "/*" {
			t.Fatalf("expected attached Echo context route, got %#v", recorder.Header())
		}
	}
}

func TestMountOpenAPIConformance(t *testing.T) {
	conformance.AssertOpenAPIConformance(t, []byte(openAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := echoframework.New()
		err := MountOpenAPI(engine, []byte(openAPISpec), handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountOpenAPIFallbackConformance(t *testing.T) {
	conformance.AssertEmptyOpenAPIFallback(t, []byte(emptyOpenAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := echoframework.New()
		err := MountOpenAPI(engine, []byte(emptyOpenAPISpec), handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountOpenAPIRestRouteConformance(t *testing.T) {
	conformance.AssertOpenAPIRestRoute(t, "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := echoframework.New()
		err := MountRoutes(engine, routes, handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountRoutesRebasesPrefixedRedirect(t *testing.T) {
	conformance.AssertPrefixedRedirect(t, "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := echoframework.New()
		err := MountRoutes(engine, routes, handler, WithPrefix(prefix))
		return engine, err
	})
}
