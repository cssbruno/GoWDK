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

func TestValidateAddonsRejectsInvalidIdentityAndOwnership(t *testing.T) {
	tests := []struct {
		name   string
		addons []gowdk.Addon
		want   string
	}{
		{name: "nil", addons: []gowdk.Addon{nil}, want: "addons[0] is nil"},
		{name: "empty name", addons: []gowdk.Addon{gowdk.NewAddon("", gowdk.FeatureCSS)}, want: "Name is required"},
		{name: "empty features", addons: []gowdk.Addon{gowdk.NewAddon("empty")}, want: "at least one feature"},
		{name: "empty feature", addons: []gowdk.Addon{gowdk.NewAddon("empty-feature", gowdk.Feature(""))}, want: "empty feature"},
		{name: "duplicate names", addons: []gowdk.Addon{
			gowdk.NewAddon("css", gowdk.FeatureCSS),
			gowdk.NewAddon("css", gowdk.FeatureSEO),
		}, want: "duplicates addons[0]"},
		{name: "duplicate features", addons: []gowdk.Addon{
			gowdk.NewAddon("css-a", gowdk.FeatureCSS),
			gowdk.NewAddon("css-b", gowdk.FeatureCSS),
		}, want: "duplicates feature"},
		{name: "seo provider mismatch", addons: []gowdk.Addon{
			gowdk.NewAddon("seo-marker", gowdk.FeatureSEO),
		}, want: "does not implement gowdk.SEOProvider"},
		{name: "auth provider mismatch", addons: []gowdk.Addon{
			gowdk.NewAddon("auth-marker", gowdk.FeatureAuth),
		}, want: "does not implement gowdk.AuthSessionProvider"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := gowdk.ValidateAddons(test.addons)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
		})
	}
}

func TestValidateAddonsAllowsExplicitSPAAliasFeatureOverlap(t *testing.T) {
	err := gowdk.ValidateAddons([]gowdk.Addon{
		gowdk.NewAddon("spa", gowdk.FeatureSPA),
		gowdk.NewAddon("static", gowdk.FeatureSPA),
	})
	if err != nil {
		t.Fatalf("expected legacy SPA/static feature overlap to be allowed, got %v", err)
	}
}

func TestRenderConfigDefaultMode(t *testing.T) {
	if got := (gowdk.RenderConfig{}).DefaultMode(); got != gowdk.SPA {
		t.Fatalf("expected spa default, got %q", got)
	}
	if got := (gowdk.RenderConfig{Default: gowdk.SSR}).DefaultMode(); got != gowdk.SSR {
		t.Fatalf("expected configured default, got %q", got)
	}
}

func TestEnvConfigValidateRejectsMisuse(t *testing.T) {
	err := gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{},
			{Name: "GOWDK_TEST_API_TOKEN"},
			{Name: "GOWDK_TEST_DATABASE_URL"},
		},
		Secrets: []gowdk.SecretEnv{
			{},
			{Name: "GOWDK_TEST_DATABASE_URL"},
		},
	}.Validate(nil)
	if err == nil {
		t.Fatal("expected env validation error")
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_API_TOKEN looks like a secret") {
		t.Fatalf("expected secret-looking var error, got %v", err)
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_DATABASE_URL is declared more than once in Env.Vars and Env.Secrets") {
		t.Fatalf("expected duplicate env error, got %v", err)
	}
	if !strings.Contains(err.Error(), "environment variable name is required") {
		t.Fatalf("expected empty env var error, got %v", err)
	}
	if !strings.Contains(err.Error(), "secret environment variable name is required") {
		t.Fatalf("expected empty secret env error, got %v", err)
	}
}

func TestEnvConfigValidateReportsMissingRequiredNames(t *testing.T) {
	err := gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{Name: "GOWDK_TEST_BACKEND_ORIGIN", Required: true},
			{Name: "GOWDK_TEST_ADDR", Required: true, Default: "127.0.0.1:8080"},
		},
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
		},
	}.Validate(func(name string) (string, bool) {
		return "   ", true
	})
	if err == nil {
		t.Fatal("expected missing env validation error")
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_BACKEND_ORIGIN is required but is not set") {
		t.Fatalf("expected missing var error, got %v", err)
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_DATABASE_URL is required but is not set") {
		t.Fatalf("expected missing secret error, got %v", err)
	}
	if strings.Contains(err.Error(), "GOWDK_TEST_ADDR is required but is not set") {
		t.Fatalf("var with default should not be reported missing, got %v", err)
	}
}

func TestEnvConfigValidateRejectsShortSecret(t *testing.T) {
	config := gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_TEST_SESSION_SECRET", Required: true, MinBytes: 32},
		},
	}

	short := config.Validate(func(string) (string, bool) { return "too-short", true })
	if short == nil || !strings.Contains(short.Error(), "GOWDK_TEST_SESSION_SECRET must be at least 32 bytes") {
		t.Fatalf("expected short-secret error, got %v", short)
	}

	missing := config.Validate(func(string) (string, bool) { return "", true })
	if missing == nil || !strings.Contains(missing.Error(), "GOWDK_TEST_SESSION_SECRET is required but is not set") {
		t.Fatalf("expected missing-secret error, got %v", missing)
	}

	if err := config.Validate(func(string) (string, bool) {
		return "0123456789ABCDEF0123456789ABCDEF", true
	}); err != nil {
		t.Fatalf("expected a 32-byte secret to pass, got %v", err)
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
		{"hybrid", gowdk.Hybrid, false, false},
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
	_, err = gowdk.ParseRenderMode("action")
	if err == nil || !strings.Contains(err.Error(), `unknown render mode "action"`) {
		t.Fatalf("expected removed action mode error, got %v", err)
	}
}
