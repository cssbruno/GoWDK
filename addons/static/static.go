package static

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the static addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/static"

// Addon enables build-time static page output.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("static", gowdk.FeatureSPA)
}
