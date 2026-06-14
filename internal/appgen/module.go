package appgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/goblockgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

const gowdkRuntimeModulePath = "github.com/cssbruno/gowdk"

func moduleSource(options Options) (string, error) {
	version := gowdkRuntimeModuleVersion()
	if version == "" {
		version = "v0.0.0"
	}

	lines := []string{
		"module gowdk-generated-app",
		"",
		"go 1.26.4",
		"",
		"require " + gowdkRuntimeModulePath + " " + version,
	}

	if version == "v0.0.0" {
		root, ok := gowdkRuntimeModuleRoot()
		if !ok {
			return "", fmt.Errorf("cannot locate %s module root for generated app runtime imports", gowdkRuntimeModulePath)
		}
		lines = append(lines, "", "replace "+gowdkRuntimeModulePath+" => "+filepath.ToSlash(root))
	}
	appModule, err := currentAppModule()
	if err != nil {
		// A missing/broken main module is only fatal when the generated app
		// imports app-owned packages: without the module path we cannot add the
		// require/replace, so the generated app would otherwise fail to build
		// later with an opaque "cannot find package" error. When the app imports
		// nothing app-owned, the module is not needed and the failure is ignored.
		if appHasLocalModuleImports(options) {
			return "", fmt.Errorf("cannot determine the app Go module for generated app imports: %w", err)
		}
	} else if appModule.Path != gowdkRuntimeModulePath && optionsUsesModuleImports(options, appModule.Path) {
		lines = append(lines,
			"",
			"require "+appModule.Path+" v0.0.0",
			"replace "+appModule.Path+" => "+filepath.ToSlash(appModule.Dir),
		)
	}

	return strings.Join(lines, "\n") + "\n", nil
}

type appModuleInfo struct {
	Path  string
	Dir   string
	GoMod string
}

func currentAppModule() (appModuleInfo, error) {
	command := exec.Command("go", "list", "-m", "-json")
	output, err := command.Output()
	if err != nil {
		return appModuleInfo{}, goListModuleError(err)
	}
	goModOutput, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return appModuleInfo{}, goListModuleError(err)
	}
	return parseCurrentAppModule(output, strings.TrimSpace(string(goModOutput)))
}

func parseCurrentAppModule(output []byte, currentGoMod string) (appModuleInfo, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	var modules []appModuleInfo
	for {
		var info appModuleInfo
		if err := decoder.Decode(&info); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return appModuleInfo{}, fmt.Errorf("parse go list -m output: %w", err)
		}
		modules = append(modules, info)
	}
	if len(modules) == 0 {
		return appModuleInfo{}, fmt.Errorf("go list -m did not report a main module path and directory")
	}
	if currentGoMod != "" && currentGoMod != os.DevNull {
		cleanCurrentGoMod := filepath.Clean(currentGoMod)
		for _, info := range modules {
			if filepath.Clean(info.GoMod) == cleanCurrentGoMod {
				return validateAppModuleInfo(info)
			}
		}
	}
	if len(modules) == 1 {
		return validateAppModuleInfo(modules[0])
	}
	return appModuleInfo{}, fmt.Errorf("go list -m reported %d workspace modules but none matched current go.mod %q", len(modules), currentGoMod)
}

func validateAppModuleInfo(info appModuleInfo) (appModuleInfo, error) {
	if strings.TrimSpace(info.Path) == "" || strings.TrimSpace(info.Dir) == "" {
		return appModuleInfo{}, fmt.Errorf("go list -m did not report a main module path and directory")
	}
	return info, nil
}

// goListModuleError surfaces the underlying go list -m failure, including its
// stderr (e.g. a missing go.mod), instead of an opaque exit status.
func goListModuleError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
			return fmt.Errorf("%w\n%s", err, stderr)
		}
	}
	return err
}

// appHasLocalModuleImports reports whether the generated app imports any
// app-owned package (a module-path import that is not stdlib and not the GOWDK
// runtime module), which is exactly the case that needs the main module's
// require/replace lines.
func appHasLocalModuleImports(options Options) bool {
	for path := range appBackendImportPaths(options) {
		if isLocalModuleImportPath(path) {
			return true
		}
	}
	return false
}

func isLocalModuleImportPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || path == gowdkRuntimeModulePath || strings.HasPrefix(path, gowdkRuntimeModulePath+"/") {
		return false
	}
	first := path
	if index := strings.Index(path, "/"); index >= 0 {
		first = path[:index]
	}
	// Standard-library import paths have no dot in their first segment.
	return strings.Contains(first, ".")
}

func optionsUsesModuleImports(options Options, modulePath string) bool {
	modulePath = strings.TrimRight(strings.TrimSpace(modulePath), "/")
	if modulePath == "" {
		return false
	}
	for importPath := range appBackendImportPaths(options) {
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			return true
		}
	}
	return false
}

// appBackendImportPaths collects every Go import the generated app's backend
// glue pulls in: request-time handlers, contract exposures, and inline go {}
// blocks.
func appBackendImportPaths(options Options) map[string]bool {
	paths := map[string]bool{}
	adapter := backendAdapterIR(options)
	for importPath := range backendImports(adapter, options.SSR) {
		paths[importPath] = true
	}
	for importPath := range backendContractImports(executableContractExposures(adapter.ContractExposures)) {
		paths[importPath] = true
	}
	for importPath := range inlineGoBlockImports(options.IR) {
		paths[importPath] = true
	}
	return paths
}

func inlineGoBlockImports(ir *gwdkir.Program) map[string]bool {
	imports := map[string]bool{}
	if ir == nil {
		return imports
	}
	for _, group := range inlineGoBlockGroups(*ir) {
		for _, item := range group.imports {
			if strings.TrimSpace(item.Path) != "" {
				imports[item.Path] = true
			}
		}
		for _, script := range group.goBlocks {
			file, err := goblockgen.ParseFile("goBlocks", script)
			if err != nil {
				continue
			}
			for _, spec := range file.Imports {
				importPath := importSpecPath(spec)
				if importPath != "" {
					imports[importPath] = true
				}
			}
		}
	}
	return imports
}

func importSpecPath(spec *ast.ImportSpec) string {
	if spec == nil || spec.Path == nil {
		return ""
	}
	importPath := strings.Trim(spec.Path.Value, `"`)
	if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil {
		importPath = unquoted
	}
	return strings.TrimSpace(importPath)
}

func gowdkRuntimeModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == gowdkRuntimeModulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, dependency := range info.Deps {
		if dependency.Path == gowdkRuntimeModulePath && dependency.Version != "" && dependency.Version != "(devel)" {
			return dependency.Version
		}
	}
	return ""
}

func gowdkRuntimeModuleRoot() (string, bool) {
	if _, file, _, ok := runtime.Caller(0); ok {
		if root, ok := findModuleRoot(filepath.Dir(file)); ok {
			return root, true
		}
	}
	wd, err := os.Getwd()
	if err == nil {
		if root, ok := findModuleRoot(wd); ok {
			return root, true
		}
	}
	return "", false
}

func findModuleRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		payload, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && modulePath(string(payload)) == gowdkRuntimeModulePath {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func modulePath(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
