package buildgen

import (
	"fmt"
	"strconv"
	"strings"
)

func parseLiteralDeclarations(body, blockName, itemName string) ([]map[string]string, error) {
	var declarations []map[string]string
	for index, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		literal, ok := parseLiteralDeclaration(line)
		if !ok {
			return nil, fmt.Errorf("%s line %d must use `=> { name: \"value\" }`", blockName, index+1)
		}
		params, err := parseLiteralStringMap(literal, itemName)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", blockName, index+1, err)
		}
		declarations = append(declarations, params)
	}
	return declarations, nil
}

func parseLiteralStringMap(source, itemName string) (map[string]string, error) {
	assignments, err := splitPathAssignments(source)
	if err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return nil, fmt.Errorf("literal declaration must include values")
	}

	params := map[string]string{}
	for _, assignment := range assignments {
		name, rawValue, ok := strings.Cut(assignment, ":")
		if !ok {
			return nil, fmt.Errorf("%s %q must use name: \"value\"", itemName, strings.TrimSpace(assignment))
		}
		name = strings.TrimSpace(name)
		if !isLiteralName(name) {
			return nil, fmt.Errorf("invalid %s name %q", itemName, name)
		}
		if _, exists := params[name]; exists {
			return nil, fmt.Errorf("duplicate %s %q", itemName, name)
		}
		value, err := parsePathString(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", itemName, name, err)
		}
		params[name] = value
	}
	return params, nil
}

func splitPathAssignments(source string) ([]string, error) {
	var assignments []string
	start := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case ',':
			part := strings.TrimSpace(source[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty path param assignment")
			}
			assignments = append(assignments, part)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	part := strings.TrimSpace(source[start:])
	if part != "" {
		assignments = append(assignments, part)
	}
	return assignments, nil
}

func parsePathString(source string) (string, error) {
	if !strings.HasPrefix(source, `"`) {
		return "", fmt.Errorf("value must be a string literal")
	}
	value, err := strconv.Unquote(source)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("value must not be empty")
	}
	return value, nil
}

func validateRouteParamValue(name, value string) error {
	if strings.ContainsAny(value, "/?#") {
		return fmt.Errorf("route param %q value must not contain /, ?, or #", name)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("route param %q value is unsafe", name)
	}
	return nil
}
