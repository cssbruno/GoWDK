package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// islandWASMExecGoVersion reports the Go toolchain version that supplies
// wasm_exec.js. It reads $GOROOT/VERSION -- the same GOROOT that
// islandWASMExecArtifact reads wasm_exec.js from -- so the reported version
// matches the emitted glue even when this binary was compiled by a different
// toolchain. It falls back to the build version when VERSION is unreadable.
func islandWASMExecGoVersion() string {
	if contents, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "VERSION")); err == nil {
		line, _, _ := strings.Cut(strings.TrimSpace(string(contents)), "\n")
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return runtime.Version()
}

func islandWASMLoaderArtifact(outputDir string, component gwdkir.Component) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(component.Package, component.Name)
	return plannedAssetArtifact{
		AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:             []byte(islandWASMLoaderSource(component)),
		obfuscationCandidate: true,
	}
}

func clientGoBlockWASMLoaderArtifact(outputDir string, page gwdkir.Page) plannedAssetArtifact {
	assetPath := clientGoBlockWASMLoaderAssetPath(page)
	return plannedAssetArtifact{
		AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:             []byte(clientGoBlockWASMLoaderSource(page)),
		obfuscationCandidate: true,
	}
}

func islandWASMExecArtifact(outputDir string) (plannedAssetArtifact, error) {
	assetPath := islandWASMExecAssetPath()
	contents, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return plannedAssetArtifact{}, fmt.Errorf("read Go wasm_exec.js runtime: %w", err)
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      contents,
	}, nil
}

func islandJSAssetPath(packageName, componentName string) string {
	return islandComponentAssetPath(packageName, componentName, ".js")
}

func islandSharedRuntimeAssetPath() string {
	return path.Join(islandRuntimeDir, "island.js")
}

func islandWASMAssetPath(packageName, componentName string) string {
	return islandComponentAssetPath(packageName, componentName, ".wasm")
}

func islandWASMLoaderAssetPath(packageName, componentName string) string {
	return islandComponentAssetPath(packageName, componentName, ".wasm.js")
}

func clientGoBlockWASMAssetPath(page gwdkir.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm")
}

func clientGoBlockWASMLoaderAssetPath(page gwdkir.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm.js")
}

func islandWASMExecAssetPath() string {
	return path.Join(islandRuntimeDir, "wasm_exec.js")
}

func islandJSSourceMapAssetPath(packageName, componentName string) string {
	return islandComponentAssetPath(packageName, componentName, ".js.map")
}

func componentAssetName(componentName string) string {
	return source.ExportedIdentifier(componentName, "Component")
}

func islandComponentAssetPath(packageName, componentName, suffix string) string {
	componentPart := componentAssetName(componentName)
	if strings.TrimSpace(packageName) == "" {
		return path.Join(islandRuntimeDir, componentPart+suffix)
	}
	packagePart := safeCSSPathPart(packageName)
	if packagePart == "" {
		packagePart = "_"
	}
	return path.Join(islandRuntimeDir, packagePart, componentPart+suffix)
}

func islandComponentID(packageName, componentName string) string {
	if strings.TrimSpace(packageName) == "" {
		return componentName
	}
	return packageName + "." + componentName
}

func clientGoBlockAssetName(page gwdkir.Page) string {
	return source.ExportedIdentifier(page.ID, "Page")
}

func clientGoBlockMountExportName(page gwdkir.Page) string {
	return "GOWDKMount" + clientGoBlockAssetName(page)
}
