package buildgen

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func parsePathDeclarations(body string) ([]map[string]string, error) {
	return parseLiteralDeclarations(body, "paths", "path param")
}

func parsePathParams(source string) (map[string]string, error) {
	return parseLiteralStringMap(source, "path param")
}

func parseBuildData(body string, routeParams map[string]string, imports []manifest.Import, scripts []manifest.GoBlock, source string) (map[string]string, error) {
	lines := significantBuildLines(body)
	if len(lines) == 1 {
		call, ok, err := parseBuildDataCallLine(lines[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return runBuildDataCallRef(call, imports, scripts, source)
		}
	}
	data := map[string]buildValue{}
	declarations := 0
	for index, line := range lines {
		declaration, ok, err := parseBuildLiteralLine(line)
		if err != nil {
			return nil, fmt.Errorf("build line %d: %w", index+1, err)
		}
		if !ok {
			return nil, fmt.Errorf("build line %d must use `=> { name: value }` or `=> BuildData()`", index+1)
		}
		declarations++
		if len(declaration.Elts) == 0 && index == 0 {
			return nil, fmt.Errorf("build {} declaration must not be empty")
		}
		for _, element := range declaration.Elts {
			key, value, err := buildFieldValue(element, routeParams, data)
			if err != nil {
				return nil, fmt.Errorf("build line %d: %w", index+1, err)
			}
			if _, exists := data[key]; exists {
				return nil, fmt.Errorf("duplicate build field %q", key)
			}
			data[key] = value
		}
	}
	if declarations == 0 {
		return nil, nil
	}
	return buildValueStrings(data), nil
}
