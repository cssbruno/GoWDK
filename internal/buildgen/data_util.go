package buildgen

import (
	"fmt"
	"path/filepath"
)

func mergeBuildData(buildData, routeData map[string]string) (map[string]string, error) {
	merged := cloneStringMap(buildData)
	for key, value := range routeData {
		if _, exists := merged[key]; exists {
			return nil, fmt.Errorf("build data field %q conflicts with route param", key)
		}
		merged[key] = value
	}
	return merged, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func sourcePathSet(paths []string) map[string]bool {
	set := map[string]bool{}
	for _, sourcePath := range paths {
		abs, err := filepath.Abs(sourcePath)
		if err != nil {
			continue
		}
		set[filepath.Clean(abs)] = true
	}
	return set
}

func sourcePathChanged(set map[string]bool, sourcePath string) bool {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return false
	}
	return set[filepath.Clean(abs)]
}
