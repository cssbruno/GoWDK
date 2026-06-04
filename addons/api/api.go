package api

import "github.com/gowdk/gowdk"

// Addon enables generated API handlers.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("api", gowdk.FeatureAPI)
}
