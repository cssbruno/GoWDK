// Package css registers compile-time CSS extension support.
package css

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the CSS addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/css"

// Addon enables compile-time CSS processing.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("css", gowdk.FeatureCSS)
}

// Processor is the compile-time CSS plugin contract.
type Processor = gowdk.CSSProcessor

// Context is the metadata passed to compile-time CSS plugins.
type Context = gowdk.CSSContext

// Result is returned by compile-time CSS plugins.
type Result = gowdk.CSSResult

// Asset is a CSS file emitted by a compile-time CSS plugin.
type Asset = gowdk.CSSAsset
