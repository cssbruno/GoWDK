package asset

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
