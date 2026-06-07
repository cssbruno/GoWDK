// Package tailwind integrates Tailwind CSS v4 through the standalone CLI.
package tailwind

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cssbruno/gowdk"
)

const (
	// ImportPath is the canonical Go import path for the Tailwind addon.
	ImportPath             = "github.com/cssbruno/gowdk/addons/tailwind"
	defaultCommand         = "tailwindcss"
	defaultDownloadDir     = ".gowdk/bin"
	defaultOutputPath      = "assets/app.css"
	defaultHref            = "/assets/app.css"
	defaultDownloadBaseURL = "https://github.com/tailwindlabs/tailwindcss/releases"
)

var (
	downloadBaseURL = defaultDownloadBaseURL
	downloadClient  = http.DefaultClient
)

// Options configures the Tailwind CSS v4 processor.
type Options struct {
	// Input is the Tailwind input CSS file, for example "assets/app.css".
	Input string
	// OutputPath is the generated CSS asset path inside the GOWDK output
	// directory. It defaults to "assets/app.css".
	OutputPath string
	// Href is the stylesheet href emitted into generated HTML. It defaults to
	// "/assets/app.css".
	Href string
	// Command is the Tailwind standalone executable. It defaults to
	// "tailwindcss" on PATH, then a downloaded standalone executable cached
	// under DownloadDir. Projects can pass an absolute path to a pinned
	// executable.
	Command string
	// Version selects a Tailwind release tag such as "v4.2.4" for downloads. It
	// defaults to "latest".
	Version string
	// DownloadDir is where the default standalone executable is cached when
	// Command is omitted and tailwindcss is not on PATH. It defaults to
	// ".gowdk/bin".
	DownloadDir string
	// Minify passes --minify to the Tailwind CLI.
	Minify bool
}

// Addon returns a compile-time CSS processor that wraps the Tailwind v4
// standalone CLI. When no command is configured it uses tailwindcss on PATH or
// downloads the official standalone executable into a project-local cache. It
// does not use npm or run through a shell.
func Addon(options Options) gowdk.CSSProcessor {
	return processor{options: options}
}

type processor struct {
	options Options
}

func (processor) Name() string {
	return "tailwind"
}

func (processor) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (p processor) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	options := normalizeOptions(p.options)
	if strings.TrimSpace(options.Input) == "" {
		return gowdk.CSSResult{}, fmt.Errorf("tailwind input css path is required")
	}
	commandPath, err := resolveCommand(options)
	if err != nil {
		return gowdk.CSSResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "gowdk-tailwind-*")
	if err != nil {
		return gowdk.CSSResult{}, err
	}
	defer os.RemoveAll(tempDir)

	tempOutput := filepath.Join(tempDir, "app.css")
	input := options.Input
	if len(context.Sources) > 0 {
		generatedInput, err := writeInputWithSources(tempDir, options.Input, context.Sources)
		if err != nil {
			return gowdk.CSSResult{}, err
		}
		input = generatedInput
	}
	args := []string{"-i", input, "-o", tempOutput}
	if options.Minify {
		args = append(args, "--minify")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.Command(commandPath, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return gowdk.CSSResult{}, commandError(commandPath, err, stdout.String(), stderr.String())
	}

	contents, err := os.ReadFile(tempOutput)
	if err != nil {
		return gowdk.CSSResult{}, fmt.Errorf("tailwind output was not written: %w", err)
	}

	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{{
			Path:     options.OutputPath,
			Contents: contents,
		}},
		Stylesheets: []gowdk.Stylesheet{{Href: options.Href}},
	}, nil
}

