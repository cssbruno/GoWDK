package lang

import (
	"bufio"
	"bytes"
	"strings"
)

// Format normalizes whitespace for top-level .gwdk annotations and blocks.
func Format(source []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	var out []string
	blankPending := false
	depth := 0

	for scanner.Scan() {
		raw := strings.TrimRight(scanner.Text(), " \t")
		line := strings.TrimSpace(raw)
		if line == "" {
			blankPending = true
			continue
		}

		if len(out) > 0 && blankPending && shouldKeepBlank(out[len(out)-1], line) {
			out = append(out, "")
		}
		blankPending = false

		if strings.HasPrefix(line, "}") && depth > 0 {
			depth--
		}
		out = append(out, strings.Repeat("  ", depth)+line)
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth < 0 {
			depth = 0
		}
	}

	return []byte(strings.Join(out, "\n") + "\n")
}

func shouldKeepBlank(previous, next string) bool {
	if strings.HasPrefix(previous, "@") && strings.HasPrefix(next, "@") {
		return false
	}
	return true
}
