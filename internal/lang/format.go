package lang

import (
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/parser"
	"github.com/cssbruno/gowdk/internal/viewmodel"
)

// Format normalizes whitespace for .gwdk source. When the file parses, it
// formats from the parsed structure: block kinds and boundaries come from the
// parser and view markup indentation comes from the parsed view node tree, so
// nested markup, multi-line attributes, and interpolations are indented from
// structure rather than line-by-line tag heuristics. When the file does not
// parse, it falls back to the conservative line-oriented formatter so malformed
// source is preserved instead of being rewritten destructively.
func Format(source []byte) []byte {
	formatted, _ := FormatChecked(source)
	return formatted
}

// FormatChecked formats source and reports whether the file was formatted from
// its parsed structure. ok is false when the syntax does not parse; in that case
// the returned bytes come from the conservative line-oriented fallback, which
// only normalizes whitespace and never drops user source. Callers that must not
// rewrite malformed files (for example `gowdk fmt --write`) gate on ok.
func FormatChecked(source []byte) (formatted []byte, ok bool) {
	if out, parsed := formatStructured(source); parsed {
		return out, true
	}
	return formatLegacy(source), false
}

// formatStructured formats source from the typed syntax tree. It returns
// ok=false when the source cannot be parsed so the caller can fall back without
// touching content.
func formatStructured(source []byte) ([]byte, bool) {
	file, err := parser.ParseSyntax(source)
	if err != nil {
		return nil, false
	}
	viewDepths := viewMarkupDepths(file)

	var out []string
	blankPending := false
	depth := 0
	braces := parser.NewBraceDepth()

	for index, raw := range strings.Split(string(source), "\n") {
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
		// View body lines are indented by their parsed markup depth on top of the
		// block's brace depth. The block's closing "}" is not a view body line and
		// is handled by the brace-depth path above.
		if markup, isViewBody := viewDepths[index+1]; isViewBody && !strings.HasPrefix(line, "}") {
			out = append(out, strings.Repeat("  ", indent+markup)+line)
		} else {
			out = append(out, strings.Repeat("  ", indent)+line)
		}
		depth += braces.Delta(line)
		if depth < 0 {
			depth = 0
		}
	}

	return []byte(strings.Join(out, "\n") + "\n"), true
}

// viewMarkupDepths maps a 1-based source line to its markup nesting depth for
// every view block in the file. Depths are derived from the parsed view node
// tree rather than scanning tags line by line.
func viewMarkupDepths(file gwdkast.File) map[int]int {
	depths := map[int]int{}
	for _, block := range file.Blocks {
		if block.Kind != "view" || len(block.View) == 0 {
			continue
		}
		assignViewMarkupDepths(depths, block)
	}
	return depths
}

// assignViewMarkupDepths records the markup nesting depth of each source line in
// one view block. Node offsets are rune offsets into the trimmed block body, and
// block.BodyStart.Line is the source line of the body's first rune, so a body
// line index maps directly to a source line.
func assignViewMarkupDepths(depths map[int]int, block gwdkast.Block) {
	body := []rune(block.Body)
	var newlines []int
	for offset, r := range body {
		if r == '\n' {
			newlines = append(newlines, offset)
		}
	}
	baseLine := block.BodyStart.Line

	// bodyLine returns the 0-based body line index containing the given rune
	// offset (the count of newlines before it).
	bodyLine := func(offset int) int {
		if offset < 0 {
			offset = 0
		}
		if offset > len(body) {
			offset = len(body)
		}
		return sort.SearchInts(newlines, offset)
	}
	// set records depth for a body line unless an earlier (left-most) construct
	// already claimed it, so a line is indented by its first construct.
	set := func(bodyLineIndex, depth int) {
		line := baseLine + bodyLineIndex
		if _, exists := depths[line]; !exists {
			depths[line] = depth
		}
	}
	// container assigns depths for an element or component: the open tag line at
	// depth, any multi-line attribute continuation lines one level deeper, and a
	// separate close tag line back at depth.
	container := func(start, end int, attrs []viewmodel.Attr, depth int) {
		openLine := bodyLine(start)
		set(openLine, depth)
		tagEnd := openLine
		for _, attr := range attrs {
			if line := bodyLine(maxInt(attr.Start, attr.End-1)); line > tagEnd {
				tagEnd = line
			}
		}
		for line := openLine + 1; line <= tagEnd; line++ {
			set(line, depth+1)
		}
		if closeLine := bodyLine(maxInt(start, end-1)); closeLine != openLine {
			set(closeLine, depth)
		}
	}

	var walk func(nodes []viewmodel.Node, depth int)
	walk = func(nodes []viewmodel.Node, depth int) {
		for _, node := range nodes {
			switch typed := node.(type) {
			case viewmodel.Element:
				container(typed.Start, typed.End, typed.Attrs, depth)
				walk(typed.Children, depth+1)
			case viewmodel.ComponentCall:
				container(typed.Start, typed.End, typed.Attrs, depth)
				walk(typed.Children, depth+1)
			case viewmodel.Text:
				start := bodyLine(typed.Start)
				end := bodyLine(maxInt(typed.Start, typed.End-1))
				for line := start; line <= end; line++ {
					set(line, depth)
				}
			}
		}
	}
	walk(block.View, 0)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatLegacy is the conservative line-oriented fallback used when source does
// not parse. It tracks brace depth with the parser's string/comment-aware
// scanner so braces inside string literals, comments, and template literals do
// not skew indentation (for example `title "a { b"` or `// note about }`), and
// it indents view markup with a line-by-line tag scanner. It only normalizes
// whitespace and never changes line content.
func formatLegacy(source []byte) []byte {
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
