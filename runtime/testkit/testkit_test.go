package testkit

import (
	"net/http"
	"testing"
)

func TestRunChecksStatusAndHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Test", "ok")
		response.WriteHeader(http.StatusAccepted)
	})

	Run(t, handler, []Scenario{{
		Name:       "accepted",
		Method:     http.MethodPost,
		Path:       "/submit",
		WantStatus: http.StatusAccepted,
		WantHeader: map[string]string{"X-Test": "ok"},
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
