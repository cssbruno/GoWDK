package fiber

import (
	"io"
	"net/http"
	"testing"

	fiberframework "github.com/gofiber/fiber/v2"
)

func TestHandlerWrapsHTTPHandler(t *testing.T) {
	app := fiberframework.New()
	Mount(app, "/*", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/docs" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("X-GOWDK-Test", "fiber")
		_, _ = writer.Write([]byte("ok"))
	}))

	request, err := http.NewRequest(http.MethodGet, "/docs", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("unexpected response: %d %q", response.StatusCode, body)
	}
	if response.Header.Get("X-GOWDK-Test") != "fiber" {
		t.Fatalf("expected wrapped handler header, got %#v", response.Header)
	}
}
