package project

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

func TestLoadConfigFileReadsLiteralSourceAndBuildFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	AppName: "Example App",
	Source: gowdk.SourceConfig{
		Include: []string{
			"src/**/*.gwdk",
			"modules/**/*.gwdk",
		},
		Exclude: []string{"src/**/draft.page.gwdk"},
	},
	Modules: []gowdk.ModuleConfig{
		{
			Name: "frontend",
			Type: "frontend",
			Source: gowdk.SourceConfig{
				Include: []string{"frontend/**/*.gwdk"},
				Exclude: []string{"frontend/**/draft.page.gwdk"},
			},
		},
		{
			Name: "backendmicroservice",
			Type: "backendmicroservice",
			Source: gowdk.SourceConfig{
				Include: []string{"backend/**/*.gwdk"},
			},
		},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
		Mode: gowdk.Production,
		Head: gowdk.HeadConfig{
			SiteName: "Example",
			Favicon: "/favicon.ico",
			Image: "https://example.com/social.png",
			TwitterCard: "summary_large_image",
		},
		CSRF: gowdk.CSRFConfig{
			Enabled: true,
			SecretEnv: "EXAMPLE_CSRF_SECRET",
			CookieName: "__Host-example-csrf",
			FieldName: "_example_csrf",
			HeaderName: "X-Example-CSRF",
			Insecure: true,
		},
		BodyLimits: gowdk.BodyLimitsConfig{
			ActionBytes: 2097152,
			APIBytes: 524288,
		},
		AllowMissingBackend: true,
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/app.css"},
			{Href: "/assets/theme.css"},
		},
		Scripts: []gowdk.Script{
			{Src: "/assets/app.js", Type: "module"},
			{Src: "/assets/legacy.js"},
		},
		Targets: []gowdk.BuildTargetConfig{
			{
				Name: "admin",
				Modules: []string{"admin"},
				Output: "dist/admin",
				App: ".gowdk/admin",
				Binary: "bin/admin",
				WASM: "bin/admin.wasm",
			},
			{
				Name: "public-admin",
				Modules: []string{"public", "admin"},
				Output: "dist/public-admin",
			},
		},
	},
	Render: gowdk.RenderConfig{
		Default: gowdk.Action,
	},
	Env: gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{Name: "GOWDK_TEST_BACKEND_ORIGIN", Required: true},
			{Name: "GOWDK_TEST_ADDR", Default: "127.0.0.1:8080"},
		},
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_TEST_DATABASE_URL", Required: true},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Exclude: []string{"styles/old.css"},
		Default: []string{"global", "tokens"},
		Output: gowdk.CSSOutputConfig{
			Dir: "/assets/pages/",
			HrefPrefix: "/app/pages",
		},
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWDK_TEST_BACKEND_ORIGIN", "https://backend.example.com")
	t.Setenv("GOWDK_TEST_DATABASE_URL", "postgres://example")

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Source.Include) != 2 || config.Source.Include[0] != "src/**/*.gwdk" || config.Source.Include[1] != "modules/**/*.gwdk" {
		t.Fatalf("unexpected includes: %#v", config.Source.Include)
	}
	if config.AppName != "Example App" {
		t.Fatalf("unexpected app name: %q", config.AppName)
	}
	if len(config.Source.Exclude) != 1 || config.Source.Exclude[0] != "src/**/draft.page.gwdk" {
		t.Fatalf("unexpected excludes: %#v", config.Source.Exclude)
	}
	if len(config.Modules) != 2 {
		t.Fatalf("unexpected modules: %#v", config.Modules)
	}
	if config.Modules[0].Name != "frontend" || config.Modules[0].Type != "frontend" {
		t.Fatalf("unexpected frontend module: %#v", config.Modules[0])
	}
	if len(config.Modules[0].Source.Include) != 1 || config.Modules[0].Source.Include[0] != "frontend/**/*.gwdk" {
		t.Fatalf("unexpected frontend module includes: %#v", config.Modules[0].Source.Include)
	}
	if len(config.Modules[0].Source.Exclude) != 1 || config.Modules[0].Source.Exclude[0] != "frontend/**/draft.page.gwdk" {
		t.Fatalf("unexpected frontend module excludes: %#v", config.Modules[0].Source.Exclude)
	}
	if config.Modules[1].Name != "backendmicroservice" || config.Modules[1].Type != "backendmicroservice" {
		t.Fatalf("unexpected backend module: %#v", config.Modules[1])
	}
	if len(config.Modules[1].Source.Include) != 1 || config.Modules[1].Source.Include[0] != "backend/**/*.gwdk" {
		t.Fatalf("unexpected backend module includes: %#v", config.Modules[1].Source.Include)
	}
	if config.Build.Output != "dist/site" {
		t.Fatalf("unexpected output: %q", config.Build.Output)
	}
	if config.Build.Mode != gowdk.Production {
		t.Fatalf("unexpected build mode: %q", config.Build.Mode)
	}
	if config.Build.Head.SiteName != "Example" || config.Build.Head.Favicon != "/favicon.ico" || config.Build.Head.Image != "https://example.com/social.png" || config.Build.Head.TwitterCard != "summary_large_image" {
		t.Fatalf("unexpected build head config: %#v", config.Build.Head)
	}
	if !config.Build.CSRF.Enabled || config.Build.CSRF.SecretEnv != "EXAMPLE_CSRF_SECRET" || config.Build.CSRF.CookieName != "__Host-example-csrf" || config.Build.CSRF.FieldName != "_example_csrf" || config.Build.CSRF.HeaderName != "X-Example-CSRF" || !config.Build.CSRF.Insecure {
		t.Fatalf("unexpected build csrf config: %#v", config.Build.CSRF)
	}
	if config.Build.BodyLimits.ActionBytes != 2097152 || config.Build.BodyLimits.APIBytes != 524288 {
		t.Fatalf("unexpected body limits config: %#v", config.Build.BodyLimits)
	}
	if !config.Build.AllowMissingBackend {
		t.Fatal("expected AllowMissingBackend to be parsed")
	}
	if len(config.Build.Stylesheets) != 2 || config.Build.Stylesheets[0].Href != "/assets/app.css" || config.Build.Stylesheets[1].Href != "/assets/theme.css" {
		t.Fatalf("unexpected stylesheets: %#v", config.Build.Stylesheets)
	}
	if len(config.Build.Scripts) != 2 || config.Build.Scripts[0].Src != "/assets/app.js" || config.Build.Scripts[0].Type != "module" || config.Build.Scripts[1].Src != "/assets/legacy.js" || config.Build.Scripts[1].Type != "" {
		t.Fatalf("unexpected scripts: %#v", config.Build.Scripts)
	}
	if len(config.Build.Targets) != 2 {
		t.Fatalf("unexpected build targets: %#v", config.Build.Targets)
	}
	if config.Build.Targets[0].Name != "admin" || config.Build.Targets[0].Output != "dist/admin" || config.Build.Targets[0].App != ".gowdk/admin" || config.Build.Targets[0].Binary != "bin/admin" || config.Build.Targets[0].WASM != "bin/admin.wasm" {
		t.Fatalf("unexpected admin build target: %#v", config.Build.Targets[0])
	}
	if len(config.Build.Targets[0].Modules) != 1 || config.Build.Targets[0].Modules[0] != "admin" {
		t.Fatalf("unexpected admin target modules: %#v", config.Build.Targets[0].Modules)
	}
	if config.Build.Targets[1].Name != "public-admin" || len(config.Build.Targets[1].Modules) != 2 || config.Build.Targets[1].Modules[0] != "public" || config.Build.Targets[1].Modules[1] != "admin" {
		t.Fatalf("unexpected combined build target: %#v", config.Build.Targets[1])
	}
	if config.Render.Default != gowdk.Action {
		t.Fatalf("unexpected render default: %q", config.Render.Default)
	}
	if len(config.Env.Vars) != 2 || config.Env.Vars[0].Name != "GOWDK_TEST_BACKEND_ORIGIN" || !config.Env.Vars[0].Required || config.Env.Vars[1].Name != "GOWDK_TEST_ADDR" || config.Env.Vars[1].Default != "127.0.0.1:8080" {
		t.Fatalf("unexpected env vars: %#v", config.Env.Vars)
	}
	if len(config.Env.Secrets) != 1 || config.Env.Secrets[0].Name != "GOWDK_TEST_DATABASE_URL" || !config.Env.Secrets[0].Required {
		t.Fatalf("unexpected env secrets: %#v", config.Env.Secrets)
	}
	if len(config.CSS.Include) != 1 || config.CSS.Include[0] != "styles/**/*.css" {
		t.Fatalf("unexpected css includes: %#v", config.CSS.Include)
	}
	if len(config.CSS.Exclude) != 1 || config.CSS.Exclude[0] != "styles/old.css" {
		t.Fatalf("unexpected css excludes: %#v", config.CSS.Exclude)
	}
	if len(config.CSS.Default) != 2 || config.CSS.Default[0] != "global" || config.CSS.Default[1] != "tokens" {
		t.Fatalf("unexpected css default: %#v", config.CSS.Default)
	}
	if config.CSS.Output.Dir != "/assets/pages/" || config.CSS.Output.HrefPrefix != "/app/pages" {
		t.Fatalf("unexpected css output: %#v", config.CSS.Output)
	}
}

