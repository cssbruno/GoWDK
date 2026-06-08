package view

func ParamReferences(source string) ([]string, error) {
	nodes, err := Parse(source)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	collectParamReferences(nodes, names)
	return sortedKeys(names), nil
}
