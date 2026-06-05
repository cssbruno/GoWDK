package publicapi_test

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestNewAddonAndEnabledFeatures(t *testing.T) {
	addon := gowdk.NewAddon("custom", gowdk.FeatureSPA, gowdk.FeatureCSS)

	if addon.Name() != "custom" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	features := addon.Features()
	features[0] = gowdk.FeatureSSR

	enabled := gowdk.EnabledFeatures(gowdk.Config{Addons: []gowdk.Addon{addon}})
	if !enabled.Has(gowdk.FeatureSPA) || !enabled.Has(gowdk.FeatureCSS) {
		t.Fatalf("expected spa and css features, got %#v", enabled)
	}
	if enabled.Has(gowdk.FeatureSSR) {
		t.Fatalf("addon features should be copied defensively, got %#v", enabled)
	}
}

func TestConfigHasFeature(t *testing.T) {
	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}

	if !config.HasFeature(gowdk.FeatureSSR) {
		t.Fatal("expected SSR feature")
	}
	if config.HasFeature(gowdk.FeatureActions) {
		t.Fatal("did not expect actions feature")
	}
}

func TestRenderConfigDefaultMode(t *testing.T) {
	if got := (gowdk.RenderConfig{}).DefaultMode(); got != gowdk.SPA {
		t.Fatalf("expected spa default, got %q", got)
	}
	if got := (gowdk.RenderConfig{Default: gowdk.Action}).DefaultMode(); got != gowdk.Action {
		t.Fatalf("expected configured default, got %q", got)
	}
}

func TestBuildConfigDebugAssets(t *testing.T) {
	if !(gowdk.BuildConfig{}).DebugAssets() {
		t.Fatal("expected omitted build mode to include debug assets")
	}
	if !(gowdk.BuildConfig{Mode: gowdk.Development}).DebugAssets() {
		t.Fatal("expected development mode to include debug assets")
	}
	if (gowdk.BuildConfig{Mode: gowdk.Production}).DebugAssets() {
		t.Fatal("did not expect production mode to include debug assets")
	}
}

func TestParseRenderModeAndModePredicates(t *testing.T) {
	cases := []struct {
		value       string
		mode        gowdk.RenderMode
		requiresSSR bool
		buildTime   bool
	}{
		{"spa", gowdk.SPA, false, true},
		{"action", gowdk.Action, false, true},
		{"hybrid", gowdk.Hybrid, true, false},
		{"ssr", gowdk.SSR, true, false},
	}

	for _, tc := range cases {
		mode, err := gowdk.ParseRenderMode(tc.value)
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

	_, err := gowdk.ParseRenderMode("server")
	if err == nil || !strings.Contains(err.Error(), `unknown render mode "server"`) {
		t.Fatalf("expected unknown mode error, got %v", err)
	}
}
