package css

import (
	"testing"

	"github.com/gowdk/gowdk"
)

func TestAddonRegistersCSSFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "css" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !gowdk.EnabledFeatures(gowdk.Config{Addons: []gowdk.Addon{addon}}).Has(gowdk.FeatureCSS) {
		t.Fatal("expected css feature")
	}
}
