package buildgen

import (
	"path"

	"github.com/cssbruno/gowdk/internal/clientrt"
)

func islandJSSource(componentName string, includeSourceMap bool) string {
	name := componentAssetName(componentName)
	source := clientrt.IslandJSSource(clientrt.IslandJSOptions{
		Component:       componentName,
		MountFunction:   "mount" + name + "Island",
		DestroyFunction: "destroy" + name + "Island",
	})
	if includeSourceMap {
		source += "//# sourceMappingURL=" + path.Base(islandJSSourceMapAssetPath(componentName)) + "\n"
	}
	return source
}
