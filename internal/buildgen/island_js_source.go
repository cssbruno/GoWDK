package buildgen

import (
	"path"

	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func islandJSSource(component gwdkir.Component, includeSourceMap bool) string {
	name := componentAssetName(component.Name)
	source := clientrt.IslandJSSource(clientrt.IslandJSOptions{
		Component:       islandComponentID(component.Package, component.Name),
		MountFunction:   "mount" + name + "Island",
		DestroyFunction: "destroy" + name + "Island",
	})
	if includeSourceMap {
		source += "//# sourceMappingURL=" + path.Base(islandJSSourceMapAssetPath(component.Package, component.Name)) + "\n"
	}
	return source
}