func TestLoadConfigFileValidatesEnvContract(t *testing.T) {
	missingName := "GOWDK_TEST_MISSING_DATABASE_URL"
	unsetEnvForTest(t, missingName)

	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Env: gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_TEST_MISSING_DATABASE_URL", Required: true},
		},
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), "GOWDK_TEST_MISSING_DATABASE_URL is required but is not set") {
		t.Fatalf("expected missing env validation error, got %v", err)
	}
}

func TestLoadConfigFileRejectsSecretEnvMisuse(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Env: gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{},
			{Name: "GOWDK_TEST_API_TOKEN"},
		},
		Secrets: []gowdk.SecretEnv{
			{},
			{Name: "GOWDK_TEST_API_TOKEN"},
		},
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigFile(path)
	if err == nil {
		t.Fatal("expected env validation error")
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_API_TOKEN looks like a secret") {
		t.Fatalf("expected secret-looking var error, got %v", err)
	}
	if !strings.Contains(err.Error(), "GOWDK_TEST_API_TOKEN is declared more than once") {
		t.Fatalf("expected duplicate env error, got %v", err)
	}
	if !strings.Contains(err.Error(), "environment variable name is required") {
		t.Fatalf("expected empty env var error, got %v", err)
	}
	if !strings.Contains(err.Error(), "secret environment variable name is required") {
		t.Fatalf("expected empty secret env error, got %v", err)
	}
}

