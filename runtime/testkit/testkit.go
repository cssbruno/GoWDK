// Package testkit provides small helpers for generated runtime integration
// tests.
package testkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Scenario describes one HTTP expectation against a generated app handler.
type Scenario struct {
	Name       string
	Method     string
	Path       string
	Body       string
	Headers    map[string]string
	WantStatus int
	WantHeader map[string]string
}

// Run executes scenarios against handler.
func Run(t testing.TB, handler http.Handler, scenarios []Scenario) {
	t.Helper()
	if runner, ok := t.(interface {
		Run(string, func(*testing.T)) bool
	}); ok {
		for _, scenario := range scenarios {
			scenario := scenario
			runner.Run(scenarioName(scenario), func(t *testing.T) {
				t.Helper()
				runScenario(t, handler, scenario)
			})
		}
		return
	}
	for _, scenario := range scenarios {
		runScenario(t, handler, scenario)
	}
}

func runScenario(t testing.TB, handler http.Handler, scenario Scenario) {
	t.Helper()
	response := Response(handler, scenario)
	if scenario.WantStatus != 0 && response.Code != scenario.WantStatus {
		t.Errorf("expected status %d, got %d with body %s", scenario.WantStatus, response.Code, responseBodySummary(response.Body.String()))
	}
	for name, want := range scenario.WantHeader {
		if got := response.Header().Get(name); got != want {
			t.Errorf("expected header %s=%q, got %q", name, want, got)
		}
	}
}

// AssertStatus checks one request's response status.
func AssertStatus(t testing.TB, handler http.Handler, method, requestPath, body string, want int) {
	t.Helper()
	Run(t, handler, []Scenario{{
		Name:       method + " " + requestPath,
		Method:     method,
		Path:       requestPath,
		Body:       body,
		WantStatus: want,
	}})
}

// AssertHeader checks one response header value.
func AssertHeader(t testing.TB, handler http.Handler, method, requestPath, name, want string) {
	t.Helper()
	Run(t, handler, []Scenario{{
		Name:       method + " " + requestPath,
		Method:     method,
		Path:       requestPath,
		WantHeader: map[string]string{name: want},
	}})
}

// Response executes one scenario and returns the recorder for custom checks.
func Response(handler http.Handler, scenario Scenario) *httptest.ResponseRecorder {
	method := strings.TrimSpace(scenario.Method)
	if method == "" {
		method = http.MethodGet
	}
	requestPath := strings.TrimSpace(scenario.Path)
	if requestPath == "" {
		requestPath = "/"
	}
	request := httptest.NewRequest(method, requestPath, strings.NewReader(scenario.Body))
	for name, value := range scenario.Headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func scenarioName(scenario Scenario) string {
	if strings.TrimSpace(scenario.Name) != "" {
		return scenario.Name
	}
	return strings.TrimSpace(scenario.Method + " " + scenario.Path)
}

func responseBodySummary(body string) string {
	if body == "" {
		return "<empty>"
	}
	return fmt.Sprintf("<%d byte response body redacted>", len(body))
}
