package actions

import "github.com/gowdk/gowdk"

// Addon enables typed backend actions and form handling.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("actions", gowdk.FeatureActions)
}
