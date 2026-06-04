package gowdk

import "fmt"

// RenderMode describes where full-page HTML is produced.
type RenderMode string

const (
	// Static renders full pages at build time.
	Static RenderMode = "static"
	// Action renders the page statically while allowing backend actions.
	Action RenderMode = "action"
	// Hybrid allows a route to combine static output and request-time behavior.
	Hybrid RenderMode = "hybrid"
	// SSR renders full pages at request time through the SSR addon.
	SSR RenderMode = "ssr"
)

// ParseRenderMode validates a render mode from source.
func ParseRenderMode(value string) (RenderMode, error) {
	mode := RenderMode(value)
	switch mode {
	case Static, Action, Hybrid, SSR:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown render mode %q", value)
	}
}

// RequiresSSR reports whether this mode needs the SSR addon.
func (mode RenderMode) RequiresSSR() bool {
	return mode == SSR || mode == Hybrid
}

// IsBuildTime reports whether route params must be known at build time.
func (mode RenderMode) IsBuildTime() bool {
	return mode == Static || mode == Action
}
