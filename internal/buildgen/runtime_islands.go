package buildgen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

func islandRuntimeArtifacts(config gowdk.Config, pages []gwdkir.Page, allComponents []gwdkir.Component, outputDir string, layouts map[string]gwdkir.Layout) ([]plannedAssetArtifact, error) {
	components := componentsByName(allComponents)
	includeSourceMaps := config.Build.DebugAssets()
	planned := map[string]plannedAssetArtifact{}
	for _, page := range pages {
		nodes, err := composePageViewNodes(page, layouts)
		if err != nil {
			return nil, fmt.Errorf("compose island view nodes for page %q: %w", page.ID, err)
		}
		usages, err := recursiveComponentCallUsagesForView("", nodes, components, page.Package, componentUses(page.Uses), manifestComponentResolver)
		if err != nil {
			return nil, fmt.Errorf("resolve island components for page %q: %w", page.ID, err)
		}
		for _, usage := range usages {
			component := usage.component
			switch manifestComponentRuntimeMode(usage.call.Island, component) {
			case "wasm":
				if _, exists := planned[filepath.Join(outputDir, filepath.FromSlash(islandWASMAssetPath(component.Package, component.Name)))]; !exists {
					artifact, err := islandWASMArtifact(outputDir, component)
					if err != nil {
						return nil, err
					}
					addAsset(planned, artifact)
				}
				if strings.TrimSpace(component.WASM.Package) != "" {
					artifact, err := islandWASMExecArtifact(outputDir)
					if err != nil {
						return nil, err
					}
					addAsset(planned, artifact)
				}
				addAsset(planned, islandWASMLoaderArtifact(outputDir, component))
			case "":
				if componentNeedsJSIsland(component) || usage.call.ReactiveProps {
					addAsset(planned, islandSharedRuntimeArtifact(outputDir, includeSourceMaps))
					addAsset(planned, islandJSArtifact(outputDir, component, includeSourceMaps))
					if includeSourceMaps {
						addAsset(planned, islandJSSourceMapArtifact(outputDir, component))
					}
				}
			}
		}
	}
	if len(planned) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(planned))
	for path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	artifacts := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, planned[path])
	}
	return artifacts, nil
}

func islandScriptHrefs(source string, components map[string]view.Component, ownerPackage string, uses map[string]string) ([]string, error) {
	return islandScriptHrefsForView(source, nil, components, ownerPackage, uses)
}

func islandScriptHrefsForView(source string, nodes []view.Node, components map[string]view.Component, ownerPackage string, uses map[string]string) ([]string, error) {
	usages, err := recursiveComponentCallUsagesForView(source, nodes, components, ownerPackage, uses, viewComponentResolver)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	needsSharedRuntime := false
	var stubs []string
	for _, usage := range usages {
		href := ""
		component := usage.component
		switch viewComponentRuntimeMode(usage.call.Island, component) {
		case "wasm":
			href = "/" + islandWASMLoaderAssetPath(component.Package, component.Name)
		case "":
			if component.StateJSON != "" || component.HandlersJSON != "" || len(component.Emits) > 0 || len(component.Exports) > 0 || usage.call.ReactiveProps || componentViewHasAwait(component.Body, component.Nodes) {
				needsSharedRuntime = true
				href = "/" + islandJSAssetPath(component.Package, component.Name)
			}
		}
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		stubs = append(stubs, href)
	}
	sort.Strings(stubs)
	if needsSharedRuntime {
		return append([]string{"/" + islandSharedRuntimeAssetPath()}, stubs...), nil
	}
	return stubs, nil
}

func manifestComponentRuntimeMode(explicit string, component gwdkir.Component) string {
	if explicit != "" {
		return explicit
	}
	if strings.TrimSpace(component.WASM.Package) != "" {
		return "wasm"
	}
	return ""
}

func viewComponentRuntimeMode(explicit string, component view.Component) string {
	if explicit != "" {
		return explicit
	}
	return component.DefaultIsland
}

type componentResolver[T any] struct {
	Body     func(T) string
	Nodes    func(T) []view.Node
	Identity func(T) string
	Package  func(T) string
	Uses     func(T) map[string]string
}

type resolvedComponentCallUsage[T any] struct {
	call      viewanalysis.ComponentCallUsage
	component T
}

var viewComponentResolver = componentResolver[view.Component]{
	Body:     func(component view.Component) string { return component.Body },
	Nodes:    func(component view.Component) []view.Node { return component.Nodes },
	Identity: func(component view.Component) string { return component.Identity() },
	Package:  func(component view.Component) string { return component.Package },
	Uses:     func(component view.Component) map[string]string { return component.Uses },
}

var manifestComponentResolver = componentResolver[gwdkir.Component]{
	Body:     func(component gwdkir.Component) string { return component.Blocks.ViewBody },
	Nodes:    func(component gwdkir.Component) []view.Node { return component.Blocks.ViewNodes },
	Identity: manifestComponentIdentity,
	Package:  func(component gwdkir.Component) string { return component.Package },
	Uses:     func(component gwdkir.Component) map[string]string { return componentUses(component.Uses) },
}

func recursiveViewComponentCallUsages(source string, components map[string]view.Component, ownerPackage string, uses map[string]string) ([]resolvedComponentCallUsage[view.Component], error) {
	return recursiveViewComponentCallUsagesForView(source, nil, components, ownerPackage, uses)
}

func recursiveViewComponentCallUsagesForView(source string, nodes []view.Node, components map[string]view.Component, ownerPackage string, uses map[string]string) ([]resolvedComponentCallUsage[view.Component], error) {
	return recursiveComponentCallUsagesForView(source, nodes, components, ownerPackage, uses, viewComponentResolver)
}

