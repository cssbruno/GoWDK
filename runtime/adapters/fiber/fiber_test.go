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
		context, ok := Context(request.Context())
		if !ok {
			t.Fatal("expected Fiber context")
		}
		writer.Header().Set("X-GOWDK-Test", "fiber")
		writer.Header().Set("X-GOWDK-Route", context.Route().Path)
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
		request, err := http.NewRequest(test.method, test.path, nil)
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

		if response.StatusCode != http.StatusOK || string(body) != test.method+" "+test.path {
			t.Fatalf("unexpected response for %s %s: %d %q", test.method, test.path, response.StatusCode, body)
		}
		if response.Header.Get("X-GOWDK-Test") != "fiber" {
			t.Fatalf("expected wrapped handler header, got %#v", response.Header)
		}
		if response.Header.Get("X-GOWDK-Route") != "/*" {
			t.Fatalf("expected attached Fiber context route, got %#v", response.Header)
		}
	}
}
