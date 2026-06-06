package embed

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the embed addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/embed"

// Addon enables one-binary embedded asset serving.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("embed", gowdk.FeatureEmbed)
}
