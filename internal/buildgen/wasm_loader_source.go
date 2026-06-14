package buildgen

import (
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

const wasmIslandABIVersion = "gowdk-wasm-island-v1"

func clientGoBlockWASMLoaderSource(page gwdkir.Page) string {
	return clientrt.ClientGoBlockWASMLoaderSource(clientrt.ClientGoBlockWASMLoaderOptions{
		PageID:       page.ID,
		LoaderPath:   "/" + clientGoBlockWASMLoaderAssetPath(page),
		WASMPath:     "/" + clientGoBlockWASMAssetPath(page),
		WASMExecPath: "/" + islandWASMExecAssetPath(),
		MountExport:  clientGoBlockMountExportName(page),
	})
}

func islandWASMLoaderSource(componentName string) string {
	return clientrt.WASMIslandLoaderSource(clientrt.WASMIslandLoaderOptions{
		Component:    componentName,
		ABIVersion:   wasmIslandABIVersion,
		WASMPath:     "/" + islandWASMAssetPath(componentName),
		WASMExecPath: "/" + islandWASMExecAssetPath(),
	})
}
