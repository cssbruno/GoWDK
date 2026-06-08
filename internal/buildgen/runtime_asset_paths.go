package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func islandWASMLoaderArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandWASMLoaderSource(componentName)),
	}
}

func clientGoBlockWASMLoaderArtifact(outputDir string, page manifest.Page) plannedAssetArtifact {
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

func clientGoBlockWASMAssetPath(page manifest.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm")
}

func clientGoBlockWASMLoaderAssetPath(page manifest.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm.js")
}

func islandWASMExecAssetPath() string {
	return path.Join(islandRuntimeDir, "wasm_exec.js")
}

func islandJSSourceMapAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js.map")
}

func componentAssetName(componentName string) string {
	name := exportedPascalSafe(componentName)
	if name == "" {
		return "Component"
	}
	return name
}

func clientGoBlockAssetName(page manifest.Page) string {
	name := exportedPascalSafe(page.ID)
	if name == "" {
		return "Page"
	}
	return name
}

func clientGoBlockMountExportName(page manifest.Page) string {
	return "GOWDKMount" + clientGoBlockAssetName(page)
}

func exportedPascalSafe(value string) string {
	out := make([]rune, 0, len(value))
	upperNext := true
	for _, char := range value {
		validLower := char >= 'a' && char <= 'z'
		validUpper := char >= 'A' && char <= 'Z'
		validDigit := char >= '0' && char <= '9'
		if !validLower && !validUpper && !validDigit {
			upperNext = true
			continue
		}
		if len(out) == 0 && validDigit {
			out = append(out, 'P')
		}
		if upperNext && validLower {
			char -= 'a' - 'A'
		}
		out = append(out, char)
		upperNext = false
	}
	return string(out)
}