func TestLoadConfigFileRejectsSecretEnvInlineValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Env: gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_TEST_DATABASE_URL", Default: "postgres://secret"},
		},
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), "Env.Secrets entries cannot declare Default") {
		t.Fatalf("expected secret default rejection, got %v", err)
	}
	if strings.Contains(err.Error(), "postgres://secret") {
		t.Fatalf("expected secret value to stay out of diagnostics, got %v", err)
	}
}

func TestLoadConfigFileFallsBackForNonLiteralValues(t *testing.T) {
	root := t.TempDir()
	repoRoot := repositoryRoot(t)
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+repoRoot+`
`)
	path := filepath.Join(root, DefaultConfigFile)
	writeTestFile(t, path, `package app

import "github.com/cssbruno/gowdk"

var includes = []string{"src/**/*.gwdk"}

func outputDir() string {
	return "dist/site"
}

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: includes,
	},
	Build: gowdk.BuildConfig{
		Output: outputDir(),
	},
}
`)
	tidyTestModule(t, root)

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Source.Include) != 1 || config.Source.Include[0] != "src/**/*.gwdk" {
		t.Fatalf("expected executable config to load includes, got %#v", config.Source.Include)
	}
	if config.Build.Output != "dist/site" {
		t.Fatalf("expected executable config to load output, got %q", config.Build.Output)
	}
}

func TestLoadConfigFileReadsSSRAddon(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import (
	"github.com/cssbruno/gowdk"
	s "github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		s.Addon(),
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Addons) != 1 || config.Addons[0].Name() != "ssr" {
		t.Fatalf("unexpected addons: %#v", config.Addons)
	}
	if !config.HasFeature(gowdk.FeatureSSR) {
		t.Fatal("expected parsed config to enable SSR")
	}
}

