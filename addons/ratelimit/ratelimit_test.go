package ratelimit

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersRateLimitFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "ratelimit" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureRateLimit) {
		t.Fatal("expected ratelimit feature")
	}
}
