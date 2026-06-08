package gin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ginframework "github.com/gin-gonic/gin"
)

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
