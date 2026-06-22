package testkit

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func TestRunChecksStatusAndHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Test", "ok")
		response.WriteHeader(http.StatusAccepted)
		_, _ = response.Write([]byte("accepted by test handler"))
	})

	Run(t, handler, []Scenario{{
		Name:             "accepted",
		Method:           http.MethodPost,
		Path:             "/submit",
		WantStatus:       http.StatusAccepted,
		WantHeader:       map[string]string{"X-Test": "ok"},
		WantBodyContains: "test handler",
	}})
}

func TestAssertHelpers(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Frame-Options", "DENY")
		response.WriteHeader(http.StatusNoContent)
	})

	AssertStatus(t, handler, http.MethodGet, "/", "", http.StatusNoContent)
	AssertHeader(t, handler, http.MethodGet, "/", "X-Frame-Options", "DENY")
}

func TestClientKeepsCookiesAcrossDirectRequests(t *testing.T) {
	handler := testSessionHandler(t)
	client := NewClient(t, handler)

	login := client.PostForm(t, "/login", url.Values{"email": []string{"ada@example.com"}})
	login.AssertStatus(t, http.StatusNoContent)
	login.AssertCookie(t, "sid")

	dashboard := client.Get(t, "/dashboard")
	dashboard.AssertStatus(t, http.StatusOK)
	dashboard.AssertBodyContains(t, "session ok")
}

func TestServerClientKeepsCookiesAcrossTransportRequests(t *testing.T) {
	handler := testSessionHandler(t)
	client := NewServerClient(t, handler)

	client.PostForm(t, "/login", url.Values{"email": []string{"ada@example.com"}}).AssertStatus(t, http.StatusNoContent)
	client.Get(t, "/dashboard").AssertStatus(t, http.StatusOK)
}

func TestClientBuildsJSONRequestsAndAssertions(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Query().Get("mode") != "strict" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("X-Test") != "1" {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.Header().Set("X-GOWDK-Fragment-Target", "#result")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"ok":    true,
			"email": payload["email"],
		})
	})
	client := NewClient(t, handler)

	result := client.Do(t, PostJSON("/api/session", map[string]string{"email": "ada@example.com"}).
		WithQuery("mode", "strict").
		WithHeader("X-Test", "1"))

	result.AssertStatusRange(t, 200, 299)
	result.AssertContentType(t, "application/json")
	result.AssertHeaderContains(t, "X-GOWDK-Fragment-Target", "result")
	result.AssertJSONEqual(t, map[string]any{"ok": true, "email": "ada@example.com"})
}

func TestRequestBuilderMethodsDoNotMutateOriginal(t *testing.T) {
	original := Get("/api").
		WithHeader("X-Original", "1").
		WithQuery("page", "1")

	derived := original.
		WithHeader("X-Derived", "1").
		WithQuery("mode", "strict")

	if _, ok := original.Headers["X-Derived"]; ok {
		t.Fatalf("original headers were mutated: %#v", original.Headers)
	}
	if got := original.Query.Get("mode"); got != "" {
		t.Fatalf("original query was mutated: %#v", original.Query)
	}
	if derived.Headers["X-Original"] != "1" || derived.Query.Get("page") != "1" {
		t.Fatalf("derived request lost original values: %#v %#v", derived.Headers, derived.Query)
	}
}

func TestResponseBodySummaryRedactsPayload(t *testing.T) {
	body := `secret_token=live_sk_abc123`
	summary := responseBodySummary(body)
	if summary == body || summary == "" {
		t.Fatalf("expected redacted body summary, got %q", summary)
	}
}

func testSessionHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if request.Form.Get("email") != "ada@example.com" {
				t.Fatalf("unexpected form: %#v", request.Form)
			}
			http.SetCookie(response, &http.Cookie{Name: "sid", Value: "session-1", Path: "/"})
			response.WriteHeader(http.StatusNoContent)
		case "/dashboard":
			cookie, err := request.Cookie("sid")
			if err != nil || cookie.Value != "session-1" {
				http.Error(response, "missing session", http.StatusUnauthorized)
				return
			}
			_, _ = response.Write([]byte("session ok"))
		default:
			http.NotFound(response, request)
		}
	})
}
