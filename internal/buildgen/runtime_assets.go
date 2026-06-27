package buildgen

import (
	"path/filepath"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

func clientRuntimeArtifacts(config gowdk.Config, ir gwdkir.Program, outputDir string, layouts map[string]gwdkir.Layout, components map[string]view.Component) ([]plannedAssetArtifact, error) {
	queryTypeNames := queryInvalidationTypeNames(ir.QueryInvalidations)
	for _, page := range ir.Pages {
		viewNodes, err := composePageViewNodes(page, layouts)
		if err != nil {
			return nil, err
		}
		usesSPANavigation, err := pageUsesSPANavigationRuntime(config, page, viewNodes, components)
		if err != nil {
			return nil, err
		}
		usesRealtime, err := pageUsesRealtimeRuntime(page, viewNodes, components, queryTypeNames)
		if err != nil {
			return nil, err
		}
		usesCommand, err := pageUsesCommandRuntime(page, viewNodes, components)
		if err != nil {
			return nil, err
		}
		if pageUsesPartialRuntime(page, viewNodes) || usesSPANavigation || usesRealtime || usesCommand {
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
	clientRuntime, err := clientRuntimeArtifacts(config, ir, outputDir, layouts, components)
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
