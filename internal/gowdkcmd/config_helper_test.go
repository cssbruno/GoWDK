package gowdkcmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeProjectCommandPathArgsAbsolutizesExistingInputs(t *testing.T) {
	root := t.TempDir()
	configRel := filepath.Join("examples", "css", "gowdk.config.go")
	pageRel := filepath.Join("examples", "css", "styled.page.gwdk")
	configPath := writeHelperArgTestFile(t, root, configRel)
	pagePath := writeHelperArgTestFile(t, root, pageRel)
	args := []string{"build", "--config", configRel, "--out", "dist", pageRel}

	got := normalizeProjectCommandPathArgs(args, root, 1)
	want := []string{"build", "--config", configPath, "--out", "dist", pagePath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeProjectCommandPathArgs() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(args, []string{"build", "--config", configRel, "--out", "dist", pageRel}) {
		t.Fatalf("normalizeProjectCommandPathArgs mutated input args: %#v", args)
	}
}

func TestNormalizeProjectCommandPathArgsHandlesEqualsAndBooleanFlags(t *testing.T) {
	root := t.TempDir()
	configRel := filepath.Join("examples", "css", "gowdk.config.go")
	pageRel := filepath.Join("examples", "css", "styled.page.gwdk")
	configPath := writeHelperArgTestFile(t, root, configRel)
	pagePath := writeHelperArgTestFile(t, root, pageRel)
	args := []string{"build", "--config=" + configRel, "--timings", pageRel}

	got := normalizeProjectCommandPathArgs(args, root, 1)
	want := []string{"build", "--config=" + configPath, "--timings", pagePath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeProjectCommandPathArgs() = %#v, want %#v", got, want)
	}
}

func TestNormalizeDevHelperArgsNormalizesForwardedBuildInputs(t *testing.T) {
	root := t.TempDir()
	configRel := filepath.Join("examples", "css", "gowdk.config.go")
	pageRel := filepath.Join("examples", "css", "styled.page.gwdk")
	configPath := writeHelperArgTestFile(t, root, configRel)
	pagePath := writeHelperArgTestFile(t, root, pageRel)
	args := []string{
		"dev",
		"--addr", "127.0.0.1:8081",
		"--interval=250ms",
		"--config", configRel,
		"--out", "dist",
		pageRel,
	}

	got := normalizeDevHelperArgs(args, root)
	want := []string{
		"dev",
		"--addr", "127.0.0.1:8081",
		"--interval=250ms",
		"--config", configPath,
		"--out", "dist",
		pagePath,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeDevHelperArgs() = %#v, want %#v", got, want)
	}
}

func writeHelperArgTestFile(t *testing.T, root string, rel string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
