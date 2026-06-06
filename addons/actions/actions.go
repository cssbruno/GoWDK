package actions

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the actions addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/actions"

// Addon enables typed backend actions and form handling.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("actions", gowdk.FeatureActions)
}
