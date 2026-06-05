// Package appgen emits a generated Go app that embeds static build output.
package appgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appPackageDirName = "gowdkapp"
	serverDirName     = "cmd/server"
	staticDirName     = appPackageDirName + "/static"
	appFileName       = appPackageDirName + "/app.go"
	mainFileName      = serverDirName + "/main.go"
	modFileName       = "go.mod"
)

// Generate writes a self-contained Go app that embeds staticDir.
func Generate(staticDir, appDir string) (Result, error) {
	return GenerateWithOptions(staticDir, appDir, Options{})
}

// GenerateWithOptions writes a self-contained Go app that embeds staticDir.
func GenerateWithOptions(staticDir, appDir string, options Options) (Result, error) {
	if strings.TrimSpace(staticDir) == "" {
		return Result{}, fmt.Errorf("static output directory is required")
	}
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated app directory is required")
	}

	absStatic, err := filepath.Abs(staticDir)
	if err != nil {
		return Result{}, err
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	if err := validateDirectories(absStatic, absApp); err != nil {
		return Result{}, err
	}
	if err := validateActionRoutes(options.Actions); err != nil {
		return Result{}, err
	}
	if err := validateSSRRoutes(options.SSR); err != nil {
		return Result{}, err
	}

	targetStatic := filepath.Join(absApp, staticDirName)
	if isSameOrWithin(targetStatic, absStatic) {
		return Result{}, fmt.Errorf("static output directory %q must not be inside generated app static directory %q", absStatic, targetStatic)
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(targetStatic, 0o755); err != nil {
		return Result{}, err
	}

	files, err := copyStaticFiles(absStatic, targetStatic)
	if err != nil {
		return Result{}, err
	}
	if err := removeStaleStaticFiles(targetStatic, files); err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, modFileName), []byte(moduleSource)); err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, appFileName), []byte(appPackageSource(options.Actions, options.SSR))); err != nil {
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
		StaticDir:   targetStatic,
		Files:       files,
	}, nil
}
