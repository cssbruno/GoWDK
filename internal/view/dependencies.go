package view

import (
	"sort"
	"strings"
)

func collectViewDependencies(nodes []Node, assets, classes, styles map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			for _, attr := range typed.Attrs {
				switch attr.Name {
				case "class":
					for _, className := range strings.Fields(attr.Value) {
						if !strings.ContainsAny(className, "{}") {
							classes[className] = true
						}
					}
				case "style":
					style := strings.TrimSpace(attr.Value)
					if style != "" && !strings.ContainsAny(style, "{}") {
						styles[style] = true
					}
				case "src", "href", "poster":
					if isSPAAssetReference(attr.Value) {
						assets[strings.TrimSpace(attr.Value)] = true
					}
				}
			}
			collectViewDependencies(typed.Children, assets, classes, styles)
		case ComponentCall:
			collectViewDependencies(typed.Children, assets, classes, styles)
		}
	}
}

func sortedKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isSPAAssetReference(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "{}") || strings.HasPrefix(value, "#") {
		return false
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"http://", "https://", "//", "mailto:", "tel:", "data:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}
