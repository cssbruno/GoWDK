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

func islandWASMLoaderArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandWASMLoaderSource(componentName)),
	}
}

func clientGoBlockWASMLoaderArtifact(outputDir string, page gwdkir.Page) plannedAssetArtifact {
	assetPath := clientGoBlockWASMLoaderAssetPath(page)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(clientGoBlockWASMLoaderSource(page)),
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

func islandJSAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js")
}

func islandWASMAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm")
}

func islandWASMLoaderAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm.js")
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

func islandJSSourceMapAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js.map")
}

func componentAssetName(componentName string) string {
	return source.ExportedIdentifier(componentName, "Component")
}

func clientGoBlockAssetName(page gwdkir.Page) string {
	return source.ExportedIdentifier(page.ID, "Page")
}

func clientGoBlockMountExportName(page gwdkir.Page) string {
	return "GOWDKMount" + clientGoBlockAssetName(page)
}
