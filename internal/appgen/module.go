package appgen

import (
	"encoding/json"
	"fmt"
	"go/ast"
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

	var builder strings.Builder
	builder.WriteString("module gowdk-generated-app\n\n")
	builder.WriteString("go 1.26.4\n\n")
	builder.WriteString("require ")
	builder.WriteString(gowdkRuntimeModulePath)
	builder.WriteByte(' ')
	builder.WriteString(version)
	builder.WriteByte('\n')

	if version == "v0.0.0" {
		root, ok := gowdkRuntimeModuleRoot()
		if !ok {
			return "", fmt.Errorf("cannot locate %s module root for generated app runtime imports", gowdkRuntimeModulePath)
		}
		builder.WriteByte('\n')
		builder.WriteString("replace ")
		builder.WriteString(gowdkRuntimeModulePath)
		builder.WriteString(" => ")
		builder.WriteString(filepath.ToSlash(root))
		builder.WriteByte('\n')
	}
	if appModule, ok := currentAppModule(); ok && appModule.Path != gowdkRuntimeModulePath && optionsUsesModuleImports(options, appModule.Path) {
		builder.WriteByte('\n')
		builder.WriteString("require ")
		builder.WriteString(appModule.Path)
		builder.WriteString(" v0.0.0\n")
		builder.WriteString("replace ")
		builder.WriteString(appModule.Path)
		builder.WriteString(" => ")
		builder.WriteString(filepath.ToSlash(appModule.Dir))
		builder.WriteByte('\n')
	}

	return builder.String(), nil
}

type appModuleInfo struct {
	Path string
	Dir  string
}

func currentAppModule() (appModuleInfo, bool) {
	command := exec.Command("go", "list", "-m", "-json")
	output, err := command.Output()
	if err != nil {
		return appModuleInfo{}, false
	}
	var info appModuleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return appModuleInfo{}, false
	}
	if strings.TrimSpace(info.Path) == "" || strings.TrimSpace(info.Dir) == "" {
		return appModuleInfo{}, false
	}
	return info, true
}

func optionsUsesModuleImports(options Options, modulePath string) bool {
	modulePath = strings.TrimRight(strings.TrimSpace(modulePath), "/")
	if modulePath == "" {
		return false
	}
	for importPath := range backendImports(options.Actions, options.APIs, options.Fragments, options.SSR) {
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			return true
		}
	}
	for importPath := range backendContractImports(executableContractExposures(backendAdapterIR(options).ContractExposures)) {
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			return true
		}
	}
	for importPath := range inlineGoBlockImports(options.IR) {
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			return true
		}
	}
	return false
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
