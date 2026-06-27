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
	// ImportPath is the canonical Go import path for the Tailwind addon.
	ImportPath        = "github.com/cssbruno/gowdk/addons/tailwind"
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
	// "tailwindcss" on PATH. Projects can pass an absolute path to a pinned
	// installed executable.
	Command string
	// Minify passes --minify to the Tailwind CLI.
	Minify bool
}

// Addon returns a compile-time CSS processor that wraps the Tailwind v4
// standalone CLI. When no command is configured it uses tailwindcss on PATH. It
// does not download Tailwind, use npm, or run through a shell.
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
	workingDir := cssWorkingDir(context)
	input := resolveCSSPath(workingDir, options.Input)
	if len(context.Sources) > 0 {
		generatedInput, err := writeInputWithSources(tempDir, workingDir, input, context.Sources)
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
	command.Dir = workingDir
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

func writeInputWithSources(tempDir string, workingDir string, input string, sources []gowdk.CSSSource) (string, error) {
	inputPath := resolveCSSPath(workingDir, input)
	relInput, err := filepath.Rel(tempDir, inputPath)
	if err != nil {
		return "", err
	}

	lines := []string{`@import "` + cssPath(relInput) + `";`}

	seen := map[string]bool{}
	for _, source := range sources {
		sourcePath := strings.TrimSpace(source.Path)
		if sourcePath == "" || seen[sourcePath] {
			continue
		}
		seen[sourcePath] = true
		absoluteSource := resolveCSSPath(workingDir, sourcePath)
		relativeSource, err := filepath.Rel(tempDir, absoluteSource)
		if err != nil {
			return "", err
		}
		lines = append(lines, `@source "`+cssPath(relativeSource)+`";`)
	}

	generatedInput := filepath.Join(tempDir, "input.css")
	if err := os.WriteFile(generatedInput, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return generatedInput, nil
}

func cssWorkingDir(context gowdk.CSSContext) string {
	for _, candidate := range []string{context.WorkingDir, context.ConfigDir, context.ProjectRoot, context.SourceRoot} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return "."
}

func resolveCSSPath(workingDir string, path string) string {
	path = strings.TrimSpace(path)
	if filepath.IsAbs(path) {
		return path
	}
	absolute, err := filepath.Abs(filepath.Join(workingDir, path))
	if err != nil {
		return filepath.Join(workingDir, path)
	}
	return absolute
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
	return options
}

func resolveCommand(options Options) (string, error) {
	if command := strings.TrimSpace(options.Command); command != "" {
		return command, nil
	}
	if command, err := exec.LookPath(defaultCommand); err == nil {
		return command, nil
	}
	return "", missingTailwindError()
}

func missingTailwindError() error {
	return fmt.Errorf("tailwindcss is not installed; install the Tailwind CSS standalone CLI and make it available on PATH, or set tailwind.Options.Command to an installed executable")
}

func commandError(command string, err error, stdout string, stderr string) error {
	var execError *exec.Error
	if errors.As(err, &execError) && errors.Is(execError.Err, exec.ErrNotFound) {
		return fmt.Errorf("tailwind executable not found %q; install the Tailwind CSS standalone CLI and set tailwind.Options.Command to the installed executable, or make tailwindcss available on PATH", command)
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
