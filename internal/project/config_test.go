package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileReadsLiteralSourceAndBuildFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/gowdk/gowdk"

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
}

func TestLoadConfigFileIgnoresNonLiteralValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	if err := os.WriteFile(path, []byte(`package app

import "github.com/gowdk/gowdk"

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
