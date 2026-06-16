package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryStoresHTTPHandlers(t *testing.T) {
	registry := Registry{
		"health": func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		},
	}

	recorder := httptest.NewRecorder()
	registry["health"](recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}
