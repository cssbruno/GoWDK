package static

import "github.com/gowdk/gowdk"

// Addon enables build-time prerendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("static", gowdk.FeatureStatic)
}
