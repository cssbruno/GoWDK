package embed

import "github.com/cssbruno/gowdk"

// Addon enables one-binary embedded asset serving.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("embed", gowdk.FeatureEmbed)
}
