package asset

import "strings"

// Manifest maps logical asset names to emitted paths.
type Manifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

// Resolve returns the emitted path for a logical asset name.
func (manifest Manifest) Resolve(name string) string {
	if manifest.Files == nil {
		return ""
	}
	return manifest.Files[name]
}

// URL returns the browser-facing URL for a logical asset name.
func (manifest Manifest) URL(name string) string {
	resolved := manifest.Resolve(name)
	if resolved == "" {
		return ""
	}
	if strings.HasPrefix(resolved, "/") {
		return resolved
	}
	return "/" + strings.TrimLeft(resolved, "/")
}
