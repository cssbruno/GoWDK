package viewrender

import "github.com/cssbruno/gowdk/internal/viewanalysis"

func ParamReferences(source string) ([]string, error) {
	return viewanalysis.ParamReferences(source)
}

// ParamReferencesFromNodes returns unique param("name") route-param references
// directly referenced by an already-parsed view fragment.
func ParamReferencesFromNodes(nodes []Node) []string {
	return viewanalysis.ParamReferencesFromNodes(nodes)
}
