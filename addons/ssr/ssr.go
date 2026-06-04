package ssr

import "github.com/gowdk/gowdk"

// Addon enables request-time full-page rendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("ssr", gowdk.FeatureSSR)
}
