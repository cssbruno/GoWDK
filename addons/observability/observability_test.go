package observability

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonEnablesObservabilityFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "observability" {
		t.Fatalf("addon.Name() = %q, want observability", addon.Name())
	}
	config := gowdk.Config{Addons: []gowdk.Addon{addon}}
	if !config.HasFeature(gowdk.FeatureObservability) {
		t.Fatal("expected observability feature")
	}
}
