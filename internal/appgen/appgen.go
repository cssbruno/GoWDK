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

var errInvalidApplicationPlan = fmt.Errorf("application plan was not constructed by appgen planning")

// Generate writes a self-contained Go app that embeds outputDir.
func Generate(outputDir, appDir string) (Result, error) {
	return GenerateWithOptions(outputDir, appDir, Options{})
}

// PlanApplication resolves and validates generated app options before emission.
func PlanApplication(outputDir string, options Options) (ApplicationPlan, error) {
	if strings.TrimSpace(outputDir) == "" {
		return ApplicationPlan{}, fmt.Errorf("build output directory is required")
	}
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return ApplicationPlan{}, err
	}
	planned, err := resolveOptions(absOutput, options)
	if err != nil {
		return ApplicationPlan{}, err
	}
	if err := validateAppPlanOptions(planned, true); err != nil {
		return ApplicationPlan{}, err
	}
	return ApplicationPlan{options: planned, outputDir: absOutput, valid: true}, nil
}

// PlanBackendApplication resolves and validates backend-only generated app
// options before emission.
func PlanBackendApplication(options Options) (ApplicationPlan, error) {
	planned, err := resolveBackendOptions(options)
	if err != nil {
		return ApplicationPlan{}, err
	}
	if err := validateAppPlanOptions(planned, false); err != nil {
		return ApplicationPlan{}, err
	}
	return ApplicationPlan{options: planned, backendOnly: true, valid: true}, nil
}

func validateAppPlanOptions(options Options, includeSSR bool) error {
	if err := validateActionEndpoints(options.Actions); err != nil {
		return err
	}
	if err := validateAPIEndpoints(options.APIs); err != nil {
		return err
	}
	if err := validateFragmentEndpoints(options.Fragments); err != nil {
		return err
	}
	if err := validateContractRoutes(options.IR); err != nil {
		return err
	}
	if includeSSR {
		if err := validateSSRRoutes(options.SSR); err != nil {
			return err
		}
	}
	return validateCORSConfig(options)
}

// GenerateWithOptions writes a self-contained Go app that embeds outputDir.
func GenerateWithOptions(outputDir, appDir string, options Options) (result Result, err error) {
	plan, err := PlanApplication(outputDir, options)
	if err != nil {
		return Result{}, err
	}
	return GenerateWithPlan(outputDir, appDir, plan)
}

