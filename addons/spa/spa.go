package spa

import "github.com/cssbruno/gowdk"

// Addon enables build-time prerendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("spa", gowdk.FeatureSPA)
}
