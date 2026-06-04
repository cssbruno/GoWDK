// Package tailwind integrates Tailwind CSS v4 through the standalone CLI.
package tailwind

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
)

const (
	defaultCommand    = "tailwindcss"
	defaultOutputPath = "assets/app.css"
	defaultHref       = "/assets/app.css"
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
	// "tailwindcss", resolved from PATH. Projects can pass an absolute path to a
	// pinned executable.
	Command string
	// Minify passes --minify to the Tailwind CLI.
	Minify bool
}

// Addon returns a compile-time CSS processor that wraps the Tailwind v4
// standalone CLI. It does not download Tailwind, use npm, or run through a
// shell.
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
	command := exec.Command(options.Command, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return gowdk.CSSResult{}, commandError(options.Command, err, stdout.String(), stderr.String())
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
	if strings.TrimSpace(options.Command) == "" {
		options.Command = defaultCommand
	}
	if strings.TrimSpace(options.OutputPath) == "" {
		options.OutputPath = defaultOutputPath
	}
	if strings.TrimSpace(options.Href) == "" {
		options.Href = defaultHref
	}
	return options
}

func commandError(command string, err error, stdout string, stderr string) error {
	var execError *exec.Error
	if errors.As(err, &execError) && errors.Is(execError.Err, exec.ErrNotFound) {
		return fmt.Errorf("tailwind executable not found %q: set tailwind.Options.Command to a standalone Tailwind CLI binary", command)
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
