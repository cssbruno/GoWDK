package clientrt

import (
	"embed"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
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

// IslandRuntimeSource returns the shared generated JavaScript island runtime.
func IslandRuntimeSource() string {
	return strings.ReplaceAll(assetSource("island.js"), "__GOWDK_EXPRESSION_SPEC__", clientlang.RuntimeExpressionSpecJSON())
}

// IslandJSSource returns the generated JavaScript island registration stub for
// one component.
func IslandJSSource(options IslandJSOptions) string {
	return replaceQuotedPlaceholder(`(() => {
  const component = "__GOWDK_COMPONENT__";
  const register = window.__gowdkRegisterJSIsland;
  if (typeof register === "function") {
    register(component);
    return;
  }
  const registry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
  registry.components[component] = true;
})();
`, "__GOWDK_COMPONENT__", options.Component)
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
	ComponentID  string
	ABIVersion   string
	WASMPath     string
	WASMExecPath string
}

// WASMIslandLoaderSource returns the generated WASM island host loader.
func WASMIslandLoaderSource(options WASMIslandLoaderOptions) string {
	componentID := options.ComponentID
	if componentID == "" {
		componentID = options.Component
	}
	source := assetSource("wasm_island_loader.js")
	source = replaceQuotedPlaceholder(source, "__GOWDK_COMPONENT__", options.Component)
	source = replaceQuotedPlaceholder(source, "__GOWDK_COMPONENT_ID__", componentID)
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
