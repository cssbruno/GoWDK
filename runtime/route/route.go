package route

import (
	"path"
	"strings"
)

// Match compares a concrete request path to a simple generated route pattern.
// Parameter segments use "{name}" and reject empty, ".", and ".." values.
func Match(pattern, requestPath string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	requestParts := splitPath(requestPath)
	if len(patternParts) != len(requestParts) {
		return nil, false
	}

	params := map[string]string{}
	for index, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			value := requestParts[index]
			if value == "" || value == "." || value == ".." {
				return nil, false
			}
			params[name] = value
			continue
		}
		if part != requestParts[index] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(value string) []string {
	clean := path.Clean("/" + value)
	trimmed := strings.Trim(clean, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
