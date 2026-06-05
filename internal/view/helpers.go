package view

import (
	"github.com/cssbruno/gowdk/internal/clientlang"
	"sort"
	"strings"
)

func keys(input map[string]string) []string {
	out := make([]string, 0, len(input))
	for key := range input {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func keysFromTypes(input map[string]clientlang.ValueType) []string {
	out := make([]string, 0, len(input))
	for key := range input {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func boolSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func stripLineComments(source string) string {
	var lines []string
	for _, rawLine := range strings.Split(source, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, rawLine)
	}
	return strings.Join(lines, "\n")
}
