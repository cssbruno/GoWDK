// Command doclint checks local Markdown links and heading anchors across the
// repository without any network access. It exists so a broken in-repo doc link
// fails a targeted CI gate (see docs/engineering/ci.md) rather than rotting
// silently.
//
// It deliberately checks only local references:
//   - relative file/directory links resolve to an existing path, and
//   - "#fragment" anchors (same-file or "file.md#fragment") resolve to a
//     GitHub-style heading slug in the target Markdown file.
//
// External links (http, https, mailto, protocol-relative) are skipped so the
// check stays fast and offline. Links inside fenced or inline code are ignored
// because they are documentation examples, not live references.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root := flag.String("root", ".", "repository root to scan for Markdown files")
	exclude := flag.String("exclude", strings.Join(defaultExclusions, ","),
		"comma-separated directory names to skip (generated, vendored, or local-output paths)")
	flag.Parse()

	cfg := Config{Root: *root, ExcludedDirs: splitList(*exclude)}
	problems, err := Check(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doclint: %v\n", err)
		os.Exit(2)
	}
	if len(problems) == 0 {
		fmt.Println("docs links ok")
		return
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p.String())
	}
	fmt.Fprintf(os.Stderr, "\ndoclint: %d broken local doc link(s)\n", len(problems))
	os.Exit(1)
}

// defaultExclusions are directories that hold generated, vendored, or local
// build output. Markdown inside them is not authored docs, so it is skipped.
var defaultExclusions = []string{".git", ".gowdk", "node_modules", "vendor", "dist", "bin", "tmp"}

// Config controls a documentation link scan.
type Config struct {
	Root         string
	ExcludedDirs []string
}

// Problem is a single broken local reference.
type Problem struct {
	File   string // Markdown file containing the link, relative to root.
	Line   int    // 1-based line number of the link.
	Target string // The raw link target as written.
	Reason string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s:%d: %s -> %s", p.File, p.Line, p.Reason, p.Target)
}

// Check scans every Markdown file under cfg.Root and returns one Problem per
// broken local link, sorted by file then line for stable output.
func Check(cfg Config) ([]Problem, error) {
	root := cfg.Root
	if root == "" {
		root = "."
	}
	excluded := map[string]bool{}
	for _, name := range cfg.ExcludedDirs {
		if name != "" {
			excluded[name] = true
		}
	}

	var files []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if excluded[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)

	// Anchor sets are resolved lazily and cached: a single docs tree links into
	// the same target files repeatedly.
	anchorCache := map[string]map[string]bool{}
	anchorsFor := func(path string) (map[string]bool, error) {
		if cached, ok := anchorCache[path]; ok {
			return cached, nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			anchorCache[path] = nil
			return nil, err
		}
		set := headingAnchors(string(data))
		anchorCache[path] = set
		return set, nil
	}

	var problems []Problem
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, link := range extractLinks(string(data)) {
			rel, _ := filepath.Rel(root, file)
			if rel == "" {
				rel = file
			}
			if p, ok := checkLink(root, file, rel, link, anchorsFor); ok {
				problems = append(problems, p)
			}
		}
	}
	return problems, nil
}

// link is a Markdown link occurrence: its raw target and source line.
type link struct {
	Target string
	Line   int
}

func checkLink(root, file, rel string, l link, anchorsFor func(string) (map[string]bool, error)) (Problem, bool) {
	target := l.Target
	if target == "" || isExternal(target) {
		return Problem{}, false
	}

	pathPart, fragment := splitFragment(target)

	// Pure "#anchor" links point at a heading in the same file.
	if pathPart == "" {
		anchors, err := anchorsFor(file)
		if err != nil {
			return Problem{File: rel, Line: l.Line, Target: target, Reason: "cannot read file for anchor"}, true
		}
		if fragment != "" && !anchors[fragment] {
			return Problem{File: rel, Line: l.Line, Target: target, Reason: "missing heading anchor"}, true
		}
		return Problem{}, false
	}

	resolved := filepath.Join(filepath.Dir(file), filepath.FromSlash(pathPart))
	info, err := os.Stat(resolved)
	if err != nil {
		return Problem{File: rel, Line: l.Line, Target: target, Reason: "missing local file"}, true
	}

	// A fragment against a Markdown file must resolve to a heading anchor.
	if fragment != "" && !info.IsDir() && strings.EqualFold(filepath.Ext(resolved), ".md") {
		anchors, err := anchorsFor(resolved)
		if err != nil {
			return Problem{File: rel, Line: l.Line, Target: target, Reason: "cannot read target for anchor"}, true
		}
		if !anchors[fragment] {
			return Problem{File: rel, Line: l.Line, Target: target, Reason: "missing heading anchor"}, true
		}
	}
	return Problem{}, false
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
