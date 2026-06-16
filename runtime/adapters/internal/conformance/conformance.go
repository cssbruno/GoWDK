package conformance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"
)

// MountFunc mounts routes into a host framework and returns the host handler.
type MountFunc func(routes []gowdkadapters.Route, handler http.Handler, prefix string) (http.Handler, error)

// AssertOpenAPIConformance verifies that an adapter serves every OpenAPI
// method/path route through the generated GOWDK handler and strips the host
// mount prefix before dispatch.
func AssertOpenAPIConformance(t *testing.T, spec []byte, prefix string, mount MountFunc) {
	t.Helper()

	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) == 0 {
		t.Fatal("expected at least one OpenAPI route")
	}
	assertServerURL(t, spec, prefix)

	expected := expectedGeneratedRequests(t, routes)
	expected[http.MethodGet+" /assets/app.css"] = true
	gowdkHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !expected[request.Method+" "+request.URL.Path] {
			http.Error(writer, "unexpected generated route", http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte("matched"))
	})
	host, err := mount(routes, gowdkHandler, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if host == nil {
		t.Fatal("mount returned nil host handler")
	}

	for _, route := range routes {
		gowdkPath := samplePath(route.Path)
		hostPath, err := gowdkadapters.JoinPrefix(prefix, gowdkPath)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.Method, hostPath, nil)
		host.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "matched" {
			t.Fatalf("unexpected response for host route %s %s: status=%d body=%q", route.Method, hostPath, recorder.Code, recorder.Body.String())
		}
	}
	hostPath, err := gowdkadapters.JoinPrefix(prefix, "/assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, hostPath, nil)
	host.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "matched" {
		t.Fatalf("unexpected fallback response for host route GET %s: status=%d body=%q", hostPath, recorder.Code, recorder.Body.String())
	}
}

// AssertEmptyOpenAPIFallback verifies that MountOpenAPI still mounts generated
// static-only apps whose OpenAPI report has no endpoint paths.
func AssertEmptyOpenAPIFallback(t *testing.T, spec []byte, prefix string, mount MountFunc) {
	t.Helper()

	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected no OpenAPI routes, got %#v", routes)
	}
	assertServerURL(t, spec, prefix)

	expected := map[string]bool{
		http.MethodGet + " /":               true,
		http.MethodGet + " /assets/app.css": true,
	}
	gowdkHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !expected[request.Method+" "+request.URL.Path] {
			http.Error(writer, "unexpected generated route", http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte("matched"))
	})
	host, err := mount(routes, gowdkHandler, prefix)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/", "/assets/app.css"} {
		hostPath, err := gowdkadapters.JoinPrefix(prefix, path)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, hostPath, nil)
		host.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "matched" {
			t.Fatalf("unexpected fallback response for host route GET %s: status=%d body=%q", hostPath, recorder.Code, recorder.Body.String())
		}
	}
}

// AssertOpenAPIRestRoute verifies that adapters use x-gowdk.route to preserve
// final rest-param semantics lost by standard OpenAPI path syntax.
func AssertOpenAPIRestRoute(t *testing.T, prefix string, mount MountFunc) {
	t.Helper()

	spec := []byte(`{
  "openapi": "3.1.0",
  "servers": [{"url": "/app"}],
  "paths": {
    "/docs/{path}": {
      "get": {
        "x-gowdk": {"route": "/docs/{path...}"},
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`)
	routes, err := gowdkadapters.RoutesFromOpenAPI(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Path != "/docs/{path...}" {
		t.Fatalf("expected GOWDK rest route from OpenAPI, got %#v", routes)
	}

	gowdkHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/docs/one/two" {
			http.Error(writer, "unexpected generated route", http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte("matched"))
	})
	host, err := mount(routes, gowdkHandler, prefix)
	if err != nil {
		t.Fatal(err)
	}
	hostPath, err := gowdkadapters.JoinPrefix(prefix, "/docs/one/two")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, hostPath, nil)
	host.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "matched" {
		t.Fatalf("unexpected rest route response for host route GET %s: status=%d body=%q", hostPath, recorder.Code, recorder.Body.String())
	}
}

// AssertPrefixedRedirect verifies that prefixed mounts rebase generated
// same-origin redirects back under the host-framework prefix.
func AssertPrefixedRedirect(t *testing.T, prefix string, mount MountFunc) {
	t.Helper()

	routes := []gowdkadapters.Route{{Method: http.MethodPost, Path: "/login"}}
	gowdkHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/login" {
			http.Error(writer, "unexpected generated route", http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/dashboard", http.StatusSeeOther)
	})
	host, err := mount(routes, gowdkHandler, prefix)
	if err != nil {
		t.Fatal(err)
	}
	hostPath, err := gowdkadapters.JoinPrefix(prefix, "/login")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, hostPath, nil)
	host.ServeHTTP(recorder, request)
	wantLocation, err := gowdkadapters.JoinPrefix(prefix, "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != wantLocation {
		t.Fatalf("unexpected prefixed redirect: status=%d location=%q want %q", recorder.Code, recorder.Header().Get("Location"), wantLocation)
	}
}

func expectedGeneratedRequests(t *testing.T, routes []gowdkadapters.Route) map[string]bool {
	t.Helper()
	expected := map[string]bool{}
	for _, route := range routes {
		expected[route.Method+" "+samplePath(route.Path)] = true
	}
	return expected
}

func assertServerURL(t *testing.T, spec []byte, prefix string) {
	t.Helper()
	want := "/"
	if normalized, err := gowdkadapters.NormalizeMountPrefix(prefix); err == nil && normalized != "" {
		want = normalized
	}
	if !strings.Contains(string(spec), fmt.Sprintf(`"url": %q`, want)) {
		t.Fatalf("openapi spec does not contain server URL %q:\n%s", want, spec)
	}
}

func samplePath(routePath string) string {
	clean, err := gowdkadapters.CleanRoutePath(routePath)
	if err != nil {
		return routePath
	}
	if clean == "/" {
		return "/"
	}
	segments := strings.Split(strings.Trim(clean, "/"), "/")
	for index, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		name = strings.TrimSuffix(name, "...")
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		if strings.EqualFold(name, "id") {
			segments[index] = "42"
			continue
		}
		segments[index] = strings.ToLower(name) + "-value"
	}
	return "/" + strings.Join(segments, "/")
}
