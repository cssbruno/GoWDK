package partial

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the partial addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/partial"

// Addon enables server fragments and partial swaps.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("partial", gowdk.FeaturePartial)
}
