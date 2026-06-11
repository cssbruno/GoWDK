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
		usesSPANavigation, err := pageUsesSPANavigationRuntime(config, page, viewSource, components)
		if err != nil {
			return nil, err
		}
		if pageUsesPartialRuntime(page, viewSource) || usesSPANavigation {
			return []plannedAssetArtifact{{
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath))},
				contents:      clientrt.Source(),
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
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(storeRuntimeAssetPath))},
				contents:      []byte(storeRuntimeSource()),
			}}
		}
	}
	return nil
}

func storeRuntimeSource() string {
	return compactGeneratedJSSource(`(() => {
  const registry = window.__gowdkStores || (window.__gowdkStores = {
    stores: Object.create(null),
    listeners: Object.create(null)
  });

  registry.init = (name, state) => {
    if (!name || registry.stores[name]) return;
    registry.stores[name] = Object.assign({}, state || {});
  };

  registry.get = (name) => {
    return Object.assign({}, registry.stores[name] || {});
  };

  registry.set = (name, next) => {
    if (!name) return;
    registry.stores[name] = Object.assign({}, registry.stores[name] || {}, next || {});
    (registry.listeners[name] || []).slice().forEach((listener) => listener(registry.get(name)));
  };

  registry.subscribe = (name, listener) => {
    if (!name || typeof listener !== "function") return () => {};
    if (!registry.listeners[name]) registry.listeners[name] = [];
    registry.listeners[name].push(listener);
    return () => {
      registry.listeners[name] = (registry.listeners[name] || []).filter((item) => item !== listener);
    };
  };

  document.querySelectorAll("script[type=\"application/json\"][data-gowdk-store]").forEach((node) => {
    const name = node.getAttribute("data-gowdk-store");
    try {
      registry.init(name, JSON.parse(node.textContent || "{}"));
    } catch (error) {
      registry.init(name, {});
    }
  });
})();
`)
}
