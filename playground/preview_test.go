package playground

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreviewServerRendersCompilerAssets(t *testing.T) {
	server := NewPreviewServer(PreviewOptions{
		AssetPathPrefix: "/playground/assets/",
		ActionPath:      "/playground/preview-post",
	})
	result := Result{
		Files: map[string]string{
			"index.html":                        `<form action="/" method="get" target="_blank"><button>Save</button></form><script src="/assets/gowdk/islands/Counter.js"></script>`,
			"assets/gowdk/islands/Counter.js":   `window.counter = true;`,
			"assets/gowdk/islands/Counter.css":  `.counter{color:red}`,
			"gowdk-assets.json":                 `{"version":1}`,
			"assets/gowdk/islands/Counter.wasm": "wasm",
		},
		HTML: map[string]string{
			"index.html": `<form action="/" method="get" target="_blank"><button>Save</button></form><script src="/assets/gowdk/islands/Counter.js"></script>`,
		},
	}

	preview := server.Render(result)

	if preview.HTMLPath != "index.html" {
		t.Fatalf("unexpected preview HTML path: %q", preview.HTMLPath)
	}
	if preview.Token == "" {
		t.Fatal("expected preview token")
	}
	if strings.Contains(preview.HTML, `target="_blank"`) || strings.Contains(preview.HTML, `method="get"`) {
		t.Fatalf("expected form to target preview post route:\n%s", preview.HTML)
	}
	if !strings.Contains(preview.HTML, `method="post" action="/playground/preview-post"`) {
		t.Fatalf("expected rewritten form:\n%s", preview.HTML)
	}

	assetPath := "/playground/assets/" + preview.Token + "/assets/gowdk/islands/Counter.js"
	if !strings.Contains(preview.HTML, assetPath) {
		t.Fatalf("expected compiler asset route %q in preview:\n%s", assetPath, preview.HTML)
	}

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected asset 200, got %d", response.Code)
	}
	if strings.TrimSpace(response.Body.String()) != "window.counter = true;" {
		t.Fatalf("unexpected asset body: %q", response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "javascript") {
		t.Fatalf("expected javascript content type, got %q", contentType)
	}
}

func TestPreviewServerRejectsInvalidAssets(t *testing.T) {
	server := NewPreviewServer(PreviewOptions{})
	preview := server.Render(Result{
		Files: map[string]string{"assets/app.js": "window.ok = true;"},
		HTML:  map[string]string{"index.html": `<script src="/assets/app.js"></script>`},
	})

	for _, requestPath := range []string{
		"/playground/assets/" + preview.Token + "/../secret",
		"/playground/assets/" + preview.Token + "/gowdk-assets.json",
		"/playground/assets/missing/assets/app.js",
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %q, got %d", requestPath, response.Code)
		}
	}
}
