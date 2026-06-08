package echo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	echoframework "github.com/labstack/echo/v5"
)

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
