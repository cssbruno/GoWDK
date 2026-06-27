package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSPreflightVarySelectorsForWildcardOrigin(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Kind:    "api",
		Method:  http.MethodGet,
		Path:    "/api/items",
		Handler: func(http.ResponseWriter, *http.Request) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := router.SetCORSPolicy(CORSPolicy{AllowedOrigins: []string{"*"}}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/items", nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)

	if !router.Dispatch(recorder, request) {
		t.Fatal("expected preflight to be handled")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected preflight status: %d", recorder.Code)
	}
	assertVaryTokens(t, recorder.Header(), map[string]int{
		"access-control-request-method":  1,
		"access-control-request-headers": 1,
	})
}

func TestCORSPreflightVaryMergesExistingValuesCaseInsensitively(t *testing.T) {
	router, err := NewBackendRouter(BackendRoute{
		Kind:    "api",
		Method:  http.MethodPost,
		Path:    "/api/items",
		Handler: func(http.ResponseWriter, *http.Request) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := router.SetCORSPolicy(CORSPolicy{
		AllowedOrigins: []string{"https://app.example"},
		AllowedHeaders: []string{"Content-Type"},
	}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	recorder.Header().Add("Vary", "Accept-Encoding, origin")
	recorder.Header().Add("Vary", "ACCESS-CONTROL-REQUEST-METHOD")
	request := httptest.NewRequest(http.MethodOptions, "/api/items", nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type")

	if !router.Dispatch(recorder, request) {
		t.Fatal("expected preflight to be handled")
	}
	assertVaryTokens(t, recorder.Header(), map[string]int{
		"accept-encoding":                1,
		"origin":                         1,
		"access-control-request-method":  1,
		"access-control-request-headers": 1,
	})
}

func TestAddVaryHeaderTreatsWildcardAsTerminal(t *testing.T) {
	header := http.Header{}
	header.Add("Vary", "Accept-Encoding")
	header.Add("Vary", "*")
	before := append([]string(nil), header.Values("Vary")...)

	addVaryHeader(header, "Origin", "Access-Control-Request-Method")

	after := header.Values("Vary")
	if len(after) != len(before) {
		t.Fatalf("Vary: * must prevent additions: before=%q after=%q", before, after)
	}
	for index := range before {
		if before[index] != after[index] {
			t.Fatalf("Vary: * must preserve existing values: before=%q after=%q", before, after)
		}
	}
}

func assertVaryTokens(t *testing.T, header http.Header, expected map[string]int) {
	t.Helper()
	actual := map[string]int{}
	for _, line := range header.Values("Vary") {
		for _, token := range strings.Split(line, ",") {
			token = strings.ToLower(strings.TrimSpace(token))
			if token != "" {
				actual[token]++
			}
		}
	}
	if len(actual) != len(expected) {
		t.Fatalf("unexpected Vary tokens: got %#v want %#v", actual, expected)
	}
	for token, count := range expected {
		if actual[token] != count {
			t.Fatalf("unexpected Vary token %q count: got %d want %d (all=%#v)", token, actual[token], count, actual)
		}
	}
}