func recursiveComponentCallUsages[T any](source string, components map[string]T, ownerPackage string, uses map[string]string, resolver componentResolver[T]) ([]resolvedComponentCallUsage[T], error) {
	return recursiveComponentCallUsagesForView(source, nil, components, ownerPackage, uses, resolver)
}

func recursiveComponentCallUsagesForView[T any](source string, nodes []view.Node, components map[string]T, ownerPackage string, uses map[string]string, resolver componentResolver[T]) ([]resolvedComponentCallUsage[T], error) {
	var usages []resolvedComponentCallUsage[T]
	visiting := map[string]bool{}
	var walk func(string, []view.Node, string, map[string]string) error
	walk = func(source string, nodes []view.Node, ownerPackage string, uses map[string]string) error {
		var direct []viewanalysis.ComponentCallUsage
		if len(nodes) > 0 {
			var err error
			direct, err = viewanalysis.ComponentCallUsagesFromNodes(nodes)
			if err != nil {
				return err
			}
		} else {
			var err error
			direct, err = viewanalysis.ComponentCallUsages(source)
			if err != nil {
				return err
			}
		}
		for _, usage := range direct {
			component, ok := lookupComponent(components, usage.Component, ownerPackage, uses)
			if !ok {
				continue
			}
			usages = append(usages, resolvedComponentCallUsage[T]{call: usage, component: component})
			identity := resolver.Identity(component)
			if visiting[identity] {
				continue
			}
			visiting[identity] = true
			if err := walk(resolver.Body(component), resolver.Nodes(component), resolver.Package(component), resolver.Uses(component)); err != nil {
				return err
			}
			delete(visiting, identity)
		}
		return nil
	}
	if err := walk(source, nodes, ownerPackage, uses); err != nil {
		return nil, err
	}
	return usages, nil
}

func lookupComponent[T any](components map[string]T, name string, ownerPackage string, uses map[string]string) (T, bool) {
	var zero T
	if strings.Contains(name, ".") {
		if component, ok := components[name]; ok {
			return component, true
		}
		alias, componentName, _ := strings.Cut(name, ".")
		packageName := uses[alias]
		if packageName == "" {
			return zero, false
		}
		component, ok := components[componentRegistryKey(packageName, componentName)]
		return component, ok
	}
	if ownerPackage != "" {
		component, ok := components[componentRegistryKey(ownerPackage, name)]
		return component, ok
	}
	component, ok := components[name]
	return component, ok
}

func statefulComponentNames(components []gwdkir.Component) map[string]bool {
	out := map[string]bool{}
	for _, component := range components {
		if componentNeedsJSIsland(component) {
			out[component.Name] = true
			if component.Package != "" {
				out[component.Package+"."+component.Name] = true
			}
		}
	}
	return out
}

func componentNeedsJSIsland(component gwdkir.Component) bool {
	return component.State.Type.Name != "" || component.Blocks.Client || len(component.Emits) > 0 || componentViewHasAwait(component.Blocks.ViewBody, component.Blocks.ViewNodes)
}

func componentViewHasAwait(source string, nodes []view.Node) bool {
	if len(nodes) == 0 && strings.TrimSpace(source) != "" {
		parsed, err := view.Parse(source)
		if err == nil {
			nodes = parsed
		}
	}
	return nodesHaveAwait(nodes)
}

func nodesHaveAwait(nodes []view.Node) bool {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.AwaitBlock:
			return true
		case view.Element:
			if nodesHaveAwait(typed.Children) {
				return true
			}
		case view.ComponentCall:
			if nodesHaveAwait(typed.Children) {
				return true
			}
		}
	}
	return false
}

func componentsByName(components []gwdkir.Component) map[string]gwdkir.Component {
	out := map[string]gwdkir.Component{}
	for _, component := range components {
		key := componentRegistryKey(component.Package, component.Name)
		out[key] = component
		if component.Package == "" {
			out[component.Name] = component
		}
	}
	return out
}

func addAsset(artifacts map[string]plannedAssetArtifact, artifact plannedAssetArtifact) {
	artifacts[artifact.Path] = artifact
}

func dedupeAssetArtifacts(artifacts []plannedAssetArtifact) []plannedAssetArtifact {
	if len(artifacts) < 2 {
		return artifacts
	}
	seen := map[string]plannedAssetArtifact{}
	for _, artifact := range artifacts {
		seen[artifact.Path] = artifact
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		out = append(out, seen[path])
	}
	return out
}

func islandSharedRuntimeArtifact(outputDir string, includeSourceMap bool) plannedAssetArtifact {
	assetPath := islandSharedRuntimeAssetPath()
	source := clientrt.IslandRuntimeSource()
	if !includeSourceMap {
		source = compactGeneratedJSSource(source)
	}
	return plannedAssetArtifact{
		AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:             []byte(source),
		obfuscationCandidate: true,
	}
}

func islandJSArtifact(outputDir string, component gwdkir.Component, includeSourceMap bool) plannedAssetArtifact {
	assetPath := islandJSAssetPath(component.Package, component.Name)
	source := islandJSSource(component, includeSourceMap)
	if !includeSourceMap {
		source = compactGeneratedJSSource(source)
	}
	return plannedAssetArtifact{
		AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:             []byte(source),
		obfuscationCandidate: true,
	}
}

func compactGeneratedJSSource(source string) string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func islandJSSourceMapArtifact(outputDir string, component gwdkir.Component) plannedAssetArtifact {
	assetPath := islandJSSourceMapAssetPath(component.Package, component.Name)
	source := islandJSSource(component, true)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      islandJSSourceMap(component, source),
	}
}
