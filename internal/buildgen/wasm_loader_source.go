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

func islandWASMLoaderSource(component gwdkir.Component) string {
	return clientrt.WASMIslandLoaderSource(clientrt.WASMIslandLoaderOptions{
		Component:    component.Name,
		ComponentID:  islandComponentID(component.Package, component.Name),
		ABIVersion:   wasmIslandABIVersion,
		WASMPath:     "/" + islandWASMAssetPath(component.Package, component.Name),
		WASMExecPath: "/" + islandWASMExecAssetPath(),
	})
}
