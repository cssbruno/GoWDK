package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func planScopedJSAssets(assets []gwdkir.Asset, outputDir string) ([]plannedAssetArtifact, []string) {
	var planned []plannedAssetArtifact
	var failures []string
	seen := map[string]bool{}
	for _, asset := range assets {
		if asset.Kind != gwdkir.AssetJS {
			continue
		}
		sourcePath, err := scopedJSSourcePath(asset.Source, asset.Path)
		if err != nil {
			failures = append(failures, scopedJSErrorPrefix(asset)+err.Error())
			continue
		}
		contents, err := os.ReadFile(sourcePath)
		if err != nil {
			failures = append(failures, scopedJSErrorPrefix(asset)+err.Error())
			continue
		}
		logicalPath := scopedJSLogicalPath(asset)
		outputPath, err := cssOutputPath(outputDir, logicalPath)
		if err != nil {
			failures = append(failures, scopedJSErrorPrefix(asset)+err.Error())
			continue
		}
		if seen[outputPath] {
			failures = append(failures, scopedJSErrorPrefix(asset)+fmt.Sprintf("duplicate script output path %q", logicalPath))
			continue
		}
		seen[outputPath] = true
		planned = append(planned, plannedAssetArtifact{
			AssetArtifact: AssetArtifact{
				Path:        outputPath,
				LogicalPath: logicalPath,
				Hash:        contentHash(contents),
				CachePolicy: immutableAssetCachePolicy,
			},
			contents: contents,
		})
	}
	return planned, failures
}

func scopedJSErrorPrefix(asset gwdkir.Asset) string {
	owner := "page"
	if scopedJSOwnerKind(asset.Source) == "component" {
		owner = "component"
	}
	return fmt.Sprintf("%s %s js %q: ", owner, asset.OwnerID, asset.Path)
}

func scopedJSSourcePath(ownerSource string, scriptPath string) (string, error) {
	scriptPath = strings.TrimSpace(scriptPath)
	if scriptPath == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(scriptPath) {
		return "", fmt.Errorf("path must be relative")
	}
	if strings.ContainsAny(scriptPath, "\x00?#") {
		return "", fmt.Errorf("path must not contain query, fragment, or NUL")
	}
	ext := strings.ToLower(path.Ext(filepath.ToSlash(scriptPath)))
	if ext != ".js" && ext != ".mjs" {
		return "", fmt.Errorf("path must end in .js or .mjs")
	}
	baseDir := "."
	if strings.TrimSpace(ownerSource) != "" {
		baseDir = filepath.Dir(filepath.FromSlash(ownerSource))
	}
	return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(scriptPath))), nil
}

func scopedJSLogicalPath(asset gwdkir.Asset) string {
	if scopedJSOwnerKind(asset.Source) == "component" {
		return componentScopedJSLogicalPath(asset.Package, asset.OwnerID, asset.Path)
	}
	return pageScopedJSLogicalPath(asset.OwnerID, asset.Path)
}

func scopedJSOwnerKind(source string) string {
	if strings.HasSuffix(filepath.ToSlash(source), ".cmp.gwdk") {
		return "component"
	}
	return "page"
}

func pageScopedJSLogicalPath(pageID string, scriptPath string) string {
	pagePart := safeCSSPathPart(pageID)
	if pagePart == "" {
		pagePart = "page"
	}
	return path.Join(defaultPageCSSDir, "pages", pagePart, safeComponentFileAssetName(scriptPath))
}

func componentScopedJSLogicalPath(packageName string, componentName string, scriptPath string) string {
	packagePart := safeCSSPathPart(packageName)
	if packagePart == "" {
		packagePart = "_"
	}
	componentPart := safeCSSPathPart(componentAssetName(componentName))
	if componentPart == "" {
		componentPart = "component"
	}
	return path.Join(defaultPageCSSDir, "components", packagePart, componentPart, safeComponentFileAssetName(scriptPath))
}

func scopedScriptHrefs(page manifest.Page, viewSource string, components map[string]view.Component) []string {
	seen := map[string]bool{}
	var scripts []string
	add := func(href string) {
		if href == "" || seen[href] {
			return
		}
		seen[href] = true
		scripts = append(scripts, href)
	}
	for _, script := range page.JS {
		add("/" + pageScopedJSLogicalPath(page.ID, script))
	}
	componentHrefs := scopedComponentScriptHrefs(page, viewSource, components)
	for _, href := range componentHrefs {
		add(href)
	}
	return scripts
}

func scopedComponentScriptHrefs(page manifest.Page, viewSource string, components map[string]view.Component) []string {
	usages, err := recursiveViewComponentCallUsages(viewSource, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var scripts []string
	for _, usage := range usages {
		component := usage.component
		for _, script := range component.JS {
			href := "/" + componentScopedJSLogicalPath(component.Package, component.Name, script)
			if seen[href] {
				continue
			}
			seen[href] = true
			scripts = append(scripts, href)
		}
	}
	sort.Strings(scripts)
	return scripts
}