// GenerateWithPlan writes a self-contained Go app from an application plan.
func GenerateWithPlan(outputDir, appDir string, plan ApplicationPlan) (result Result, err error) {
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
	if plan.backendOnly {
		return Result{}, fmt.Errorf("backend application plan cannot be used for embedded app generation")
	}
	if plan.outputDir != "" && filepath.Clean(plan.outputDir) != filepath.Clean(absOutput) {
		return Result{}, fmt.Errorf("application plan output directory %q does not match build output directory %q", plan.outputDir, absOutput)
	}
	options, err := plan.optionsForEmit()
	if err != nil {
		return Result{}, err
	}

	targetOutput := filepath.Join(absApp, appOutputDirName)
	if isSameOrWithin(targetOutput, absOutput) {
		return Result{}, fmt.Errorf("build output directory %q must not be inside generated app output directory %q", absOutput, targetOutput)
	}
	moduleContext := resolveGeneratedModuleContext(absApp)
	files, outputFiles, err := collectOutputFiles(absOutput, targetOutput)
	if err != nil {
		return Result{}, err
	}
	plannedFiles := append([]plannedFile(nil), outputFiles...)
	var removeAfterPublish []string
	modulePath := filepath.Join(absApp, modFileName)
	if moduleContext.Nested {
		modulePayload, err := moduleSource(options)
		if err != nil {
			return Result{}, err
		}
		plannedFiles = append(plannedFiles, plannedFile{path: modulePath, contents: []byte(modulePayload)})
	} else {
		removeAfterPublish = append(removeAfterPublish, modulePath)
		modulePath = ""
	}
	packageSource, err := appPackageSource(options)
	if err != nil {
		return Result{}, err
	}
	appSource, err := formatGeneratedGo(appFileName, []byte(packageSource))
	if err != nil {
		return Result{}, err
	}
	plannedFiles = append(plannedFiles, plannedFile{path: filepath.Join(absApp, appFileName), contents: appSource})
	lifecycleSources, err := lifecycleServiceFileSources(options)
	if err != nil {
		return Result{}, err
	}
	for _, name := range []string{lifecycleFileName, lifecycleJSName} {
		path := filepath.Join(absApp, name)
		source, ok := lifecycleSources[name]
		if !ok {
			removeAfterPublish = append(removeAfterPublish, path)
			continue
		}
		formatted, err := formatGeneratedGo(name, source)
		if err != nil {
			return Result{}, err
		}
		plannedFiles = append(plannedFiles, plannedFile{path: path, contents: formatted})
	}
	auditTestSource, err := GeneratedAuditTestSource(options)
	if err != nil {
		return Result{}, err
	}
	auditTestPath := filepath.Join(absApp, auditTestFileName)
	if len(auditTestSource) > 0 {
		plannedFiles = append(plannedFiles, plannedFile{path: auditTestPath, contents: auditTestSource})
	} else {
		removeAfterPublish = append(removeAfterPublish, auditTestPath)
	}
	scriptFiles, scriptPlannedFiles, err := collectInlineGoBlockFiles(absApp, options)
	if err != nil {
		return Result{}, err
	}
	plannedFiles = append(plannedFiles, scriptPlannedFiles...)
	addonGoBlockFiles, addonPlannedFiles, err := collectAddonGoBlockFiles(absApp, options)
	if err != nil {
		return Result{}, err
	}
	plannedFiles = append(plannedFiles, addonPlannedFiles...)
	files = append(files, scriptFiles...)
	files = append(files, addonGoBlockFiles...)
	mainSource, err := serverMainSource(moduleContext.ImportBase + "/" + appPackageDirName)
	if err != nil {
		return Result{}, err
	}
	plannedFiles = append(plannedFiles, plannedFile{path: filepath.Join(absApp, mainFileName), contents: []byte(mainSource)})
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(targetOutput, 0o755); err != nil {
		return Result{}, err
	}
	for _, file := range plannedFiles {
		if err := writeFileIfChanged(file.path, file.contents); err != nil {
			return Result{}, err
		}
	}
	if err := removeStaleOutputFiles(targetOutput, files); err != nil {
		return Result{}, err
	}
	for _, path := range removeAfterPublish {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return Result{}, err
		}
	}

	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, mainFileName),
		PackagePath: filepath.Join(absApp, appFileName),
		ModulePath:  modulePath,
		OutputDir:   targetOutput,
		Files:       files,
	}, nil
}

// GenerateBackendWithOptions writes a generated Go app that serves only
// request-time backend routes for feature-bound actions and APIs.
func GenerateBackendWithOptions(appDir string, options Options) (result Result, err error) {
	plan, err := PlanBackendApplication(options)
	if err != nil {
		return Result{}, err
	}
	return GenerateBackendWithPlan(appDir, plan)
}

// GenerateBackendWithPlan writes a generated Go app that serves only
// request-time backend routes from an application plan.
func GenerateBackendWithPlan(appDir string, plan ApplicationPlan) (result Result, err error) {
	defer recoverGeneratedIdentifierError(&err)

	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated backend app directory is required")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	options, err := plan.optionsForEmit()
	if err != nil {
		return Result{}, err
	}
	if !plan.backendOnly {
		return Result{}, fmt.Errorf("embedded application plan cannot be used for backend app generation")
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	moduleContext := resolveGeneratedModuleContext(absApp)
	modulePath, err := writeGeneratedModuleFile(absApp, moduleContext, options)
	if err != nil {
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
	mainSource, err := serverMainSource(moduleContext.ImportBase + "/" + appPackageDirName)
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
		ModulePath:  modulePath,
	}, nil
}

func writeGeneratedModuleFile(absApp string, context generatedModuleContext, options Options) (string, error) {
	nestedPath := filepath.Join(absApp, modFileName)
	if !context.Nested {
		if err := os.Remove(nestedPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return "", nil
	}
	modulePayload, err := moduleSource(options)
	if err != nil {
		return "", err
	}
	if err := writeFileIfChanged(nestedPath, []byte(modulePayload)); err != nil {
		return "", err
	}
	return nestedPath, nil
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
