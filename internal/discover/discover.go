// Package discover finds portable .gwdk files from source include patterns.
package discover

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Files returns files under root that match at least one include pattern and no
// exclude pattern. Patterns use slash-separated paths and support **.
func Files(root string, includes, excludes []string) ([]string, error) {
	var files []string
	includeMatchers, err := compileGlobs(includes)
	if err != nil {
		return nil, err
	}
	excludeMatchers, err := compileGlobs(excludes)
	if err != nil {
		return nil, err
	}

	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !matchesAny(includeMatchers, rel) || matchesAny(excludeMatchers, rel) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	var matchers []*regexp.Regexp
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		matcher, err := regexp.Compile(globToRegex(filepath.ToSlash(pattern)))
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, matcher)
	}
	return matchers, nil
}

func matchesAny(matchers []*regexp.Regexp, value string) bool {
	for _, matcher := range matchers {
		if matcher.MatchString(value) {
			return true
		}
	}
	return false
}

func globToRegex(pattern string) string {
	var out strings.Builder
	out.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					out.WriteString("(?:.*/)?")
				} else {
					out.WriteString(".*")
				}
			} else {
				out.WriteString("[^/]*")
			}
		case '?':
			out.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '{', '}', '[', ']', '^', '$', '\\':
			out.WriteByte('\\')
			out.WriteByte(pattern[i])
		default:
			out.WriteByte(pattern[i])
		}
	}
	out.WriteString("$")
	return out.String()
}
