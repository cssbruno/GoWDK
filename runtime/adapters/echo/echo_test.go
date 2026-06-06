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
		if request.URL.Path != "/docs" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("X-GOWDK-Test", "echo")
		_, _ = writer.Write([]byte("ok"))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-GOWDK-Test") != "echo" {
		t.Fatalf("expected wrapped handler header, got %#v", recorder.Header())
	}
}
