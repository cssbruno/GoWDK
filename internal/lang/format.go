package lang

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/parser"
)

// Format normalizes whitespace for top-level .gwdk metadata and blocks. Brace
// depth is tracked with the parser's string/comment-aware scanner so braces
// inside string literals, comments, and template literals do not skew
// indentation (for example `title "a { b"` or `// note about }`).
func Format(source []byte) []byte {
	var out []string
	blankPending := false
	depth := 0
	braces := parser.NewBraceDepth()

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

		// A line that is textually a closing brace dedents itself, unless we are
		// inside a multi-line literal/comment where that "}" is body content.
		inMultiline := braces.InMultiline()
		indent := depth
		if !inMultiline && strings.HasPrefix(line, "}") && indent > 0 {
			indent--
		}
		out = append(out, strings.Repeat("  ", indent)+line)
		depth += braces.Delta(line)
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
	return IsMetadataKeyword(fields[0])
}
