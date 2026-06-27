package api

import (
	"testing"

	"github.com/cssbruno/gowdk"
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
