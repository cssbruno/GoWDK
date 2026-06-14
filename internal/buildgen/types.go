package buildgen

type Artifact struct {
	PageID      string
	Route       string
	Path        string
	CachePolicy string
}

type CSSArtifact struct {
	Path        string
	LogicalPath string
	LogicalHref string
	Hash        string
	CachePolicy string
	SizeBytes   int64
}

type AssetArtifact struct {
	Path        string
	LogicalPath string
	Hash        string
	CachePolicy string
	SizeBytes   int64
}

type Result struct {
	Artifacts            []Artifact
	CSSArtifacts         []CSSArtifact
	AssetArtifacts       []AssetArtifact
	RouteManifestPath    string
	AssetManifestPath    string
	SitemapPath          string
	RobotsPath           string
	OpenAPIPath          string
	SecurityManifestPath string
	BuildReportPath      string
	Report               BuildReport
	WriteStats           WriteStats
}

type WriteStats struct {
	FilesWritten           int
	IdenticalWritesSkipped int
}

type MemoryResult struct {
	Result
	Files map[string][]byte
}

// MemoryBuildOptions controls path metadata for in-memory builds.
type MemoryBuildOptions struct {
	// OutputBase is a virtual output root used for artifact path metadata.
	// Empty defaults to "." and never requires a real directory on disk.
	OutputBase string
}

type plannedArtifact struct {
	Artifact
	contents []byte
}

type plannedCSSArtifact struct {
	CSSArtifact
	contents []byte
}

type plannedAssetArtifact struct {
	AssetArtifact
	contents []byte
}

type buildPlan struct {
	pages  []plannedArtifact
	css    []plannedCSSArtifact
	assets []plannedAssetArtifact
}
