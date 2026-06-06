package buildgen

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

const internalComponentKeyPrefix = "\x00component:"

func componentRegistryKey(packageName, componentName string) string {
	if packageName == "" {
		return componentName
	}
	return internalComponentKeyPrefix + packageName + "." + componentName
}

func componentRegistryForPage(page manifest.Page, registry map[string]view.Component) map[string]view.Component {
	if page.Package == "" && len(page.Uses) == 0 {
		return registry
	}
	out := map[string]view.Component{}
	for key, component := range registry {
		if key == "" {
			continue
		}
		out[key] = component
	}
	for _, component := range sortedViewComponents(registry) {
		if component.Package == "" {
			out[component.Name] = component
			continue
		}
		if component.Package == page.Package {
			out[component.Name] = component
		}
	}
	for _, use := range page.Uses {
		for _, component := range sortedViewComponents(registry) {
			if component.Package != use.Package {
				continue
			}
			out[component.Name] = component
			out[use.Alias+"."+component.Name] = component
		}
	}
	return out
}

func sortedViewComponents(registry map[string]view.Component) []view.Component {
	seen := map[string]view.Component{}
	for _, component := range registry {
		if component.Name == "" {
			continue
		}
		seen[component.Identity()] = component
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]view.Component, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func manifestComponentIdentity(component manifest.Component) string {
	if component.Package == "" {
		return component.Name
	}
	return component.Package + "." + component.Name
}
