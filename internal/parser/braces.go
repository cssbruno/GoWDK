package parser

// braceLang selects the lexical rules used when scanning an embedded block body
// for brace depth. The supported embedded languages differ in their string and
// comment syntax, and braces inside strings or comments must be skipped so that
// valid code such as `return "}"` does not corrupt block-depth tracking.
type braceLang int

const (
	braceLangGo  braceLang = iota // " strings, ` raw strings, ' runes, // and /* */ comments
	braceLangJS                   // " ' strings, ` template literals, // and /* */ comments
	braceLangCSS                  // " ' strings, /* */ comments only
)

// braceScanner tracks brace depth across the lines of an embedded code block,
// ignoring braces that appear inside string literals or comments. It is
// stateful so multi-line constructs (block comments, Go raw strings, JS
// template literals) are handled across line boundaries.
//
// Regular-expression literals in JS are not recognized; a `/` is only treated
// as the start of a comment when immediately followed by `/` or `*`. Braces
// inside a JS regex literal can therefore still miscount, which is an accepted
// limitation versus the far larger cost of full tokenization.
type braceScanner struct {
	lang           braceLang
	inBlockComment bool // inside an open /* ... */ comment
	inRawString    bool // inside an open Go raw string or JS template literal
}

// inMultiline reports whether the scanner is currently inside a construct that
// spans line boundaries. While true, a line that is exactly "}" is body text
// (e.g. a `}` inside a block comment) rather than a block terminator.
func (s *braceScanner) inMultiline() bool {
	return s.inBlockComment || s.inRawString
}

// delta scans one line and returns the net change in brace depth it
// contributes, skipping braces inside strings and comments. Multi-line state is
// carried over to subsequent calls.
func (s *braceScanner) delta(line string) int {
	delta := 0
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if s.inBlockComment {
			if r == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				s.inBlockComment = false
				i++
			}
			continue
		}
		if s.inRawString {
			// JS template literals honor backslash escapes; Go raw strings do
			// not, but they also cannot contain a backtick, so the escape
			// branch is harmless there.
			if s.lang == braceLangJS && r == '\\' {
				i++
				continue
			}
			if r == '`' {
				s.inRawString = false
			}
			continue
		}
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		case '/':
			if s.lang != braceLangCSS && i+1 < len(runes) && runes[i+1] == '/' {
				return delta // rest of the line is a line comment
			}
			if i+1 < len(runes) && runes[i+1] == '*' {
				s.inBlockComment = true
				i++
			}
		case '"':
			i = skipQuoted(runes, i, '"')
		case '\'':
			i = skipQuoted(runes, i, '\'')
		case '`':
			if s.lang != braceLangCSS {
				s.inRawString = true // Go raw string or JS template literal
			}
		}
	}
	return delta
}

// BraceDepth tracks net brace depth across the lines of a .gwdk file for
// tooling such as the formatter, skipping braces that appear inside string
// literals, comments, Go raw strings, and JS template literals. It carries
// multi-line state across Delta calls. It uses Go lexical rules, which cover the
// top-level `.gwdk` surface and Go/JS block bodies; the one accepted edge is a
// `//` sequence inside a CSS value (e.g. a `url(http://...)`), which truncates
// brace counting for the rest of that line only.
type BraceDepth struct {
	scanner braceScanner
}

// NewBraceDepth returns a brace-depth tracker using Go lexical rules.
func NewBraceDepth() *BraceDepth {
	return &BraceDepth{scanner: braceScanner{lang: braceLangGo}}
}

// Delta scans one line and returns the net change in brace depth it
// contributes, skipping braces inside strings and comments.
func (b *BraceDepth) Delta(line string) int {
	return b.scanner.delta(line)
}

// InMultiline reports whether the tracker is currently inside a multi-line
// construct (block comment, Go raw string, or JS template literal). A line that
// is textually "}" while InMultiline is body content, not a block terminator.
func (b *BraceDepth) InMultiline() bool {
	return b.scanner.inMultiline()
}

// blockScanLang maps a top-level block kind to the lexical rules used to scan
// its body for brace depth. Kinds whose bodies are not brace-scanned default to
// Go rules, which is harmless because their scanner is never fed.
func blockScanLang(kind string) braceLang {
	switch kind {
	case "style":
		return braceLangCSS
	case "client", "js":
		return braceLangJS
	default:
		return braceLangGo
	}
}

// skipQuoted advances past a quoted run starting at runes[start] (the opening
// quote) and returns the index of the matching closing quote, honoring
// backslash escapes. Single-line quoted strings cannot span lines in any of the
// supported languages, so an unterminated quote returns the last index, ending
// the scan for this line.
func skipQuoted(runes []rune, start int, quote rune) int {
	for i := start + 1; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++ // skip the escaped rune
		case quote:
			return i
		}
	}
	return len(runes) - 1
}
