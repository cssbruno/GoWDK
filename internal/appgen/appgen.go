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
	lifecycleFileName = appPackageDirName + "/lifecycle_services.go"
	lifecycleJSName   = appPackageDirName + "/lifecycle_services_js.go"
	auditTestFileName = appPackageDirName + "/gowdk_audit_test.go"
	mainFileName      = serverDirName + "/main.go"
	modFileName       = "go.mod"
)

// Generate writes a self-contained Go app that embeds outputDir.
func Generate(outputDir, appDir string) (Result, error) {
	return GenerateWithOptions(outputDir, appDir, Options{})
}

// GenerateWithOptions writes a self-contained Go app that embeds outputDir.
func GenerateWithOptions(outputDir, appDir string, options Options) (result Result, err error) {
	defer recoverGeneratedIdentifierError(&err)

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
	if err := validateActionEndpoints(options.Actions); err != nil {
		return Result{}, err
	}
	if err := validateAPIEndpoints(options.APIs); err != nil {
		return Result{}, err
	}
	if err := validateFragmentEndpoints(options.Fragments); err != nil {
		return Result{}, err
	}
	if err := validateContractRoutes(options.IR); err != nil {
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
	packageSource, err := appPackageSource(options)
	if err != nil {
		return Result{}, err
	}
	appSource, err := formatGeneratedGo(appFileName, []byte(packageSource))
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, appFileName), appSource); err != nil {
		return Result{}, err
	}
	if err := writeLifecycleServiceFiles(absApp, options); err != nil {
		return Result{}, err
	}
	auditTestSource, err := GeneratedAuditTestSource(options)
	if err != nil {
		return Result{}, err
	}
	auditTestPath := filepath.Join(absApp, auditTestFileName)
	if len(auditTestSource) > 0 {
		if err := writeFileIfChanged(auditTestPath, auditTestSource); err != nil {
			return Result{}, err
		}
	} else if err := os.Remove(auditTestPath); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	scriptFiles, err := writeInlineGoBlockFiles(absApp, options)
	if err != nil {
		return Result{}, err
	}
	addonGoBlockFiles, err := writeAddonGoBlockFiles(absApp, options)
	if err != nil {
		return Result{}, err
	}
	files = append(files, scriptFiles...)
	files = append(files, addonGoBlockFiles...)
	mainSource, err := serverMainSource()
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, mainFileName), []byte(mainSource)); err != nil {
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
func GenerateBackendWithOptions(appDir string, options Options) (result Result, err error) {
	defer recoverGeneratedIdentifierError(&err)

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
	if err := validateActionEndpoints(options.Actions); err != nil {
		return Result{}, err
	}
	if err := validateAPIEndpoints(options.APIs); err != nil {
		return Result{}, err
	}
	if err := validateFragmentEndpoints(options.Fragments); err != nil {
		return Result{}, err
	}
	if err := validateContractRoutes(options.IR); err != nil {
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
	packageSource, err := backendAppPackageSource(options)
	if err != nil {
		return Result{}, err
	}
	appSource, err := formatGeneratedGo(appFileName, []byte(packageSource))
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, appFileName), appSource); err != nil {
		return Result{}, err
	}
	if err := writeLifecycleServiceFiles(absApp, options); err != nil {
		return Result{}, err
	}
	if _, err := writeInlineGoBlockFiles(absApp, options); err != nil {
		return Result{}, err
	}
	if _, err := writeAddonGoBlockFiles(absApp, options); err != nil {
		return Result{}, err
	}
	mainSource, err := serverMainSource()
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, mainFileName), []byte(mainSource)); err != nil {
		return Result{}, err
	}
	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, mainFileName),
		PackagePath: filepath.Join(absApp, appFileName),
		ModulePath:  filepath.Join(absApp, modFileName),
	}, nil
}

func writeLifecycleServiceFiles(absApp string, options Options) error {
	sources, err := lifecycleServiceFileSources(options)
	if err != nil {
		return err
	}
	for _, name := range []string{lifecycleFileName, lifecycleJSName} {
		path := filepath.Join(absApp, name)
		source, ok := sources[name]
		if !ok {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		formatted, err := formatGeneratedGo(name, source)
		if err != nil {
			return err
		}
		if err := writeFileIfChanged(path, formatted); err != nil {
			return err
		}
	}
	return nil
}

func formatGeneratedGo(name string, source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated %s: %w", name, err)
	}
	return formatted, nil
}
