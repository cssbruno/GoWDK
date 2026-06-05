package spa

// PathSet is produced by paths {} blocks for dynamic SPA routes.
type PathSet []Path

// Path contains route parameter values for one prerendered route.
type Path map[string]string
