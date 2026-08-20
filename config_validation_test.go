package gowdk

import (
	"strings"
	"testing"
)

func TestConfigValidateStructuralRejectsInvalidStates(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "render mode", config: Config{Render: RenderConfig{Default: "magic"}}, want: "Render.Default"},
		{name: "build mode", config: Config{Build: BuildConfig{Mode: "fast"}}, want: "Build.Mode"},
		{name: "asset mode", config: Config{Build: BuildConfig{Assets: "inline"}}, want: "Build.Assets"},
		{name: "obfuscation mode", config: Config{Build: BuildConfig{ObfuscateAssets: true}}, want: "requires Build.Mode"},
		{name: "disabled cors fields", config: Config{Build: BuildConfig{CORS: CORSConfig{AllowedOrigins: []string{"https://example.com"}}}}, want: "require Enabled"},
		{name: "disabled csrf fields", config: Config{Build: BuildConfig{CSRF: CSRFConfig{Disabled: true, SecretEnv: "SECRET"}}}, want: "cannot be combined"},
		{name: "security header state", config: Config{Build: BuildConfig{SecurityHeaders: SecurityHeadersConfig{Headers: map[string]string{"X-Test": "yes"}}}}, want: "requires Enabled"},
		{name: "negative body limit", config: Config{Build: BuildConfig{BodyLimits: BodyLimitsConfig{APIBytes: -1}}}, want: "APIBytes"},
		{name: "duplicate module", config: Config{Modules: []ModuleConfig{{Name: "site"}, {Name: "site"}}}, want: "declared more than once"},
		{name: "binary without app", config: Config{Build: BuildConfig{Targets: []BuildTargetConfig{{Name: "site", Binary: "bin/site"}}}}, want: "Binary requires App"},
		{name: "duplicate target", config: Config{Build: BuildConfig{Targets: []BuildTargetConfig{{Name: "site"}, {Name: "site"}}}}, want: "declared more than once"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.ValidateStructural()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateStructural() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestConfigValidateStructuralAcceptsDefaultAndExplicitBuild(t *testing.T) {
	for _, config := range []Config{
		{},
		{Render: RenderConfig{Default: SPA}, Build: BuildConfig{Mode: Production, Assets: Embed, ObfuscateAssets: true, Targets: []BuildTargetConfig{{Name: "site", App: ".gowdk/site", Binary: "bin/site"}}}},
	} {
		if err := config.ValidateStructural(); err != nil {
			t.Fatalf("ValidateStructural() = %v", err)
		}
	}
}
