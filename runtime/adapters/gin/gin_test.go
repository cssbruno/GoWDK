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
		if request.URL.Path != "/docs" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("X-GOWDK-Test", "gin")
		_, _ = writer.Write([]byte("ok"))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-GOWDK-Test") != "gin" {
		t.Fatalf("expected wrapped handler header, got %#v", recorder.Header())
	}
}
