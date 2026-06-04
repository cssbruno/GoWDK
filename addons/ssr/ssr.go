package ssr

import "github.com/cssbruno/gowdk"

// Addon enables request-time full-page rendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("ssr", gowdk.FeatureSSR)
}
