package tailwind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestAddonRegistersCSSFeature(t *testing.T) {
	addon := Addon(Options{})
	if addon.Name() != "tailwind" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureCSS) {
		t.Fatal("expected css feature")
	}
}

func TestProcessCSSRequiresInput(t *testing.T) {
	_, err := Addon(Options{}).ProcessCSS(gowdk.CSSContext{})
	if err == nil || !strings.Contains(err.Error(), "tailwind input css path is required") {
		t.Fatalf("expected missing input error, got %v", err)
	}
}

func TestProcessCSSRunsStandaloneCommand(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "app.css")
	if err := os.WriteFile(input, []byte(`@import "tailwindcss";`), 0o644); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(root, "args.txt")
	inputCopy := filepath.Join(root, "generated-input.css")
	t.Setenv("TAILWIND_ARGS_FILE", argsFile)
	t.Setenv("TAILWIND_INPUT_COPY", inputCopy)

	result, err := Addon(Options{
		Input:      input,
		Command:    fakeTailwindCommand(t),
		OutputPath: "assets/site.css",
		Href:       "/assets/site.css",
		Minify:     true,
	}).ProcessCSS(gowdk.CSSContext{
		Sources: []gowdk.CSSSource{{Path: "site.page.gwdk", Kind: "page", Name: "site"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("expected one css asset, got %#v", result.Assets)
	}
	if result.Assets[0].Path != "assets/site.css" {
		t.Fatalf("unexpected css asset path: %q", result.Assets[0].Path)
	}
	if string(result.Assets[0].Contents) != "/* fake tailwind */\nbody { color: black; }\n" {
		t.Fatalf("unexpected css asset contents: %q", result.Assets[0].Contents)
	}
	if len(result.Stylesheets) != 1 || result.Stylesheets[0].Href != "/assets/site.css" {
		t.Fatalf("unexpected stylesheets: %#v", result.Stylesheets)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	output := string(args)
	for _, expected := range []string{"-i\n", "-o\n", "--minify"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected command args to contain %q, got:\n%s", expected, output)
		}
	}
	generatedInput, err := os.ReadFile(inputCopy)
	if err != nil {
		t.Fatal(err)
	}
	generated := string(generatedInput)
	for _, expected := range []string{`@import "`, `app.css`, `@source "`, `site.page.gwdk`} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("expected generated tailwind input to contain %q, got:\n%s", expected, generated)
		}
	}
}

func TestProcessCSSReportsMissingExecutable(t *testing.T) {
	input := filepath.Join(t.TempDir(), "app.css")
	if err := os.WriteFile(input, []byte(`@import "tailwindcss";`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Addon(Options{
		Input:   input,
		Command: "gowdk-tailwind-missing-executable",
	}).ProcessCSS(gowdk.CSSContext{})
	if err == nil || !strings.Contains(err.Error(), "tailwind executable not found") {
		t.Fatalf("expected missing executable error, got %v", err)
	}
}

func TestProcessCSSRequiresInstalledCommandWhenDefaultMissing(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "app.css")
	if err := os.WriteFile(input, []byte(`@import "tailwindcss";`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(root, "empty-path"))

	_, err := Addon(Options{Input: input}).ProcessCSS(gowdk.CSSContext{})
	if err == nil || !strings.Contains(err.Error(), "tailwindcss is not installed") {
		t.Fatalf("expected install-required error, got %v", err)
	}
}

func TestSPABuildWritesTailwindAssetAndStylesheet(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "app.css")
	if err := os.WriteFile(input, []byte(`@import "tailwindcss";`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAILWIND_ARGS_FILE", filepath.Join(root, "args.txt"))

	outputDir := filepath.Join(root, "dist")
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "site",
		Route:  "/",
		Source: "site.page.gwdk",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main class="font-bold">Site</main>`,
		},
	}}}
	result, err := buildgen.Build(gowdk.Config{
		Addons: []gowdk.Addon{Addon(Options{
			Input:   input,
			Command: fakeTailwindCommand(t),
		})},
	}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CSSArtifacts) != 1 || result.CSSArtifacts[0].LogicalPath != "assets/app.css" {
		t.Fatalf("unexpected css artifacts: %#v", result.CSSArtifacts)
	}
	if !strings.Contains(filepath.ToSlash(result.CSSArtifacts[0].Path), "/assets/app.") || !strings.HasSuffix(result.CSSArtifacts[0].Path, ".css") {
		t.Fatalf("expected hashed tailwind css path, got %#v", result.CSSArtifacts[0])
	}

	css, err := os.ReadFile(result.CSSArtifacts[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "body{color:black;}") {
		t.Fatalf("expected generated tailwind css, got %q", css)
	}
	html, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	emittedRel, err := filepath.Rel(outputDir, result.CSSArtifacts[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	emittedHref := "/" + filepath.ToSlash(emittedRel)
	if !strings.Contains(string(html), `<link rel="stylesheet" href="`+emittedHref+`">`) {
		t.Fatalf("expected tailwind stylesheet link:\n%s", html)
	}
	payload, err := os.ReadFile(filepath.Join(outputDir, "gowdk-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var assets runtimeasset.Manifest
	if err := json.Unmarshal(payload, &assets); err != nil {
		t.Fatal(err)
	}
	if got := assets.Resolve("assets/app.css"); got != strings.TrimPrefix(emittedHref, "/") {
		t.Fatalf("unexpected asset manifest entry: %q", got)
	}
}

func fakeTailwindCommand(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tailwindcss")
	if err := os.WriteFile(path, []byte(fakeTailwindScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const fakeTailwindScript = `#!/bin/sh
set -eu
printf '%s\n' "$@" > "$TAILWIND_ARGS_FILE"
out=""
in=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-i|--input)
			shift
			in="$1"
			;;
		-o|--output)
			shift
			out="$1"
			;;
	esac
	shift
done
if [ -z "$out" ]; then
	echo "missing output path" >&2
	exit 2
fi
if [ "${TAILWIND_INPUT_COPY:-}" != "" ]; then
	cp "$in" "$TAILWIND_INPUT_COPY"
fi
printf '/* fake tailwind */\nbody { color: black; }\n' > "$out"
`
