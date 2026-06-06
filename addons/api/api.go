package api

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the API addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/api"

// Addon enables generated API handlers.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("api", gowdk.FeatureAPI)
}
