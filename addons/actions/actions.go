package actions

import "github.com/cssbruno/gowdk"

// Addon enables typed backend actions and form handling.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("actions", gowdk.FeatureActions)
}
