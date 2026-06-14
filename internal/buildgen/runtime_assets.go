package buildgen

import (
	"path/filepath"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
)

func clientRuntimeArtifacts(config gowdk.Config, pages []gwdkir.Page, outputDir string, layouts map[string]gwdkir.Layout, components map[string]view.Component) ([]plannedAssetArtifact, error) {
	for _, page := range pages {
		viewSource, err := composePageViewSource(page, layouts)
		if err != nil {
			return nil, err
		}
		usesSPANavigation, err := pageUsesSPANavigationRuntime(config, page, viewSource, composedPageViewNodes(page), components)
		if err != nil {
			return nil, err
		}
		if pageUsesPartialRuntime(page, viewSource) || usesSPANavigation {
			return []plannedAssetArtifact{{
				AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath))},
				contents:             clientrt.Source(),
				obfuscationCandidate: true,
			}}, nil
		}
	}
	return nil, nil
}

func runtimeArtifacts(config gowdk.Config, ir gwdkir.Program, outputDir string, layouts map[string]gwdkir.Layout, components map[string]view.Component) ([]plannedAssetArtifact, error) {
	var artifacts []plannedAssetArtifact
	clientRuntime, err := clientRuntimeArtifacts(config, ir.Pages, outputDir, layouts, components)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, clientRuntime...)
	artifacts = append(artifacts, storeRuntimeArtifacts(ir.Pages, outputDir)...)
	islands, err := islandRuntimeArtifacts(config, ir.Pages, ir.Components, outputDir, layouts)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, islands...)
	clientGoBlocks, err := clientGoBlockRuntimeArtifacts(ir.Pages, outputDir)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, clientGoBlocks...)
	return dedupeAssetArtifacts(artifacts), nil
}

func storeRuntimeArtifacts(pages []gwdkir.Page, outputDir string) []plannedAssetArtifact {
	for _, page := range pages {
		if len(page.Stores) > 0 {
			return []plannedAssetArtifact{{
				AssetArtifact:        AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(storeRuntimeAssetPath))},
				contents:             []byte(storeRuntimeSource()),
				obfuscationCandidate: true,
			}}
		}
	}
	return nil
}

func storeRuntimeSource() string {
	return compactGeneratedJSSource(clientrt.StoreSource())
}