func TestLoadConfigFileReadsBuiltInAddons(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import (
	"github.com/cssbruno/gowdk"
	act "github.com/cssbruno/gowdk/addons/actions"
	apiaddon "github.com/cssbruno/gowdk/addons/api"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	cssaddon "github.com/cssbruno/gowdk/addons/css"
	embedaddon "github.com/cssbruno/gowdk/addons/embed"
	partialaddon "github.com/cssbruno/gowdk/addons/partial"
	rl "github.com/cssbruno/gowdk/addons/ratelimit"
	spaaddon "github.com/cssbruno/gowdk/addons/spa"
	ssraddon "github.com/cssbruno/gowdk/addons/ssr"
	staticaddon "github.com/cssbruno/gowdk/addons/static"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		act.Addon(),
		apiaddon.Addon(),
		contractsaddon.Addon(),
		cssaddon.Addon(),
		embedaddon.Addon(),
		partialaddon.Addon(),
		rl.Addon(),
		spaaddon.Addon(),
		ssraddon.Addon(),
		staticaddon.Addon(),
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Addons) != 10 {
		t.Fatalf("unexpected addons: %#v", config.Addons)
	}
	if config.Addons[9].Name() != "static" {
		t.Fatalf("expected static addon, got %#v", config.Addons[9])
	}
	for _, feature := range []gowdk.Feature{
		gowdk.FeatureActions,
		gowdk.FeatureAPI,
		gowdk.FeatureContracts,
		gowdk.FeatureCSS,
		gowdk.FeatureEmbed,
		gowdk.FeaturePartial,
		gowdk.FeatureRateLimit,
		gowdk.FeatureSPA,
		gowdk.FeatureSSR,
	} {
		if !config.HasFeature(feature) {
			t.Fatalf("expected feature %q from parsed built-in addons", feature)
		}
	}
}

func TestLoadConfigFileReadsImportableExternalAddon(t *testing.T) {
	root := t.TempDir()
	repoRoot := repositoryRoot(t)
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require (
	github.com/cssbruno/gowdk v0.0.0
	github.com/example/gowdk-brand v0.0.0
)

replace github.com/cssbruno/gowdk => `+repoRoot+`
replace github.com/example/gowdk-brand => ./external/gowdk-brand
replace github.com/example/gowdk-theme => ./external/gowdk-theme
`)
	writeTestFile(t, filepath.Join(root, "external", "gowdk-theme", "go.mod"), `module github.com/example/gowdk-theme

go 1.22
`)
	writeTestFile(t, filepath.Join(root, "external", "gowdk-theme", "theme.go"), `package theme

func OutputPrefix() string {
	return "theme-output="
}
`)
	writeTestFile(t, filepath.Join(root, "external", "gowdk-brand", "go.mod"), `module github.com/example/gowdk-brand

go 1.22

require (
	github.com/cssbruno/gowdk v0.0.0
	github.com/example/gowdk-theme v0.0.0
)
`)
	writeTestFile(t, filepath.Join(root, "external", "gowdk-brand", "brand.go"), `package brand

import (
	"github.com/cssbruno/gowdk"
	"github.com/example/gowdk-theme"
)

type addon struct{}

func Addon() gowdk.CSSProcessor {
	return addon{}
}

func (addon) Name() string {
	return "brand"
}

func (addon) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS, gowdk.Feature("brand")}
}

func (addon) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{{
			Path:     "assets/brand.css",
			Contents: []byte(theme.OutputPrefix() + context.OutputDir),
		}},
		Stylesheets: []gowdk.Stylesheet{{Href: "/assets/brand.css"}},
	}, nil
}
`)
	writeTestFile(t, filepath.Join(root, "addons", "marker", "marker.go"), `package marker

import "github.com/cssbruno/gowdk"

func Addon() gowdk.Addon {
	return gowdk.NewAddon("marker", gowdk.Feature("marker"))
}
`)
	path := filepath.Join(root, DefaultConfigFile)
	writeTestFile(t, path, `package app

import (
	"github.com/cssbruno/gowdk"
	brand "github.com/example/gowdk-brand"
	"example.com/site/addons/marker"
)

