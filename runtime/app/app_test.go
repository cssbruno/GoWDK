package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cssbruno/gowdk/runtime/asset"
)

func TestHandlerServesStaticIndexAndIdentityHeaders(t *testing.T) {
	handler := Handler{
		Root: fstest.MapFS{
			"index.html": {Data: []byte("<main>Home</main>")},
		},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets:   asset.Manifest{Version: 1, Files: map[string]string{}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "<main>Home</main>" {
		t.Fatalf("unexpected body: %s", body)
	}
	if recorder.Header().Get("X-GOWDK-App") != "clinic" {
		t.Fatalf("unexpected app header: %q", recorder.Header().Get("X-GOWDK-App"))
	}
}

func TestHandlerHealth(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Assets: asset.Manifest{Version: 1, Files: map[string]string{
			"assets/app.css": "assets/app.css",
		}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/_gowdk/health", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	for _, expected := range []string{`"status":"ok"`, `"app":"clinic"`, `"assets":"1"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("expected health response to contain %q, got %s", expected, recorder.Body.String())
		}
	}
}

func TestHandlerDelegatesAction(t *testing.T) {
	called := false
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		Action: func(response http.ResponseWriter, request *http.Request) bool {
			called = true
			response.WriteHeader(http.StatusNoContent)
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/submit", nil)

	handler.ServeHTTP(recorder, request)

	if !called {
		t.Fatal("expected action hook to run")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerUsesDynamicSSRAfterStaticMiss(t *testing.T) {
	handler := Handler{
		Root:     fstest.MapFS{},
		Identity: Identity{AppID: "app", ModuleName: "app", InstanceID: "app-1"},
		SSRDynamic: func(response http.ResponseWriter, request *http.Request) bool {
			_, _ = response.Write([]byte("<main>SSR</main>"))
			return true
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/post/hello", nil)

	handler.ServeHTTP(recorder, request)

	payload, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "<main>SSR</main>" {
		t.Fatalf("unexpected body: %s", payload)
	}
}

func TestLoadAssetManifest(t *testing.T) {
	manifest := LoadAssetManifest(fstest.MapFS{
		"gowdk-assets.json": {Data: []byte(`{"version":1,"files":{"assets/app.css":"assets/app.css"}}`)},
	})
	if manifest.Resolve("assets/app.css") != "assets/app.css" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}
