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
	inView := false
	viewDepth := 0
	markupDepth := 0

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
		if inView && !strings.HasPrefix(line, "}") {
			delta, leadingClosers := markupIndentDelta(line)
			markupIndent := markupDepth - leadingClosers
			if markupIndent < 0 {
				markupIndent = 0
			}
			out = append(out, strings.Repeat("  ", indent+markupIndent)+line)
			markupDepth += delta
			if markupDepth < 0 {
				markupDepth = 0
			}
		} else {
			out = append(out, strings.Repeat("  ", indent)+line)
		}
		depth += braces.Delta(line)
		if depth < 0 {
			depth = 0
		}
		if !inView && opensViewBlock(line) {
			inView = true
			viewDepth = depth
			markupDepth = 0
		} else if inView && depth < viewDepth {
			inView = false
			viewDepth = 0
			markupDepth = 0
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

func opensViewBlock(line string) bool {
	return strings.TrimSpace(line) == "view {"
}

func markupIndentDelta(line string) (delta int, leadingClosers int) {
	firstTag := true
	for cursor := 0; cursor < len(line); {
		tag, next, ok := nextMarkupTag(line, cursor)
		if !ok {
			break
		}
		cursor = next
		if tag.comment {
			continue
		}
		switch {
		case tag.closing:
			delta--
			if firstTag {
				leadingClosers++
			}
		case !tag.selfClosing && !isVoidElement(tag.name):
			delta++
			firstTag = false
		default:
			firstTag = false
		}
	}
	return delta, leadingClosers
}

type markupTag struct {
	name        string
	closing     bool
	selfClosing bool
	comment     bool
}

func nextMarkupTag(line string, start int) (markupTag, int, bool) {
	for cursor := start; cursor < len(line); cursor++ {
		if line[cursor] != '<' {
			continue
		}
		if strings.HasPrefix(line[cursor:], "<!--") {
			if end := strings.Index(line[cursor+4:], "-->"); end >= 0 {
				return markupTag{comment: true}, cursor + 4 + end + 3, true
			}
			return markupTag{comment: true}, len(line), true
		}
		tagStart := cursor + 1
		closing := false
		if tagStart < len(line) && line[tagStart] == '/' {
			closing = true
			tagStart++
		}
		if tagStart >= len(line) || !isTagNameStart(line[tagStart]) {
			continue
		}
		end := markupTagEnd(line, tagStart)
		if end < 0 {
			return markupTag{}, len(line), false
		}
		body := strings.TrimSpace(line[tagStart:end])
		if body == "" {
			return markupTag{}, end + 1, false
		}
		nameEnd := 0
		for nameEnd < len(body) && isTagNamePart(body[nameEnd]) {
			nameEnd++
		}
		if nameEnd == 0 {
			return markupTag{}, end + 1, false
		}
		return markupTag{
			name:        body[:nameEnd],
			closing:     closing,
			selfClosing: strings.HasSuffix(strings.TrimSpace(body), "/"),
		}, end + 1, true
	}
	return markupTag{}, len(line), false
}

func markupTagEnd(line string, start int) int {
	var quote byte
	exprDepth := 0
	for cursor := start; cursor < len(line); cursor++ {
		ch := line[cursor]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '{':
			exprDepth++
		case '}':
			if exprDepth > 0 {
				exprDepth--
			}
		case '>':
			if exprDepth == 0 {
				return cursor
			}
		}
	}
	return -1
}

func isTagNameStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isTagNamePart(ch byte) bool {
	return isTagNameStart(ch) || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == ':'
}

func isVoidElement(name string) bool {
	switch strings.ToLower(name) {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}
