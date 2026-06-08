package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func planComponentFileAssets(assets []gwdkir.Asset, outputDir string) ([]plannedAssetArtifact, []string) {
	var planned []plannedAssetArtifact
	var failures []string
	seen := map[string]bool{}
	for _, asset := range assets {
		if asset.Kind != gwdkir.AssetFile {
			continue
		}
		sourcePath, err := componentFileAssetSourcePath(asset.Source, asset.Path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s asset %q: %v", asset.OwnerID, asset.Path, err))
			continue
		}
		contents, err := os.ReadFile(sourcePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s asset %q: %v", asset.OwnerID, asset.Path, err))
			continue
		}
		logicalPath := componentFileAssetLogicalPath(asset)
		hash := contentHash(contents)
		emittedPath := hashedAssetPath(logicalPath, hash)
		outputPath, err := cssOutputPath(outputDir, emittedPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s asset %q: %v", asset.OwnerID, asset.Path, err))
			continue
		}
		if seen[outputPath] {
			failures = append(failures, fmt.Sprintf("component %s asset %q: duplicate asset path %q", asset.OwnerID, asset.Path, outputPath))
			continue
		}
		seen[outputPath] = true
		planned = append(planned, plannedAssetArtifact{
			AssetArtifact: AssetArtifact{
				Path:        outputPath,
				LogicalPath: logicalPath,
				Hash:        hash,
				CachePolicy: immutableAssetCachePolicy,
			},
			contents: contents,
		})
	}
	return planned, failures
}

func componentFileAssetSourcePath(componentSource string, assetPath string) (string, error) {
	if strings.TrimSpace(assetPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(assetPath) {
		return "", fmt.Errorf("path must be relative")
	}
	baseDir := "."
	if strings.TrimSpace(componentSource) != "" {
		baseDir = filepath.Dir(filepath.FromSlash(componentSource))
	}
	return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(assetPath))), nil
}

func componentFileAssetLogicalPath(asset gwdkir.Asset) string {
	packagePart := safeCSSPathPart(asset.Package)
	if packagePart == "" {
		packagePart = "_"
	}
	componentPart := safeCSSPathPart(componentAssetName(asset.OwnerID))
	if componentPart == "" {
		componentPart = "component"
	}
	filePart := safeComponentFileAssetName(asset.Path)
	return path.Join(defaultPageCSSDir, "components", packagePart, componentPart, filePart)
}

func safeComponentFileAssetName(assetPath string) string {
	base := path.Base(filepath.ToSlash(strings.TrimSpace(assetPath)))
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	stem = safeCSSPathPart(stem)
	if stem == "" {
		stem = "asset"
	}
	ext = safeCSSPathPart(strings.TrimPrefix(ext, "."))
	if ext == "" {
		return stem
	}
	return stem + "." + ext
}

func hashedAssetPath(logicalPath string, hash string) string {
	return hashedCSSPath(logicalPath, hash)
}
