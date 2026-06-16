package ratelimit

import (
	"testing"
	"time"

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

func TestRuntimeReExports(t *testing.T) {
	limiter, err := New(Options{
		Limit:  1,
		Window: time.Minute,
		Store:  NewInMemoryStore(InMemoryOptions{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if limiter == nil {
		t.Fatal("expected limiter")
	}
}
