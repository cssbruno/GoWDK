package ssr

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersSSRFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "ssr" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureSSR) {
		t.Fatal("expected ssr feature")
	}
}

func TestRuntimeReExports(t *testing.T) {
	data := map[string]any{"user": map[string]any{"name": "Ada"}}
	value, ok := LoadPath(data, "user.name")
	if !ok || value != "Ada" {
		t.Fatalf("unexpected load path result: %#v, %v", value, ok)
	}
}
