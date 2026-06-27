// Package discover finds portable .gwdk files from source include patterns.
package discover

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Files returns files under root that match at least one include pattern and no
// exclude pattern. Patterns use slash-separated paths and support **.
func Files(root string, includes, excludes []string) ([]string, error) {
	files, _, err := FilesAndDirs(root, includes, excludes)
	return files, err
}

// FilesAndDirs returns matching files plus traversed, non-excluded directories
// under root. Directory paths are useful for polling callers that want to
// detect additions and removals without rewalking the tree on every tick.
func FilesAndDirs(root string, includes, excludes []string) ([]string, []string, error) {
	var files []string
	var dirs []string
	includeMatchers := compileGlobs(includes)
	excludeMatchers := compileGlobs(excludes)

	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A failure to read root is fatal; there is nothing to discover.
			// Errors deeper in the tree (e.g. an unreadable directory, possibly
			// inside an excluded subtree) must not abort the whole walk - skip
			// the offending entry and keep going.
			if path == root {
				return err
			}
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if entry.IsDir() {
			// Prune excluded directories so we never descend into large trees
			// like .git, node_modules, or vendor.
			if rel != "." && matchesExcludedDir(excludeMatchers, rel) {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
			return nil
		}

		if !matchesAny(includeMatchers, rel) || matchesAny(excludeMatchers, rel) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	sort.Strings(files)
	sort.Strings(dirs)
	return files, dirs, nil
}

type globPattern string

func compileGlobs(patterns []string) []globPattern {
	var matchers []globPattern
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		matchers = append(matchers, globPattern(filepath.ToSlash(pattern)))
	}
	return matchers
}

// matchesExcludedDir reports whether a directory (given by its slash-separated
// relative path) is fully covered by an exclude pattern and can be pruned. A
// directory matches either directly (a pattern naming the directory itself) or
// when a `dir/**`-style pattern covers everything beneath it, tested by
// appending a trailing slash so the pattern's `**` matches the empty remainder.
func matchesExcludedDir(matchers []globPattern, dir string) bool {
	return matchesAny(matchers, dir) || matchesAny(matchers, dir+"/")
}

func matchesAny(matchers []globPattern, value string) bool {
	for _, matcher := range matchers {
		if matcher.match(value) {
			return true
		}
	}
	return false
}

func (pattern globPattern) match(value string) bool {
	glob := string(pattern)
	memo := map[[2]int]bool{}
	var match func(int, int) bool
	match = func(patternIndex, valueIndex int) bool {
		key := [2]int{patternIndex, valueIndex}
		if result, ok := memo[key]; ok {
			return result
		}
		if patternIndex == len(glob) {
			memo[key] = valueIndex == len(value)
			return memo[key]
		}
		if glob[patternIndex] == '*' {
			if patternIndex+1 < len(glob) && glob[patternIndex+1] == '*' {
				if patternIndex+2 < len(glob) && glob[patternIndex+2] == '/' {
					if match(patternIndex+3, valueIndex) {
						memo[key] = true
						return true
					}
					for cursor := valueIndex; cursor < len(value); cursor++ {
						if value[cursor] == '/' && match(patternIndex+3, cursor+1) {
							memo[key] = true
							return true
						}
					}
					memo[key] = false
					return false
				}
				for cursor := valueIndex; cursor <= len(value); cursor++ {
					if match(patternIndex+2, cursor) {
						memo[key] = true
						return true
					}
				}
				memo[key] = false
				return false
			}
			for cursor := valueIndex; cursor <= len(value); cursor++ {
				if cursor > valueIndex && value[cursor-1] == '/' {
					break
				}
				if match(patternIndex+1, cursor) {
					memo[key] = true
					return true
				}
			}
			memo[key] = false
			return false
		}
		if valueIndex >= len(value) {
			memo[key] = false
			return false
		}
		if glob[patternIndex] == '?' {
			memo[key] = value[valueIndex] != '/' && match(patternIndex+1, valueIndex+1)
			return memo[key]
		}
		memo[key] = glob[patternIndex] == value[valueIndex] && match(patternIndex+1, valueIndex+1)
		return memo[key]
	}
	return match(0, 0)
}
