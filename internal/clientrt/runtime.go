package clientrt

import (
	"embed"
	"strconv"
	"strings"
)

// Filename is the conventional output name for the generated client runtime.
const Filename = "gowdk.js"

//go:embed assets/*.js
var runtimeAssets embed.FS

// Source returns the first client runtime for partial updates and SPA
// navigation enhancement.
func Source() []byte {
	return []byte(assetSource("gowdk.js"))
}

// StoreSource returns the generated browser store persistence runtime.
func StoreSource() string {
	return assetSource("store.js")
}

// IslandJSOptions names the per-component values inserted into the JavaScript
// island runtime template.
type IslandJSOptions struct {
	Component       string
	MountFunction   string
	DestroyFunction string
}

// IslandJSSource returns the generated JavaScript island runtime for one component.
func IslandJSSource(options IslandJSOptions) string {
	source := assetSource("island.js")
	source = replaceQuotedPlaceholder(source, "__GOWDK_COMPONENT__", options.Component)
	source = strings.ReplaceAll(source, "__GOWDK_MOUNT_FUNCTION__", options.MountFunction)
	source = strings.ReplaceAll(source, "__GOWDK_DESTROY_FUNCTION__", options.DestroyFunction)
	return source
}

// ClientGoBlockWASMLoaderOptions names the per-page values inserted into the
// page-level Go WASM loader template.
type ClientGoBlockWASMLoaderOptions struct {
	PageID       string
	LoaderPath   string
	WASMPath     string
	WASMExecPath string
	MountExport  string
}

// ClientGoBlockWASMLoaderSource returns the generated page-level Go WASM loader.
func ClientGoBlockWASMLoaderSource(options ClientGoBlockWASMLoaderOptions) string {
	source := assetSource("client_go_wasm_loader.js")
	source = replaceQuotedPlaceholder(source, "__GOWDK_PAGE_ID__", options.PageID)
	source = replaceQuotedPlaceholder(source, "__GOWDK_LOADER_PATH__", options.LoaderPath)
	source = replaceQuotedPlaceholder(source, "__GOWDK_WASM_PATH__", options.WASMPath)
	source = replaceQuotedPlaceholder(source, "__GOWDK_WASM_EXEC_PATH__", options.WASMExecPath)
	source = replaceQuotedPlaceholder(source, "__GOWDK_MOUNT_EXPORT__", options.MountExport)
	return source
}

// WASMIslandLoaderOptions names the per-component values inserted into the
// component WASM island loader template.
type WASMIslandLoaderOptions struct {
	Component    string
	ABIVersion   string
	WASMPath     string
	WASMExecPath string
}

// WASMIslandLoaderSource returns the generated WASM island host loader.
func WASMIslandLoaderSource(options WASMIslandLoaderOptions) string {
	source := assetSource("wasm_island_loader.js")
	source = replaceQuotedPlaceholder(source, "__GOWDK_COMPONENT__", options.Component)
	source = replaceQuotedPlaceholder(source, "__GOWDK_ABI_VERSION__", options.ABIVersion)
	source = replaceQuotedPlaceholder(source, "__GOWDK_WASM_PATH__", options.WASMPath)
	source = replaceQuotedPlaceholder(source, "__GOWDK_WASM_EXEC_PATH__", options.WASMExecPath)
	return source
}

func assetSource(name string) string {
	contents, err := runtimeAssets.ReadFile("assets/" + name)
	if err != nil {
		panic("embedded GOWDK client runtime asset missing: " + name)
	}
	return string(contents)
}

func replaceQuotedPlaceholder(source string, placeholder string, value string) string {
	return strings.ReplaceAll(source, strconv.Quote(placeholder), strconv.Quote(value))
}
