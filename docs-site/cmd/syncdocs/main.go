// Command syncdocs generates the documentation site pages from the structured
// markdown docs in the main GOWDK repository. Each markdown file under the
// selected sections becomes a `.page.gwdk` page that renders through the shared
// DocsPage component, and the sidebar navigation is generated from the same
// structure so the site stays a faithful, modular view of the repo docs.
//
// Run from the docs-site root. GOWDK_SOURCE_ROOT defaults to the parent
// repository (the GOWDK monorepo root that contains this docs site):
//
//	go run ./cmd/syncdocs
package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// section is one sidebar group, sourced from a docs subdirectory. order lists
// file stems (without .md) that should lead the group; remaining files follow
// alphabetically. "README" becomes the group's index page at /docs/<dir>.
type section struct {
	Title string
	Dir   string   // relative to <repo>/docs, "" for the docs root
	Files []string // explicit file list (relative to docs); when set, Dir is ignored
	Order []string
}

var sections = []section{
	{Title: "Start", Files: []string{"getting-started.md"}},
	{Title: "Language", Dir: "language", Order: []string{
		"README", "spec", "syntax", "semantics", "grammar", "blocks", "markup",
		"components", "layouts", "data", "actions", "api", "forms", "partials",
		"ssr", "hybrid", "formatting", "diagnostics",
	}},
	{Title: "Reference", Dir: "reference", Order: []string{
		"README", "routing", "cli", "config", "css", "hooks", "addons",
		"contracts", "errors", "diagnostics", "diagnostic-codes", "dev",
		"deployment", "testing", "framework-integrations", "manifest",
	}},
	{Title: "Compiler", Dir: "compiler", Order: []string{
		"README", "project-structure", "pipeline", "generated-output",
		"browser-compiler", "build-report", "manifest",
	}},
	{Title: "Engineering", Dir: "engineering", Order: []string{
		"architecture", "security", "conventions", "naming-conventions",
		"code-quality", "generated-code-policy", "dependency-policy",
		"operations", "testing", "ci", "release",
	}},
	{Title: "Decisions", Dir: "engineering/decisions", Order: []string{"README"}},
	{Title: "Product", Dir: "product", Order: []string{
		"vision", "roadmap", "requirements", "language-server",
	}},
}

// page is one generated documentation page.
type page struct {
	Section string
	Rel     string // path relative to docs, e.g. "language/actions.md"
	Route   string // e.g. "/docs/language/actions"
	Output  string // .page.gwdk path under src/pages/docs
	PageID  string
	Title   string
	Lead    string
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func main() {
	sourceRoot := strings.TrimSpace(os.Getenv("GOWDK_SOURCE_ROOT"))
	if sourceRoot == "" {
		// The docs site lives at <repo>/docs-site, so the parent directory is
		// the GOWDK monorepo root whose docs/ tree we render.
		sourceRoot = ".."
	}
	docsRoot := filepath.Join(sourceRoot, "docs")

	pages, err := collectPages(docsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "syncdocs:", err)
		os.Exit(1)
	}

	// Replace the generated docs tree so removed source files do not linger.
	if err := os.RemoveAll(filepath.Join("src", "pages", "docs")); err != nil {
		fmt.Fprintln(os.Stderr, "syncdocs:", err)
		os.Exit(1)
	}

	routes := map[string]bool{}
	for _, p := range pages {
		routes[p.Route] = true
	}
	for _, p := range pages {
		if err := writePage(docsRoot, p, routes); err != nil {
			fmt.Fprintln(os.Stderr, "syncdocs:", p.Rel, err)
			os.Exit(1)
		}
		fmt.Println(p.Output)
	}
	if err := writeSidebar(pages); err != nil {
		fmt.Fprintln(os.Stderr, "syncdocs:", err)
		os.Exit(1)
	}
	fmt.Println("src/components/docs-sidebar.cmp.gwdk")
	fmt.Printf("generated %d pages across %d sections\n", len(pages), len(sections))
}

func collectPages(docsRoot string) ([]page, error) {
	var pages []page
	for _, sec := range sections {
		files, err := sectionFiles(docsRoot, sec)
		if err != nil {
			return nil, err
		}
		for _, rel := range files {
			full := filepath.Join(docsRoot, filepath.FromSlash(rel))
			payload, err := os.ReadFile(full)
			if err != nil {
				// A configured file may not exist in every checkout; skip it.
				continue
			}
			title, lead := frontMatter(string(payload))
			if title == "" {
				title = humanize(strings.TrimSuffix(path.Base(rel), ".md"))
			}
			route := routeFor(rel)
			pages = append(pages, page{
				Section: sec.Title,
				Rel:     rel,
				Route:   route,
				Output:  outputFor(rel),
				PageID:  pageIDFor(route),
				Title:   title,
				Lead:    lead,
			})
		}
	}
	return pages, nil
}

