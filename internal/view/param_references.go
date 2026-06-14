package view

func ParamReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	return ParamReferencesFromNodes(nodes), nil
}

// ParamReferencesFromNodes returns unique param("name") route-param references
// directly referenced by an already-parsed view fragment.
func ParamReferencesFromNodes(nodes []Node) []string {
	names := map[string]bool{}
	collectParamReferences(nodes, names)
	return sortedKeys(names)
}
