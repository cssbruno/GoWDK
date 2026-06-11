package lang

import (
	"strings"
)

// Format normalizes whitespace for top-level .gwdk metadata and blocks.
func Format(source []byte) []byte {
	var out []string
	blankPending := false
	depth := 0

	for _, raw := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			blankPending = true
			continue
		}

		if len(out) > 0 && blankPending && shouldKeepBlank(out[len(out)-1], line) {
			out = append(out, "")
		}
		blankPending = false

		indent := depth
		if strings.HasPrefix(line, "}") && indent > 0 {
			indent--
		}
		out = append(out, strings.Repeat("  ", indent)+line)
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth < 0 {
			depth = 0
		}
	}

	return []byte(strings.Join(out, "\n") + "\n")
}

func shouldKeepBlank(previous, next string) bool {
	if isTopLevelMetadataLine(previous) && isTopLevelMetadataLine(next) {
		return false
	}
	return true
}

func isTopLevelMetadataLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "page", "route", "title", "description", "canonical", "image", "layout", "cache", "revalidate", "error", "guard", "css", "component", "wasm", "asset", "plugin":
		return true
	default:
		return false
	}
}
