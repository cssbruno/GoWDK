package ratelimit

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the rate-limit addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/ratelimit"

// Addon enables request-time rate limiting support.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("ratelimit", gowdk.FeatureRateLimit)
}
