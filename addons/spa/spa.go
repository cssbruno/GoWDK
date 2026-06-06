package spa

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the SPA addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/spa"

// Addon enables build-time prerendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("spa", gowdk.FeatureSPA)
}
