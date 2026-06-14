package asset

import "strings"

// Manifest maps logical asset names to emitted paths.
type Manifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
	Hashes  map[string]string `json:"hashes,omitempty"`
	Cache   map[string]string `json:"cache,omitempty"`
	Sizes   map[string]int64  `json:"sizes,omitempty"`
}

// Resolve returns the emitted path for a logical asset name.
func (manifest Manifest) Resolve(name string) string {
	if manifest.Files == nil {
		return ""
	}
	return manifest.Files[name]
}

// Hash returns the content hash recorded for a logical asset name.
func (manifest Manifest) Hash(name string) string {
	if manifest.Hashes == nil {
		return ""
	}
	return manifest.Hashes[name]
}

// CachePolicy returns the HTTP cache policy recorded for a logical asset name.
func (manifest Manifest) CachePolicy(name string) string {
	if manifest.Cache == nil {
		return ""
	}
	return manifest.Cache[name]
}

// SizeBytes returns the generated asset byte size recorded for a logical asset
// name, or zero when the manifest has no size metadata for that asset.
func (manifest Manifest) SizeBytes(name string) int64 {
	if manifest.Sizes == nil {
		return 0
	}
	return manifest.Sizes[name]
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
