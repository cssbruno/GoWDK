package partial

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestAddonRegistersPartialFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "partial" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeaturePartial) {
		t.Fatal("expected partial feature")
	}
}

func TestRuntimeReExports(t *testing.T) {
	result := Fragment("#messages", "<li>Hello</li>")
	if result.Kind != response.Fragment || result.Target != "#messages" || result.Body != "<li>Hello</li>" {
		t.Fatalf("unexpected fragment response: %#v", result)
	}
}
