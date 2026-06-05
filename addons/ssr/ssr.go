package ssr

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the SSR addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/ssr"

// Addon enables request-time full-page rendering.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("ssr", gowdk.FeatureSSR)
}