func writeInputWithSources(tempDir string, input string, sources []gowdk.CSSSource) (string, error) {
	inputPath, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	relInput, err := filepath.Rel(tempDir, inputPath)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(`@import "`)
	builder.WriteString(cssPath(relInput))
	builder.WriteString("\";\n")

	seen := map[string]bool{}
	for _, source := range sources {
		sourcePath := strings.TrimSpace(source.Path)
		if sourcePath == "" || seen[sourcePath] {
			continue
		}
		seen[sourcePath] = true
		absoluteSource, err := filepath.Abs(sourcePath)
		if err != nil {
			return "", err
		}
		relativeSource, err := filepath.Rel(tempDir, absoluteSource)
		if err != nil {
			return "", err
		}
		builder.WriteString(`@source "`)
		builder.WriteString(cssPath(relativeSource))
		builder.WriteString("\";\n")
	}

	generatedInput := filepath.Join(tempDir, "input.css")
	if err := os.WriteFile(generatedInput, []byte(builder.String()), 0o644); err != nil {
		return "", err
	}
	return generatedInput, nil
}

func cssPath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, `"`, `\"`)
	return path
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.OutputPath) == "" {
		options.OutputPath = defaultOutputPath
	}
	if strings.TrimSpace(options.Href) == "" {
		options.Href = defaultHref
	}
	if strings.TrimSpace(options.DownloadDir) == "" {
		options.DownloadDir = defaultDownloadDir
	}
	return options
}

func resolveCommand(options Options) (string, error) {
	if command := strings.TrimSpace(options.Command); command != "" {
		return command, nil
	}
	if command, err := exec.LookPath(defaultCommand); err == nil {
		return command, nil
	}
	return ensureDownloadedCommand(options)
}

func ensureDownloadedCommand(options Options) (string, error) {
	asset, err := tailwindAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(options.Version)
	if version == "" {
		version = "latest"
	}
	commandPath := filepath.Join(options.DownloadDir, asset)
	if info, err := os.Stat(commandPath); err == nil && !info.IsDir() {
		return commandPath, nil
	}
	if err := os.MkdirAll(options.DownloadDir, 0o755); err != nil {
		return "", err
	}

	url := tailwindDownloadURL(version, asset)
	response, err := downloadClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("download tailwind standalone cli from %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download tailwind standalone cli from %s: unexpected HTTP %d", url, response.StatusCode)
	}

	temp, err := os.CreateTemp(options.DownloadDir, ".tailwindcss-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	_, copyErr := io.Copy(temp, response.Body)
	closeErr := temp.Close()
	if copyErr != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("write downloaded tailwind standalone cli: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tempPath)
		return "", closeErr
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		os.Remove(tempPath)
		return "", err
	}
	if err := os.Rename(tempPath, commandPath); err != nil {
		os.Remove(tempPath)
		return "", err
	}
	return commandPath, nil
}

func tailwindDownloadURL(version string, asset string) string {
	release := "latest/download"
	if version != "latest" {
		release = "download/" + strings.TrimPrefix(version, "/")
	}
	return strings.TrimRight(downloadBaseURL, "/") + "/" + release + "/" + asset
}

func tailwindAssetName(goos string, goarch string) (string, error) {
	arch := ""
	switch goarch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("tailwind standalone cli download is unsupported on %s/%s", goos, goarch)
	}
	switch goos {
	case "linux":
		return "tailwindcss-linux-" + arch, nil
	case "darwin":
		return "tailwindcss-macos-" + arch, nil
	case "windows":
		return "tailwindcss-windows-" + arch + ".exe", nil
	default:
		return "", fmt.Errorf("tailwind standalone cli download is unsupported on %s/%s", goos, goarch)
	}
}

func commandError(command string, err error, stdout string, stderr string) error {
	var execError *exec.Error
	if errors.As(err, &execError) && errors.Is(execError.Err, exec.ErrNotFound) {
		return fmt.Errorf("tailwind executable not found %q", command)
	}

	output := strings.TrimSpace(stderr)
	if output == "" {
		output = strings.TrimSpace(stdout)
	}
	if output == "" {
		return fmt.Errorf("tailwind command failed: %w", err)
	}
	return fmt.Errorf("tailwind command failed: %w: %s", err, output)
}
