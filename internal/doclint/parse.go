package main

import (
	"strings"
)

// isExternal reports whether a link target points outside the repository and so
// should not be resolved on disk. Network schemes are skipped to keep the check
// offline.
func isExternal(target string) bool {
	if strings.HasPrefix(target, "//") {
		return true
	}
	for _, scheme := range []string{"http:", "https:", "mailto:", "tel:", "ftp:", "data:"} {
		if strings.HasPrefix(strings.ToLower(target), scheme) {
			return true
		}
	}
	return false
}

// splitFragment separates "path#fragment" into its path and (decoded) fragment.
func splitFragment(target string) (path, fragment string) {
	if i := strings.IndexByte(target, '#'); i >= 0 {
		return target[:i], target[i+1:]
	}
	return target, ""
}

// extractLinks returns every Markdown link target in src, skipping links inside
// fenced code blocks and inline code spans (documentation examples, not live
// references). It recognizes inline links "[text](target)" and reference
// definitions "[id]: target".
func extractLinks(src string) []link {
	var links []link
	inFence := false
	var fenceMarker string

	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(raw)

		// Toggle fenced code blocks on ``` or ~~~ runs.
		if marker := fenceOpener(trimmed); marker != "" {
			if !inFence {
				inFence, fenceMarker = true, marker
			} else if strings.HasPrefix(trimmed, fenceMarker) {
				inFence, fenceMarker = false, ""
			}
			continue
		}
		if inFence {
			continue
		}

		scrubbed := stripInlineCode(raw)

		// Reference-style definition: "[id]: target [optional title]".
		if def, ok := referenceDefinition(scrubbed); ok {
			links = append(links, link{Target: def, Line: lineNo})
			continue
		}

		for _, target := range inlineLinkTargets(scrubbed) {
			links = append(links, link{Target: target, Line: lineNo})
		}
	}
	return links
}

func fenceOpener(trimmed string) string {
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return "```"
	case strings.HasPrefix(trimmed, "~~~"):
		return "~~~"
	default:
		return ""
	}
}

// stripInlineCode blanks out backtick-delimited spans so links written as
// examples inside `code` are not treated as live references. Backtick runs of
// matching length delimit a span (CommonMark rule); an unterminated run is left
// as-is.
func stripInlineCode(line string) string {
	var b strings.Builder
	runes := []rune(line)
	for i := 0; i < len(runes); {
		if runes[i] != '`' {
			b.WriteRune(runes[i])
			i++
			continue
		}
		// Measure the opening backtick run.
		start := i
		for i < len(runes) && runes[i] == '`' {
			i++
		}
		runLen := i - start
		// Find a closing run of the same length.
		closeStart := -1
		for j := i; j < len(runes); {
			if runes[j] != '`' {
				j++
				continue
			}
			cs := j
			for j < len(runes) && runes[j] == '`' {
				j++
			}
			if j-cs == runLen {
				closeStart = cs
				i = j
				break
			}
		}
		if closeStart == -1 {
			// Unterminated: emit the backticks literally and stop scrubbing.
			b.WriteString(strings.Repeat("`", runLen))
			b.WriteString(string(runes[i:]))
			return b.String()
		}
		// Replace the whole span (including delimiters) with spaces.
		b.WriteString(strings.Repeat(" ", i-start))
	}
	return b.String()
}

// inlineLinkTargets extracts the URL portion of every "[text](url)" link on a
// line, ignoring images' leading "!" (images are checked identically). Nested
// brackets in link text are handled by balancing.
func inlineLinkTargets(line string) []string {
	var targets []string
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '[' {
			continue
		}
		// Balance the link-text brackets.
		depth, j := 1, i+1
		for ; j < len(runes) && depth > 0; j++ {
			switch runes[j] {
			case '[':
				depth++
			case ']':
				depth--
			}
		}
		if depth != 0 || j >= len(runes) || runes[j] != '(' {
			continue
		}
		// Collect the parenthesized destination.
		k := j + 1
		pdepth := 1
		var dest strings.Builder
		for ; k < len(runes) && pdepth > 0; k++ {
			switch runes[k] {
			case '(':
				pdepth++
				dest.WriteRune(runes[k])
			case ')':
				pdepth--
				if pdepth > 0 {
					dest.WriteRune(runes[k])
				}
			default:
				dest.WriteRune(runes[k])
			}
		}
		if pdepth != 0 {
			continue
		}
		if target := cleanDestination(dest.String()); target != "" {
			targets = append(targets, target)
		}
		i = k - 1
	}
	return targets
}

// cleanDestination normalizes a raw "(...)" destination: trims whitespace,
// unwraps <...>, and drops an optional "title".
func cleanDestination(raw string) string {
	dest := strings.TrimSpace(raw)
	if dest == "" {
		return ""
	}
	if strings.HasPrefix(dest, "<") {
		if end := strings.IndexByte(dest, '>'); end >= 0 {
			return dest[1:end]
		}
	}
	// Destination ends at the first unescaped whitespace; the rest is a title.
	if idx := strings.IndexAny(dest, " \t"); idx >= 0 {
		dest = dest[:idx]
	}
	return dest
}

// referenceDefinition matches a link reference definition line, returning its
// destination. The definition form is "[label]: destination [optional title]".
func referenceDefinition(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	close := strings.Index(trimmed, "]:")
	if close < 1 {
		return "", false
	}
	rest := strings.TrimSpace(trimmed[close+2:])
	if rest == "" {
		return "", false
	}
	return cleanDestination(rest), true
}

// headingAnchors returns the set of GitHub-style slugs for every ATX heading in
// src, applying GitHub's duplicate-suffix rule ("-1", "-2", ...). Headings
// inside fenced code blocks are ignored.
func headingAnchors(src string) map[string]bool {
	anchors := map[string]bool{}
	counts := map[string]int{}
	inFence := false
	var fenceMarker string

	for _, raw := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(raw)
		if marker := fenceOpener(trimmed); marker != "" {
			if !inFence {
				inFence, fenceMarker = true, marker
			} else if strings.HasPrefix(trimmed, fenceMarker) {
				inFence, fenceMarker = false, ""
			}
			continue
		}
		if inFence {
			continue
		}
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
			continue
		}
		text := strings.TrimSpace(trimmed[level:])
		slug := slugify(text)
		if slug == "" {
			continue
		}
		if n := counts[slug]; n > 0 {
			anchors[slug+"-"+itoa(n)] = true
			counts[slug] = n + 1
		} else {
			anchors[slug] = true
			counts[slug] = 1
		}
	}
	return anchors
}

// slugify converts heading text to a GitHub-style anchor slug: lowercased, with
// punctuation other than hyphens and underscores removed and spaces converted
// to hyphens.
func slugify(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		case r >= 0x80:
			// Keep non-ASCII letters; GitHub lowercases and retains them.
			b.WriteRune(r)
		}
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
