// Package appgen emits a generated Go app that embeds build output.
package appgen

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

const (
	appPackageDirName = "gowdkapp"
	serverDirName     = "cmd/server"
	appOutputDirName  = appPackageDirName + "/app"
	appFileName       = appPackageDirName + "/app.go"
	mainFileName      = serverDirName + "/main.go"
	modFileName       = "go.mod"
)

// Generate writes a self-contained Go app that embeds outputDir.
func Generate(outputDir, appDir string) (Result, error) {
	return GenerateWithOptions(outputDir, appDir, Options{})
}

// GenerateWithOptions writes a self-contained Go app that embeds outputDir.
func GenerateWithOptions(outputDir, appDir string, options Options) (Result, error) {
	if strings.TrimSpace(outputDir) == "" {
		return Result{}, fmt.Errorf("build output directory is required")
	}
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated app directory is required")
	}

	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return Result{}, err
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	if err := validateDirectories(absOutput, absApp); err != nil {
		return Result{}, err
	}
	options, err = resolveOptions(absOutput, options)
	if err != nil {
		return Result{}, err
	}
	if err := validateActionRoutes(options.Actions); err != nil {
		return Result{}, err
	}
	if err := validateSSRRoutes(options.SSR); err != nil {
		return Result{}, err
	}

	targetOutput := filepath.Join(absApp, appOutputDirName)
	if isSameOrWithin(targetOutput, absOutput) {
		return Result{}, fmt.Errorf("build output directory %q must not be inside generated app output directory %q", absOutput, targetOutput)
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(targetOutput, 0o755); err != nil {
		return Result{}, err
	}

	files, err := copyOutputFiles(absOutput, targetOutput)
	if err != nil {
		return Result{}, err
	}
	if err := removeStaleOutputFiles(targetOutput, files); err != nil {
		return Result{}, err
	}
	modulePayload, err := moduleSource(options)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, modFileName), []byte(modulePayload)); err != nil {
		return Result{}, err
	}
	appSource, err := formatGeneratedGo(appFileName, []byte(appPackageSource(options)))
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, appFileName), appSource); err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, mainFileName), []byte(serverMainSource)); err != nil {
		return Result{}, err
	}

	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, mainFileName),
		PackagePath: filepath.Join(absApp, appFileName),
		ModulePath:  filepath.Join(absApp, modFileName),
		OutputDir:   targetOutput,
		Files:       files,
	}, nil
}

// GenerateBackendWithOptions writes a generated Go app that serves only
// request-time backend routes for feature-bound actions and APIs.
func GenerateBackendWithOptions(appDir string, options Options) (Result, error) {
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated backend app directory is required")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	options, err = resolveBackendOptions(options)
	if err != nil {
		return Result{}, err
	}
	if err := validateActionRoutes(options.Actions); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	modulePayload, err := moduleSource(options)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, modFileName), []byte(modulePayload)); err != nil {
		return Result{}, err
	}
	appSource, err := formatGeneratedGo(appFileName, []byte(backendAppPackageSource(options)))
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, appFileName), appSource); err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, mainFileName), []byte(serverMainSource)); err != nil {
		return Result{}, err
	}
	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, mainFileName),
		PackagePath: filepath.Join(absApp, appFileName),
		ModulePath:  filepath.Join(absApp, modFileName),
	}, nil
}

func formatGeneratedGo(name string, source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated %s: %w", name, err)
	}
	return formatted, nil
}