func sectionFiles(docsRoot string, sec section) ([]string, error) {
	if len(sec.Files) > 0 {
		return sec.Files, nil
	}
	dir := filepath.Join(docsRoot, filepath.FromSlash(sec.Dir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil // missing section directory is non-fatal
	}
	var stems []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		stems = append(stems, strings.TrimSuffix(e.Name(), ".md"))
	}
	ordered := orderStems(stems, sec.Order)
	files := make([]string, 0, len(ordered))
	for _, stem := range ordered {
		files = append(files, path.Join(sec.Dir, stem+".md"))
	}
	return files, nil
}

func orderStems(stems, order []string) []string {
	rank := map[string]int{}
	for i, name := range order {
		rank[name] = i
	}
	sort.SliceStable(stems, func(i, j int) bool {
		ri, oki := rank[stems[i]]
		rj, okj := rank[stems[j]]
		switch {
		case oki && okj:
			return ri < rj
		case oki:
			return true
		case okj:
			return false
		default:
			return stems[i] < stems[j]
		}
	})
	return stems
}

// routeFor maps a docs-relative markdown path to a site route. A README becomes
// its directory index; the top-level docs README would be /docs.
func routeFor(rel string) string {
	clean := strings.TrimSuffix(rel, ".md")
	if base := path.Base(clean); base == "README" {
		clean = path.Dir(clean)
		if clean == "." {
			return "/docs"
		}
	}
	return "/docs/" + clean
}

func outputFor(rel string) string {
	route := routeFor(rel)
	sub := strings.TrimPrefix(route, "/docs")
	sub = strings.Trim(sub, "/")
	if sub == "" {
		return filepath.Join("src", "pages", "docs", "index.page.gwdk")
	}
	return filepath.Join("src", "pages", "docs", filepath.FromSlash(sub)+".page.gwdk")
}

func pageIDFor(route string) string {
	id := strings.Trim(strings.TrimPrefix(route, "/"), "/")
	return strings.ReplaceAll(id, "/", ".")
}

// frontMatter extracts the first H1 as the title and the first paragraph after
// it as the page lead.
func frontMatter(markdown string) (title, lead string) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	i := 0
	for ; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(lines[i], "# "))
			i++
			break
		}
	}
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") ||
			strings.HasPrefix(t, "-") || strings.HasPrefix(t, "|") ||
			strings.HasPrefix(t, ">") {
			break
		}
		var para []string
		for ; i < len(lines); i++ {
			t = strings.TrimSpace(lines[i])
			if t == "" {
				break
			}
			para = append(para, t)
		}
		lead = stripInline(strings.Join(para, " "))
		break
	}
	return title, lead
}

var (
	hrefRe   = regexp.MustCompile(`href="([^"]+)"`)
	inlineRe = regexp.MustCompile("[`*_]")
	// GOWDK's view parser requires void elements to be self-closed; goldmark
	// emits some (task-list <input>, <br>, <hr>, <img>) without the slash.
	voidRe = regexp.MustCompile(`<(input|br|hr|img|col|area|base|embed|source|track|wbr)([^>]*?)\s*/?>`)
)

func selfCloseVoids(s string) string {
	return voidRe.ReplaceAllString(s, "<$1$2 />")
}

func labelTaskListCheckboxes(s string) string {
	s = strings.ReplaceAll(s,
		`<input checked="" disabled="" type="checkbox" />`,
		`<input checked="" disabled="" type="checkbox" aria-label="Completed task" />`)
	return strings.ReplaceAll(s,
		`<input disabled="" type="checkbox" />`,
		`<input disabled="" type="checkbox" aria-label="Incomplete task" />`)
}