var Config = gowdk.Config{
	AppName: "External Addon",
	Env: gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{Name: "GOWDK_TEST_EXTERNAL_ADDR", Default: "127.0.0.1:9000"},
		},
	},
	Addons: []gowdk.Addon{
		brand.Addon(),
		marker.Addon(),
	},
}
`)
	tidyTestModule(t, root)

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.AppName != "External Addon" {
		t.Fatalf("unexpected app name: %q", config.AppName)
	}
	if len(config.Addons) != 2 || config.Addons[0].Name() != "brand" || config.Addons[1].Name() != "marker" {
		t.Fatalf("unexpected addons: %#v", config.Addons)
	}
	if len(config.Env.Vars) != 1 || config.Env.Vars[0].Name != "GOWDK_TEST_EXTERNAL_ADDR" || config.Env.Vars[0].Default != "127.0.0.1:9000" {
		t.Fatalf("unexpected executable env config: %#v", config.Env)
	}
	if !config.HasFeature(gowdk.FeatureCSS) || !config.HasFeature(gowdk.Feature("brand")) || !config.HasFeature(gowdk.Feature("marker")) {
		t.Fatalf("expected external addon features, got %#v", config.Addons[0].Features())
	}
	processor, ok := config.Addons[0].(gowdk.CSSProcessor)
	if !ok {
		t.Fatalf("expected external addon proxy to implement CSSProcessor, got %T", config.Addons[0])
	}
	if _, ok := config.Addons[1].(gowdk.CSSProcessor); ok {
		t.Fatalf("expected non-css external addon proxy not to implement CSSProcessor, got %T", config.Addons[1])
	}
	result, err := processor.ProcessCSS(gowdk.CSSContext{OutputDir: "dist/site"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 || result.Assets[0].Path != "assets/brand.css" || string(result.Assets[0].Contents) != "theme-output=dist/site" {
		t.Fatalf("unexpected css result: %#v", result)
	}
	if len(result.Stylesheets) != 1 || result.Stylesheets[0].Href != "/assets/brand.css" {
		t.Fatalf("unexpected stylesheets: %#v", result.Stylesheets)
	}
}

func TestLoadConfigFileReadsTailwindAddon(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import (
	"github.com/cssbruno/gowdk"
	tw "github.com/cssbruno/gowdk/addons/tailwind"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tw.Addon(tw.Options{
			Input: "styles/app.css",
			Command: "gowdk-tailwind-missing-executable",
			OutputPath: "assets/site.css",
			Href: "/assets/site.css",
			Minify: true,
		}),
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Addons) != 1 || config.Addons[0].Name() != "tailwind" {
		t.Fatalf("unexpected addons: %#v", config.Addons)
	}
	processor, ok := config.Addons[0].(gowdk.CSSProcessor)
	if !ok {
		t.Fatalf("expected tailwind addon to implement CSSProcessor, got %T", config.Addons[0])
	}
	_, err = processor.ProcessCSS(gowdk.CSSContext{})
	if err == nil || !strings.Contains(err.Error(), "tailwind executable not found") {
		t.Fatalf("expected parsed tailwind command to be used, got %v", err)
	}
}

func TestParseTailwindAddonRejectsRemovedDownloadOptions(t *testing.T) {
	expression, err := parser.ParseExpr(`tw.Addon(tw.Options{
		Input: "styles/app.css",
		Version: "v4.2.4",
		DownloadDir: ".gowdk/bin",
	})`)
	if err != nil {
		t.Fatal(err)
	}

	if addon, ok := parseTailwindAddon(expression, map[string]string{"tw": tailwind.ImportPath}); ok {
		t.Fatalf("expected removed download options to require normal Go validation, got %#v", addon)
	}
}

func TestLoadConfigFileRejectsInvalidGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

var Config =
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfigFile(path); err == nil {
		t.Fatal("expected invalid Go syntax error")
	}
}

func TestLoadConfigFileDoesNotEchoUnsupportedSecretValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

var Config = secretConfig("SECRET_TOKEN")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigFile(path)
	if err == nil {
		t.Fatal("expected unsupported config error")
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("expected config error to omit secret value, got %v", err)
	}
}

func TestConfigHelperSourceRewritesImportWithAST(t *testing.T) {
	source, err := configHelperSource("example.com/app/config")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "helper.go", source, parser.AllErrors); err != nil {
		t.Fatalf("config helper source must parse: %v\n%s", err, source)
	}
	if !strings.Contains(source, `configpkg "example.com/app/config"`) {
		t.Fatalf("expected generated config import, got:\n%s", source)
	}
	if strings.Contains(source, configHelperImportPlaceholder) {
		t.Fatalf("placeholder import leaked into generated source:\n%s", source)
	}
}

func TestLoadConfigFailsMissingDefault(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()

	_, err = LoadConfig("")
	if err == nil || !strings.Contains(err.Error(), "gowdk.config.go is required") {
		t.Fatalf("expected missing default config error, got %v", err)
	}
}

func TestLoadConfigFailsMissingExplicitPath(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.go"))
	if err == nil {
		t.Fatal("expected missing explicit config error")
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func unsetEnvForTest(t *testing.T, name string) {
	t.Helper()
	value, ok := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !ok {
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Setenv(name, value); err != nil {
			t.Fatal(err)
		}
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(workingDir, "..", ".."))
}

func tidyTestModule(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "mod", "tidy")
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, stderr.String())
	}
}
