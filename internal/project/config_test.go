package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestLoadConfigFileReadsLiteralSourceAndBuildFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
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
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/app.css"},
			{Href: "/assets/theme.css"},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Exclude: []string{"styles/legacy.css"},
		Default: []string{"global", "tokens"},
		Output: gowdk.CSSOutputConfig{
			Dir: "/assets/pages/",
			HrefPrefix: "/static/pages",
		},
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Source.Include) != 2 || config.Source.Include[0] != "src/**/*.gwdk" || config.Source.Include[1] != "modules/**/*.gwdk" {
		t.Fatalf("unexpected includes: %#v", config.Source.Include)
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
	if len(config.Build.Stylesheets) != 2 || config.Build.Stylesheets[0].Href != "/assets/app.css" || config.Build.Stylesheets[1].Href != "/assets/theme.css" {
		t.Fatalf("unexpected stylesheets: %#v", config.Build.Stylesheets)
	}
	if len(config.CSS.Include) != 1 || config.CSS.Include[0] != "styles/**/*.css" {
		t.Fatalf("unexpected css includes: %#v", config.CSS.Include)
	}
	if len(config.CSS.Exclude) != 1 || config.CSS.Exclude[0] != "styles/legacy.css" {
		t.Fatalf("unexpected css excludes: %#v", config.CSS.Exclude)
	}
	if len(config.CSS.Default) != 2 || config.CSS.Default[0] != "global" || config.CSS.Default[1] != "tokens" {
		t.Fatalf("unexpected css default: %#v", config.CSS.Default)
	}
	if config.CSS.Output.Dir != "/assets/pages/" || config.CSS.Output.HrefPrefix != "/static/pages" {
		t.Fatalf("unexpected css output: %#v", config.CSS.Output)
	}
}

func TestLoadConfigFileIgnoresNonLiteralValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/cssbruno/gowdk"

var includes = []string{"src/**/*.gwdk"}

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: includes,
	},
	Build: gowdk.BuildConfig{
		Output: outputDir(),
	},
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Source.Include) != 0 {
		t.Fatalf("expected non-literal includes to be ignored, got %#v", config.Source.Include)
	}
	if config.Build.Output != "" {
		t.Fatalf("expected non-literal output to be ignored, got %q", config.Build.Output)
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

func TestLoadOptionalConfigIgnoresMissingDefault(t *testing.T) {
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

	_, loaded, err := LoadOptionalConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if loaded {
		t.Fatal("expected missing default config to be ignored")
	}
}

func TestLoadOptionalConfigFailsMissingExplicitPath(t *testing.T) {
	_, loaded, err := LoadOptionalConfig(filepath.Join(t.TempDir(), "missing.go"))
	if err == nil {
		t.Fatal("expected missing explicit config error")
	}
	if !loaded {
		t.Fatal("expected explicit config path to be treated as loaded")
	}
}
