package viewanalysis

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// ParamReferences returns unique param("name") route-param references directly
// visible in the current view markup subset.
func ParamReferences(source string) ([]string, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return nil, err
	}
	return ParamReferencesFromNodes(nodes), nil
}

// ParamReferencesFromNodes returns unique param("name") route-param references
// directly referenced by an already-parsed view fragment.
func ParamReferencesFromNodes(nodes []viewmodel.Node) []string {
	names := map[string]bool{}
	collectParamReferences(nodes, names)
	return sortedKeys(names)
}

func collectParamReferences(nodes []viewmodel.Node, names map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Text:
			collectParamReferencesFromString(typed.Value, names)
		case viewmodel.Element:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		case viewmodel.ComponentCall:
			for _, attr := range typed.Attrs {
				collectParamReferencesFromString(attr.Value, names)
			}
			collectParamReferences(typed.Children, names)
		}
	}
}

func collectParamReferencesFromString(value string, names map[string]bool) {
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			return
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if name, ok := routeParamExpression(expr); ok {
			names[name] = true
		}
		value = value[end+1:]
	}
}

func routeParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !isIdentifier(name) {
		return "", false
	}
	return name, true
}
