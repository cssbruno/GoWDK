package embed

import "github.com/gowdk/gowdk"

// Addon enables one-binary embedded asset serving.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("embed", gowdk.FeatureEmbed)
}