func writePage(docsRoot string, p page, routes map[string]bool) error {
	payload, err := os.ReadFile(filepath.Join(docsRoot, filepath.FromSlash(p.Rel)))
	if err != nil {
		return err
	}
	body := stripFirstH1(string(payload))

	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return err
	}
	article := selfCloseVoids(buf.String())
	article = labelTaskListCheckboxes(article)
	article = rewriteLinks(article, p.Rel)
	article = escapeBraces(article)

	var out strings.Builder
	out.WriteString("package site\n\n")
	out.WriteString("page " + p.PageID + "\n")
	out.WriteString("route \"" + p.Route + "\"\n")
	out.WriteString("title " + quote(p.Title+" - GOWDK") + "\n")
	if p.Lead != "" {
		out.WriteString("description " + quote(truncate(p.Lead, 180)) + "\n")
	}
	out.WriteString("guard public\n")
	out.WriteString("layout root, docs\n")
	out.WriteString("css none\n\n")
	out.WriteString("view {\n  <DocsPage>\n")
	out.WriteString("    <header class=\"doc-hero\">\n")
	out.WriteString("      <p class=\"eyebrow\">" + escText(p.Section) + "</p>\n")
	out.WriteString("      <h1>" + escText(p.Title) + "</h1>\n")
	if p.Lead != "" {
		out.WriteString("      <p class=\"doc-lead\">" + escText(p.Lead) + "</p>\n")
	}
	out.WriteString("    </header>\n")
	out.WriteString("    <article class=\"prose\">\n")
	out.WriteString(article)
	out.WriteString("\n    </article>\n  </DocsPage>\n}\n")

	if err := os.MkdirAll(filepath.Dir(p.Output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p.Output, []byte(out.String()), 0o644)
}

func stripFirstH1(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return markdown
}

// rewriteLinks turns relative ".md" links into site routes, resolved against
// the current page's directory.
func rewriteLinks(htmlBody, currentRel string) string {
	dir := path.Dir(currentRel)
	return hrefRe.ReplaceAllStringFunc(htmlBody, func(match string) string {
		href := hrefRe.FindStringSubmatch(match)[1]
		if href == "" || strings.HasPrefix(href, "http://") ||
			strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "mailto:") {
			return match
		}
		anchor := ""
		if i := strings.Index(href, "#"); i >= 0 {
			anchor = href[i:]
			href = href[:i]
		}
		if !strings.HasSuffix(href, ".md") {
			return match
		}
		target := path.Clean(path.Join(dir, href))
		return `href="` + routeFor(target) + "/" + anchor + `"`
	})
}

func escapeBraces(s string) string {
	s = strings.ReplaceAll(s, "{", "&#123;")
	s = strings.ReplaceAll(s, "}", "&#125;")
	return s
}

func escText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return escapeBraces(s)
}

func stripInline(s string) string {
	return strings.TrimSpace(inlineRe.ReplaceAllString(s, ""))
}

func quote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if i := strings.LastIndex(s[:n], " "); i > 0 {
		return s[:i] + "..."
	}
	return s[:n] + "..."
}

func humanize(stem string) string {
	stem = strings.ReplaceAll(stem, "-", " ")
	stem = strings.ReplaceAll(stem, "_", " ")
	words := strings.Fields(stem)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func writeSidebar(pages []page) error {
	var out strings.Builder
	out.WriteString("package site\n\ncomponent DocsSidebar\n\n")
	out.WriteString("// Generated by cmd/syncdocs from the GOWDK repo docs structure.\n\n")
	out.WriteString("view {\n")
	out.WriteString("  <aside class=\"docs-sidebar\">\n")
	out.WriteString("    <button class=\"docs-search-btn\" type=\"button\" data-docs-search aria-label=\"Search documentation\">\n")
	out.WriteString("      <span class=\"docs-search-ico\" aria-hidden=\"true\">⌕</span>\n")
	out.WriteString("      <span class=\"docs-search-label\">Search docs</span>\n")
	out.WriteString("      <kbd class=\"docs-search-kbd\">⌘K</kbd>\n")
	out.WriteString("    </button>\n")
	out.WriteString("    <nav class=\"docs-nav\" aria-label=\"Documentation\" data-docs-nav>\n")
	out.WriteString("      <a href=\"/\">Overview</a>\n")

	for _, sec := range sections {
		var inSection []page
		for _, p := range pages {
			if p.Section == sec.Title {
				inSection = append(inSection, p)
			}
		}
		if len(inSection) == 0 {
			continue
		}
		out.WriteString("      <p class=\"docs-nav-group\">" + escText(sec.Title) + "</p>\n")
		for _, p := range inSection {
			out.WriteString("      <a href=\"" + p.Route + "/\">" + escText(p.Title) + "</a>\n")
		}
	}
	out.WriteString("    </nav>\n  </aside>\n}\n")
	return os.WriteFile(filepath.Join("src", "components", "docs-sidebar.cmp.gwdk"), []byte(out.String()), 0o644)
}
