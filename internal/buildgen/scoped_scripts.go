package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	view "github.com/cssbruno/gowdk/internal/viewrender"
	"github.com/evanw/esbuild/pkg/api"
)

func planScopedJSAssets(assets []gwdkir.Asset, outputDir string) ([]plannedAssetArtifact, []string) {
	var planned []plannedAssetArtifact
	var failures []string
	seen := map[string]bool{}
	for _, asset := range assets {
		if asset.Kind != gwdkir.AssetJS {
			continue
		}
		inline := isInlineScopedScriptAsset(asset)
		contents := []byte(asset.Inline)
		sourcePath := ""
		if !inline {
			var err error
			sourcePath, err = scopedJSSourcePath(asset.Source, asset.Path)
			if err != nil {
				failures = append(failures, scopedJSErrorPrefix(asset)+err.Error())
				continue
			}
			contents, err = os.ReadFile(sourcePath)
			if err != nil {
				failures = append(failures, scopedJSErrorPrefix(asset)+err.Error())
				continue
			}
		} else if !validInlineJSAssetName(asset.Path) {
			failures = append(failures, scopedJSErrorPrefix(asset)+fmt.Sprintf("inline script name %q is invalid", asset.Path))
			continue
		}
		contents, err := transformScopedScriptContents(sourcePath, contents)
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
				// Scoped scripts are emitted at stable, unhashed paths, so
				// returning browsers must revalidate them after a deploy.
				CachePolicy: noCacheAssetCachePolicy,
			},
			contents: contents,
		})
	}
	return planned, failures
}

func isInlineScopedScriptAsset(asset gwdkir.Asset) bool {
	return asset.Name == "inline"
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
	if ext != ".js" && ext != ".mjs" && ext != ".ts" {
		return "", fmt.Errorf("path must end in .js, .mjs, or .ts")
	}
	baseDir := "."
	if strings.TrimSpace(ownerSource) != "" {
		baseDir = filepath.Dir(filepath.FromSlash(ownerSource))
	}
	return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(scriptPath))), nil
}

func validInlineJSAssetName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && !strings.ContainsAny(name, `/\?#`+"\x00") && strings.HasSuffix(name, ".js")
}

func transformScopedScriptContents(sourcePath string, contents []byte) ([]byte, error) {
	if strings.EqualFold(path.Ext(filepath.ToSlash(sourcePath)), ".ts") {
		result := api.Transform(string(contents), api.TransformOptions{
			Loader:     api.LoaderTS,
			Format:     api.FormatESModule,
			Sourcefile: filepath.ToSlash(sourcePath),
		})
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("typescript transform failed: %s", esbuildMessages(result.Errors))
		}
		return result.Code, nil
	}
	if len(contents) == 0 || contents[len(contents)-1] == '\n' {
		return contents, nil
	}
	return append(append([]byte(nil), contents...), '\n'), nil
}

func esbuildMessages(messages []api.Message) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.Location != nil {
			parts = append(parts, fmt.Sprintf("%s:%d:%d: %s", message.Location.File, message.Location.Line, message.Location.Column+1, message.Text))
			continue
		}
		parts = append(parts, message.Text)
	}
	return strings.Join(parts, "; ")
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
	return path.Join(defaultPageCSSDir, "pages", pagePart, safeScriptAssetName(scriptPath))
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
	return path.Join(defaultPageCSSDir, "components", packagePart, componentPart, safeScriptAssetName(scriptPath))
}

func safeScriptAssetName(scriptPath string) string {
	name := safeComponentFileAssetName(scriptPath)
	if strings.EqualFold(path.Ext(filepath.ToSlash(scriptPath)), ".ts") {
		ext := path.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		if stem == "" {
			stem = "script"
		}
		return stem + ".js"
	}
	return name
}

func scopedScriptHrefs(page gwdkir.Page, viewSource string, viewNodes []view.Node, components map[string]view.Component) ([]string, error) {
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
	for index, script := range page.InlineJS {
		name := script.Name
		if name == "" {
			name = source.InlineScriptName(index)
		}
		add("/" + pageScopedJSLogicalPath(page.ID, name))
	}
	componentHrefs, err := scopedComponentScriptHrefs(page, viewSource, viewNodes, components)
	if err != nil {
		return nil, err
	}
	for _, href := range componentHrefs {
		add(href)
	}
	return scripts, nil
}

func scopedComponentScriptHrefs(page gwdkir.Page, viewSource string, viewNodes []view.Node, components map[string]view.Component) ([]string, error) {
	usages, err := recursiveViewComponentCallUsagesForView(viewSource, viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return nil, err
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
		for index, script := range component.InlineJS {
			name := script.Name
			if name == "" {
				name = source.InlineScriptName(index)
			}
			href := "/" + componentScopedJSLogicalPath(component.Package, component.Name, name)
			if seen[href] {
				continue
			}
			seen[href] = true
			scripts = append(scripts, href)
		}
	}
	sort.Strings(scripts)
	return scripts, nil
}
