package actions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersActionsFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "actions" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureActions) {
		t.Fatal("expected actions feature")
	}
}

func TestRuntimeReExports(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader("email=a%40example.com"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	values, err := DecodeForm(request)
	if err != nil {
		t.Fatal(err)
	}
	if got := values.First("email"); got != "a@example.com" {
		t.Fatalf("unexpected email: %q", got)
	}
}
