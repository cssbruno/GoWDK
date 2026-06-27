package partial

import (
	"testing"

	"github.com/cssbruno/gowdk"
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
