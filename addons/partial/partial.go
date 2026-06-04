package partial

import "github.com/cssbruno/gowdk"

// Addon enables server fragments and partial swaps.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("partial", gowdk.FeaturePartial)
}
