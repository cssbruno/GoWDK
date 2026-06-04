package gowdk

import (
	"strings"
	"testing"
)

func TestNewAddonAndEnabledFeatures(t *testing.T) {
	addon := NewAddon("custom", FeatureStatic, FeatureCSS)

	if addon.Name() != "custom" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	features := addon.Features()
	features[0] = FeatureSSR

	enabled := EnabledFeatures(Config{Addons: []Addon{addon}})
	if !enabled.Has(FeatureStatic) || !enabled.Has(FeatureCSS) {
		t.Fatalf("expected static and css features, got %#v", enabled)
	}
	if enabled.Has(FeatureSSR) {
		t.Fatalf("addon features should be copied defensively, got %#v", enabled)
	}
}

func TestConfigHasFeature(t *testing.T) {
	config := Config{Addons: []Addon{NewAddon("ssr", FeatureSSR)}}

	if !config.HasFeature(FeatureSSR) {
		t.Fatal("expected SSR feature")
	}
	if config.HasFeature(FeatureActions) {
		t.Fatal("did not expect actions feature")
	}
}

func TestRenderConfigDefaultMode(t *testing.T) {
	if got := (RenderConfig{}).DefaultMode(); got != Static {
		t.Fatalf("expected static default, got %q", got)
	}
	if got := (RenderConfig{Default: Action}).DefaultMode(); got != Action {
		t.Fatalf("expected configured default, got %q", got)
	}
}

func TestParseRenderModeAndModePredicates(t *testing.T) {
	cases := []struct {
		value       string
		mode        RenderMode
		requiresSSR bool
		buildTime   bool
	}{
		{"static", Static, false, true},
		{"action", Action, false, true},
		{"hybrid", Hybrid, true, false},
		{"ssr", SSR, true, false},
	}

	for _, tc := range cases {
		mode, err := ParseRenderMode(tc.value)
		if err != nil {
			t.Fatalf("ParseRenderMode(%q): %v", tc.value, err)
		}
		if mode != tc.mode {
			t.Fatalf("ParseRenderMode(%q) = %q, want %q", tc.value, mode, tc.mode)
		}
		if mode.RequiresSSR() != tc.requiresSSR {
			t.Fatalf("%q RequiresSSR = %v", mode, mode.RequiresSSR())
		}
		if mode.IsBuildTime() != tc.buildTime {
			t.Fatalf("%q IsBuildTime = %v", mode, mode.IsBuildTime())
		}
	}

	_, err := ParseRenderMode("server")
	if err == nil || !strings.Contains(err.Error(), `unknown render mode "server"`) {
		t.Fatalf("expected unknown mode error, got %v", err)
	}
}
