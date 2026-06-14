package gin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
	"github.com/cssbruno/gowdk/runtime/adapters/internal/conformance"
	ginframework "github.com/gin-gonic/gin"
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
	ginframework.SetMode(ginframework.TestMode)
	engine := ginframework.New()
	Mount(engine, "/*path", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		context, ok := Context(request.Context())
		if !ok {
			t.Fatal("expected Gin context")
		}
		writer.Header().Set("X-GOWDK-Test", "gin")
		writer.Header().Set("X-GOWDK-Route", context.FullPath())
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
		if recorder.Header().Get("X-GOWDK-Test") != "gin" {
			t.Fatalf("expected wrapped handler header, got %#v", recorder.Header())
		}
		if recorder.Header().Get("X-GOWDK-Route") != "/*path" {
			t.Fatalf("expected attached Gin context route, got %#v", recorder.Header())
		}
	}
}

func TestMountOpenAPIConformance(t *testing.T) {
	ginframework.SetMode(ginframework.TestMode)
	conformance.AssertOpenAPIConformance(t, []byte(openAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := ginframework.New()
		err := MountOpenAPI(engine, []byte(openAPISpec), handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountOpenAPIFallbackConformance(t *testing.T) {
	ginframework.SetMode(ginframework.TestMode)
	conformance.AssertEmptyOpenAPIFallback(t, []byte(emptyOpenAPISpec), "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := ginframework.New()
		err := MountOpenAPI(engine, []byte(emptyOpenAPISpec), handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountOpenAPIRestRouteConformance(t *testing.T) {
	ginframework.SetMode(ginframework.TestMode)
	conformance.AssertOpenAPIRestRoute(t, "/app", func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error) {
		engine := ginframework.New()
		err := MountRoutes(engine, routes, handler, WithPrefix(prefix))
		return engine, err
	})
}

func TestMountRoutesReturnsGinConflict(t *testing.T) {
	ginframework.SetMode(ginframework.TestMode)
	engine := ginframework.New()
	err := MountRoutes(engine, []gowdkadapters.Route{
		{Method: http.MethodGet, Path: "/patients/{id}"},
		{Method: http.MethodGet, Path: "/patients/new"},
	}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if err == nil {
		t.Fatal("expected conflict error")
	}
	message := err.Error()
	if !strings.Contains(message, "gin route conflict") ||
		!strings.Contains(message, "GET /patients/{id}") ||
		!strings.Contains(message, "GET /patients/new") {
		t.Fatalf("expected conflict to name both routes, got %q", message)
	}
}
