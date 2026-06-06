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
}

type AssetArtifact struct {
	Path        string
	LogicalPath string
	Hash        string
	CachePolicy string
}

type Result struct {
	Artifacts         []Artifact
	CSSArtifacts      []CSSArtifact
	AssetArtifacts    []AssetArtifact
	RouteManifestPath string
	AssetManifestPath string
	BuildReportPath   string
	Report            BuildReport
}

type MemoryResult struct {
	Result
	Files map[string][]byte
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
