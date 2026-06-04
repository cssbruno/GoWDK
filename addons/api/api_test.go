package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gowdk/gowdk"
)

func TestAddonRegistersAPIFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "api" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureAPI) {
		t.Fatal("expected api feature")
	}
}

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
