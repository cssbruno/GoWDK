package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

var cssKeyframesPattern = regexp.MustCompile(`(?i)@(-[a-z]+-)?keyframes\s+([_a-zA-Z][-_a-zA-Z0-9]*)`)
var cssAnimationDeclarationPattern = regexp.MustCompile(`(?i)(animation(?:-name)?\s*:\s*)([^;}]*)`)

type cssPlan struct {
	assets          []plannedCSSArtifact
	stylesheets     []gowdk.Stylesheet
	pageStylesheets map[string][]gowdk.Stylesheet
}

type cssInput struct {
	name     string
	path     string
	contents []byte
}

func planCSS(config gowdk.Config, app manifest.Manifest, outputDir string) (cssPlan, []string) {
	planned := cssPlan{pageStylesheets: map[string][]gowdk.Stylesheet{}}
	var failures []string
	seen := map[string]bool{}
	pageIDs := pageIDSet(app.Pages)
	context := gowdk.CSSContext{
		Sources:   cssSources(app),
		OutputDir: outputDir,
		Build:     config.Build,
		CSS:       config.CSS,
	}
	for _, addon := range config.Addons {
		processor, ok := addon.(gowdk.CSSProcessor)
		if !ok {
			continue
		}
		result, err := processor.ProcessCSS(context)
		if err != nil {
			failures = append(failures, fmt.Sprintf("css processor %s failed: %v", processor.Name(), err))
			continue
		}
		planned.stylesheets = append(planned.stylesheets, nonEmptyStylesheets(result.Stylesheets)...)
		for pageID, stylesheets := range result.PageStylesheets {
			if !pageIDs[pageID] {
				failures = append(failures, fmt.Sprintf("css processor %s selected unknown page %q", processor.Name(), pageID))
				continue
			}
			planned.pageStylesheets[pageID] = append(planned.pageStylesheets[pageID], nonEmptyStylesheets(stylesheets)...)
		}
		for _, asset := range result.Assets {
			outputPath, err := cssOutputPath(outputDir, asset.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("css processor %s: %v", processor.Name(), err))
				continue
			}
			if seen[outputPath] {
				failures = append(failures, fmt.Sprintf("css processor %s: duplicate css asset path %q", processor.Name(), asset.Path))
				continue
			}
			seen[outputPath] = true
			logicalPath, err := relativeOutputPath(outputDir, outputPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("css processor %s: %v", processor.Name(), err))
				continue
			}
			planned.assets = append(planned.assets, plannedCSSArtifact{
				CSSArtifact: CSSArtifact{Path: outputPath, LogicalPath: logicalPath},
				contents:    append([]byte(nil), asset.Contents...),
			})
		}
	}
	inputs, inputFailures := discoverCSSInputs(config, outputDir)
	failures = append(failures, inputFailures...)
	if len(inputFailures) == 0 {
		pageCSS, pageStylesheets, pageFailures := planPageCSS(config, app.Pages, outputDir, inputs, seen)
		failures = append(failures, pageFailures...)
		planned.assets = append(planned.assets, pageCSS...)
		for pageID, stylesheets := range pageStylesheets {
			planned.pageStylesheets[pageID] = append(planned.pageStylesheets[pageID], stylesheets...)
		}
	}
	layoutCSS, layoutStylesheets, layoutFailures := planLayoutStyleCSS(app.Pages, app.Layouts, outputDir, seen)
	failures = append(failures, layoutFailures...)
	planned.assets = append(planned.assets, layoutCSS...)
	for pageID, stylesheets := range layoutStylesheets {
		planned.pageStylesheets[pageID] = append(planned.pageStylesheets[pageID], stylesheets...)
	}
	componentCSS, componentStylesheets, componentFailures := planComponentCSS(app.Components, outputDir, seen)
	failures = append(failures, componentFailures...)
	planned.assets = append(planned.assets, componentCSS...)
	planned.stylesheets = append(planned.stylesheets, componentStylesheets...)
	failures = append(failures, finalizeCSSPlan(outputDir, &planned)...)
	return planned, failures
}

func pageIDSet(pages []manifest.Page) map[string]bool {
	out := map[string]bool{}
	for _, page := range pages {
		out[page.ID] = true
	}
	return out
}

func discoverCSSInputs(config gowdk.Config, outputDir string) (map[string]cssInput, []string) {
	includes := appendNonEmpty(nil, config.CSS.Include)
	if len(includes) == 1 && includes[0] == DisableCSSDiscovery {
		return map[string]cssInput{}, nil
	}
	if len(includes) == 0 {
		includes = append([]string{}, defaultCSSIncludes...)
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, []string{err.Error()}
	}
	excludes := append([]string{}, defaultCSSExcludes...)
	excludes = appendNonEmpty(excludes, config.CSS.Exclude)
	if pattern := cssOutputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}

	paths, err := discover.Files(root, includes, excludes)
	if err != nil {
		return nil, []string{fmt.Sprintf("css discovery failed: %v", err)}
	}

	inputs := map[string]cssInput{}
	var failures []string
	for _, filePath := range paths {
		name := cssInputName(filePath)
		if !cssInputNamePattern.MatchString(name) {
			failures = append(failures, fmt.Sprintf("css file %q exports invalid name %q", filePath, name))
			continue
		}
		if previous, exists := inputs[name]; exists {
			failures = append(failures, fmt.Sprintf("duplicate css export %q from %s and %s", name, previous.path, filePath))
			continue
		}
		contents, err := os.ReadFile(filePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("read css file %q: %v", filePath, err))
			continue
		}
		inputs[name] = cssInput{name: name, path: filePath, contents: contents}
	}
	return inputs, failures
}

func cssInputName(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func planPageCSS(config gowdk.Config, pages []manifest.Page, outputDir string, inputs map[string]cssInput, seenAssets map[string]bool) ([]plannedCSSArtifact, map[string][]gowdk.Stylesheet, []string) {
	var assets []plannedCSSArtifact
	stylesheets := map[string][]gowdk.Stylesheet{}
	var failures []string
	for _, page := range pages {
		names, err := pageCSSInputNames(config, page, inputs)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		inlineStyle := strings.TrimSpace(page.Blocks.StyleBody)
		if len(names) == 0 && inlineStyle == "" {
			continue
		}
		assetPath, err := pageCSSOutputPath(config.CSS.Output, outputDir, page.ID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
			continue
		}
		if seenAssets[assetPath] {
			failures = append(failures, fmt.Sprintf("%s: duplicate css asset path %q", page.ID, assetPath))
			continue
		}
		seenAssets[assetPath] = true
		logicalPath, err := relativeOutputPath(outputDir, assetPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
			continue
		}
		logicalHref := pageCSSHref(config.CSS.Output, page.ID)
		assets = append(assets, plannedCSSArtifact{
			CSSArtifact: CSSArtifact{Path: assetPath, LogicalPath: logicalPath, LogicalHref: logicalHref},
			contents:    pageCSSContents(names, inputs, inlineStyle),
		})
		stylesheets[page.ID] = []gowdk.Stylesheet{{Href: logicalHref}}
	}
	return assets, stylesheets, failures
}

func planLayoutStyleCSS(pages []manifest.Page, layouts []manifest.Layout, outputDir string, seenAssets map[string]bool) ([]plannedCSSArtifact, map[string][]gowdk.Stylesheet, []string) {
	var assets []plannedCSSArtifact
	stylesheets := map[string][]gowdk.Stylesheet{}
	var failures []string
	layoutRegistry := map[string]manifest.Layout{}
	for _, layout := range layouts {
		layoutRegistry[layoutRegistryKey(layout.Package, layout.ID)] = layout
	}
	layoutHrefs := map[string]gowdk.Stylesheet{}
	for _, layout := range layouts {
		style := strings.TrimSpace(layout.Blocks.StyleBody)
		if style == "" {
			continue
		}
		hashKey := cssscope.HashKey("layout", layout.Package, layout.ID, layout.Source, inlineStyleAssetPath)
		scopeID := cssscope.ScopeID(hashKey)
		logicalPath := layoutStyleLogicalPath(layout, scopeID)
		outputPath, err := cssOutputPath(outputDir, logicalPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("layout %s style: %v", layout.ID, err))
			continue
		}
		if seenAssets[outputPath] {
			failures = append(failures, fmt.Sprintf("layout %s style: duplicate css asset path %q", layout.ID, outputPath))
			continue
		}
		seenAssets[outputPath] = true
		logicalHref := "/" + strings.TrimLeft(filepath.ToSlash(logicalPath), "/")
		assets = append(assets, plannedCSSArtifact{
			CSSArtifact: CSSArtifact{Path: outputPath, LogicalPath: logicalPath, LogicalHref: logicalHref},
			contents:    []byte(style),
		})
		layoutHrefs[layoutRegistryKey(layout.Package, layout.ID)] = gowdk.Stylesheet{Href: logicalHref}
	}
	for _, page := range pages {
		for _, layoutRef := range page.Layouts {
			layout, ok := resolvePageLayout(page, layoutRegistry, layoutRef)
			if !ok {
				continue
			}
			stylesheet, ok := layoutHrefs[layoutRegistryKey(layout.Package, layout.ID)]
			if !ok {
				continue
			}
			stylesheets[page.ID] = append(stylesheets[page.ID], stylesheet)
		}
	}
	return assets, stylesheets, failures
}

func planComponentCSS(components []manifest.Component, outputDir string, seenAssets map[string]bool) ([]plannedCSSArtifact, []gowdk.Stylesheet, []string) {
	var assets []plannedCSSArtifact
	var stylesheets []gowdk.Stylesheet
	var failures []string
	for _, component := range components {
		for _, cssPath := range component.CSS {
			sourcePath, err := componentCSSSourcePath(component.Source, cssPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("component %s css %q: %v", component.Name, cssPath, err))
				continue
			}
			contents, err := os.ReadFile(sourcePath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("component %s css %q: %v", component.Name, cssPath, err))
				continue
			}
			hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, cssPath)
			scopeID := cssscope.ScopeID(hashKey)
			logicalPath := componentCSSLogicalPath(component, scopeID)
			outputPath, err := cssOutputPath(outputDir, logicalPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("component %s css %q: %v", component.Name, cssPath, err))
				continue
			}
			if seenAssets[outputPath] {
				failures = append(failures, fmt.Sprintf("component %s css %q: duplicate css asset path %q", component.Name, cssPath, outputPath))
				continue
			}
			seenAssets[outputPath] = true
			logicalHref := "/" + strings.TrimLeft(filepath.ToSlash(logicalPath), "/")
			assets = append(assets, plannedCSSArtifact{
				CSSArtifact: CSSArtifact{Path: outputPath, LogicalPath: logicalPath, LogicalHref: logicalHref},
				contents:    scopeComponentCSS(contents, scopeID),
			})
			stylesheets = append(stylesheets, gowdk.Stylesheet{Href: logicalHref})
		}
		style := strings.TrimSpace(component.Blocks.StyleBody)
		if style == "" {
			continue
		}
		hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, inlineStyleAssetPath)
		scopeID := cssscope.ScopeID(hashKey)
		logicalPath := componentCSSLogicalPath(component, scopeID)
		outputPath, err := cssOutputPath(outputDir, logicalPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s style: %v", component.Name, err))
			continue
		}
		if seenAssets[outputPath] {
			failures = append(failures, fmt.Sprintf("component %s style: duplicate css asset path %q", component.Name, outputPath))
			continue
		}
		seenAssets[outputPath] = true
		logicalHref := "/" + strings.TrimLeft(filepath.ToSlash(logicalPath), "/")
		assets = append(assets, plannedCSSArtifact{
			CSSArtifact: CSSArtifact{Path: outputPath, LogicalPath: logicalPath, LogicalHref: logicalHref},
			contents:    scopeComponentCSS([]byte(style), scopeID),
		})
		stylesheets = append(stylesheets, gowdk.Stylesheet{Href: logicalHref})
	}
	return assets, stylesheets, failures
}

func componentCSSSourcePath(componentSource string, cssPath string) (string, error) {
	if strings.TrimSpace(cssPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(cssPath) {
		return "", fmt.Errorf("path must be relative")
	}
	baseDir := "."
	if strings.TrimSpace(componentSource) != "" {
		baseDir = filepath.Dir(filepath.FromSlash(componentSource))
	}
	return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(cssPath))), nil
}

func componentCSSLogicalPath(component manifest.Component, scopeID string) string {
	packagePart := safeCSSPathPart(component.Package)
	if packagePart == "" {
		packagePart = "_"
	}
	componentPart := safeCSSPathPart(componentAssetName(component.Name))
	if componentPart == "" {
		componentPart = "component"
	}
	return path.Join(defaultPageCSSDir, "components", packagePart, componentPart, scopeID+".css")
}

func layoutStyleLogicalPath(layout manifest.Layout, scopeID string) string {
	packagePart := safeCSSPathPart(layout.Package)
	if packagePart == "" {
		packagePart = "_"
	}
	layoutPart := safeCSSPathPart(layout.ID)
	if layoutPart == "" {
		layoutPart = "layout"
	}
	return path.Join(defaultPageCSSDir, "layouts", packagePart, layoutPart, scopeID+".css")
}

func safeCSSPathPart(value string) string {
	value = strings.TrimSpace(filepath.ToSlash(value))
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ".", "_", " ", "_", ":", "_")
	return replacer.Replace(value)
}

func finalizeCSSPlan(outputDir string, planned *cssPlan) []string {
	var failures []string
	hrefs := map[string]string{}
	for index := range planned.assets {
		artifact := &planned.assets[index]
		logicalPath := artifact.LogicalPath
		if strings.TrimSpace(logicalPath) == "" {
			rel, err := relativeOutputPath(outputDir, artifact.Path)
			if err != nil {
				failures = append(failures, err.Error())
				continue
			}
			logicalPath = rel
		}
		contents := minifyCSS(artifact.contents)
		hash := contentHash(contents)
		emittedPath := hashedCSSPath(logicalPath, hash)
		outputPath, err := cssOutputPath(outputDir, emittedPath)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		artifact.contents = contents
		artifact.Path = outputPath
		artifact.LogicalPath = logicalPath
		artifact.Hash = hash
		artifact.CachePolicy = immutableAssetCachePolicy

		hrefs["/"+strings.TrimLeft(logicalPath, "/")] = "/" + emittedPath
		hrefs[strings.TrimLeft(logicalPath, "/")] = emittedPath
		if artifact.LogicalHref != "" {
			hrefs[artifact.LogicalHref] = hashedStylesheetHref(artifact.LogicalHref, emittedPath)
		}
	}
	planned.stylesheets = rewriteStylesheets(planned.stylesheets, hrefs)
	for pageID, stylesheets := range planned.pageStylesheets {
		planned.pageStylesheets[pageID] = rewriteStylesheets(stylesheets, hrefs)
	}
	return failures
}

func scopeComponentCSS(contents []byte, scopeID string) []byte {
	if strings.TrimSpace(scopeID) == "" {
		return append([]byte(nil), contents...)
	}
	css := rewriteCSSKeyframes(string(contents), scopeID)
	return []byte(scopeCSSRules(css, componentCSSScopeSelector(scopeID)))
}

func rewriteCSSKeyframes(contents string, scopeID string) string {
	renames := map[string]string{}
	for _, match := range cssKeyframesPattern.FindAllStringSubmatch(contents, -1) {
		if len(match) < 3 || strings.TrimSpace(match[2]) == "" {
			continue
		}
		renames[match[2]] = match[2] + "-" + scopeID
	}
	if len(renames) == 0 {
		return contents
	}
	contents = cssKeyframesPattern.ReplaceAllStringFunc(contents, func(match string) string {
		parts := cssKeyframesPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return strings.TrimSuffix(match, parts[2]) + renames[parts[2]]
	})
	return cssAnimationDeclarationPattern.ReplaceAllStringFunc(contents, func(match string) string {
		parts := cssAnimationDeclarationPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		value := parts[2]
		for name, scoped := range renames {
			value = regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).ReplaceAllString(value, scoped)
		}
		return parts[1] + value
	})
}

func componentCSSScopeSelector(scopeID string) string {
	return `:where([data-gowdk-scope~="` + scopeID + `"])`
}

func scopeCSSRules(contents string, scopeSelector string) string {
	var out strings.Builder
	for cursor := 0; cursor < len(contents); {
		open := strings.IndexByte(contents[cursor:], '{')
		if open < 0 {
			out.WriteString(contents[cursor:])
			break
		}
		open += cursor
		close := matchingCSSBrace(contents, open)
		if close < 0 {
			out.WriteString(contents[cursor:])
			break
		}
		prefix := contents[cursor:open]
		body := contents[open+1 : close]
		selector := strings.TrimSpace(prefix)
		switch {
		case selector == "":
			out.WriteString(prefix)
			out.WriteByte('{')
			out.WriteString(body)
			out.WriteByte('}')
		case strings.HasPrefix(selector, "@"):
			out.WriteString(prefix)
			out.WriteByte('{')
			if cssAtRuleHasNestedRules(selector) {
				out.WriteString(scopeCSSRules(body, scopeSelector))
			} else {
				out.WriteString(body)
			}
			out.WriteByte('}')
		default:
			leading := prefix[:len(prefix)-len(strings.TrimLeft(prefix, " \n\r\t\f"))]
			trailing := prefix[len(strings.TrimRight(prefix, " \n\r\t\f")):]
			out.WriteString(leading)
			out.WriteString(scopeCSSSelectorList(selector, scopeSelector))
			out.WriteString(trailing)
			out.WriteByte('{')
			out.WriteString(body)
			out.WriteByte('}')
		}
		cursor = close + 1
	}
	return out.String()
}

func cssAtRuleHasNestedRules(selector string) bool {
	lower := strings.ToLower(strings.TrimSpace(selector))
	if strings.Contains(lower, "keyframes") {
		return false
	}
	return strings.HasPrefix(lower, "@media") || strings.HasPrefix(lower, "@supports") || strings.HasPrefix(lower, "@container") || strings.HasPrefix(lower, "@layer")
}

func matchingCSSBrace(contents string, open int) int {
	depth := 0
	inString := rune(0)
	escaped := false
	for index, current := range contents[open:] {
		absolute := open + index
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == inString {
				inString = 0
			}
			continue
		}
		if current == '"' || current == '\'' {
			inString = current
			continue
		}
		switch current {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return absolute
			}
		}
	}
	return -1
}

func scopeCSSSelectorList(selectorList string, scopeSelector string) string {
	selectors := splitCSSSelectorList(selectorList)
	for index, selector := range selectors {
		selectors[index] = scopeCSSSelector(strings.TrimSpace(selector), scopeSelector)
	}
	return strings.Join(selectors, ", ")
}

func splitCSSSelectorList(selectorList string) []string {
	var selectors []string
	start := 0
	parenDepth := 0
	bracketDepth := 0
	for index, current := range selectorList {
		switch current {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if parenDepth == 0 && bracketDepth == 0 {
				selectors = append(selectors, selectorList[start:index])
				start = index + 1
			}
		}
	}
	selectors = append(selectors, selectorList[start:])
	return selectors
}

func scopeCSSSelector(selector string, scopeSelector string) string {
	if selector == "" || strings.Contains(selector, ":global(") {
		return selector
	}
	if index := strings.LastIndex(selector, "::"); index >= 0 {
		return strings.TrimSpace(selector[:index]) + scopeSelector + selector[index:]
	}
	return selector + scopeSelector
}

func hashedCSSPath(logicalPath string, hash string) string {
	clean := strings.TrimLeft(filepath.ToSlash(logicalPath), "/")
	digest := strings.TrimPrefix(hash, "sha256:")
	if len(digest) > 12 {
		digest = digest[:12]
	}
	ext := path.Ext(clean)
	base := strings.TrimSuffix(clean, ext)
	if ext == "" {
		return base + "." + digest
	}
	return base + "." + digest + ext
}

func hashedStylesheetHref(logicalHref string, emittedPath string) string {
	if strings.Contains(logicalHref, "://") || strings.HasPrefix(logicalHref, "//") {
		return logicalHref
	}
	prefix := path.Dir(strings.TrimSpace(logicalHref))
	file := path.Base(emittedPath)
	if prefix == "." || prefix == "/" {
		return "/" + file
	}
	if strings.HasPrefix(logicalHref, "/") {
		return path.Join("/", strings.Trim(prefix, "/"), file)
	}
	return path.Join(prefix, file)
}

func rewriteStylesheets(stylesheets []gowdk.Stylesheet, hrefs map[string]string) []gowdk.Stylesheet {
	out := make([]gowdk.Stylesheet, 0, len(stylesheets))
	for _, stylesheet := range stylesheets {
		if rewritten, ok := hrefs[stylesheet.Href]; ok {
			stylesheet.Href = rewritten
		}
		out = append(out, stylesheet)
	}
	return out
}

func minifyCSS(contents []byte) []byte {
	var builder strings.Builder
	inString := rune(0)
	escaped := false
	pendingSpace := false
	last := rune(0)
	runes := []rune(string(contents))
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if inString != 0 {
			builder.WriteRune(current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == inString {
				inString = 0
			}
			last = current
			continue
		}
		if current == '/' && index+1 < len(runes) && runes[index+1] == '*' {
			index++
			for index+1 < len(runes) && !(runes[index] == '*' && runes[index+1] == '/') {
				index++
			}
			if index+1 < len(runes) {
				index++
			}
			continue
		}
		if current == '"' || current == '\'' {
			if pendingSpace && cssNeedsSpaceBefore(last, current) {
				builder.WriteByte(' ')
			}
			pendingSpace = false
			builder.WriteRune(current)
			inString = current
			last = current
			continue
		}
		if isCSSWhitespace(current) {
			pendingSpace = true
			continue
		}
		if isCSSPunctuation(current) {
			trimTrailingSpace(&builder)
			pendingSpace = false
			builder.WriteRune(current)
			last = current
			continue
		}
		if pendingSpace && cssNeedsSpaceBefore(last, current) {
			builder.WriteByte(' ')
		}
		pendingSpace = false
		builder.WriteRune(current)
		last = current
	}
	return []byte(strings.TrimSpace(builder.String()))
}

func isCSSWhitespace(value rune) bool {
	return value == ' ' || value == '\n' || value == '\r' || value == '\t' || value == '\f'
}

func isCSSPunctuation(value rune) bool {
	switch value {
	case '{', '}', ':', ';', ',', '>', '+', '~', '(', ')':
		return true
	default:
		return false
	}
}

func cssNeedsSpaceBefore(previous rune, current rune) bool {
	if previous == ')' && !isCSSPunctuation(current) {
		return true
	}
	if previous == 0 || isCSSPunctuation(previous) {
		return false
	}
	return !isCSSPunctuation(current)
}

func trimTrailingSpace(builder *strings.Builder) {
	value := builder.String()
	trimmed := strings.TrimRight(value, " \n\r\t\f")
	if len(trimmed) == len(value) {
		return
	}
	builder.Reset()
	builder.WriteString(trimmed)
}

func pageCSSInputNames(config gowdk.Config, page manifest.Page, inputs map[string]cssInput) ([]string, error) {
	references := page.CSS
	if len(references) == 0 {
		references = []string{"default", "page"}
	}
	if len(references) == 1 && references[0] == "none" {
		return nil, nil
	}

	var names []string
	seen := map[string]bool{}
	add := func(name string) error {
		if seen[name] {
			return nil
		}
		if _, ok := inputs[name]; !ok {
			return fmt.Errorf("%s: unknown css input %q", page.ID, name)
		}
		seen[name] = true
		names = append(names, name)
		return nil
	}

	for _, reference := range references {
		switch reference {
		case "default":
			defaults, err := defaultCSSInputs(config, inputs)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", page.ID, err)
			}
			for _, name := range defaults {
				if err := add(name); err != nil {
					return nil, err
				}
			}
		case "page":
			if _, ok := inputs[page.ID]; ok {
				if err := add(page.ID); err != nil {
					return nil, err
				}
			}
		case "none":
			return nil, fmt.Errorf("%s: @css none must be used by itself", page.ID)
		default:
			if err := add(reference); err != nil {
				return nil, err
			}
		}
	}
	return names, nil
}

func defaultCSSInputs(config gowdk.Config, inputs map[string]cssInput) ([]string, error) {
	if len(config.CSS.Default) > 0 {
		for _, name := range config.CSS.Default {
			if _, ok := inputs[name]; !ok {
				return nil, fmt.Errorf("unknown default css input %q", name)
			}
		}
		return append([]string{}, config.CSS.Default...), nil
	}
	if _, ok := inputs["global"]; ok {
		return []string{"global"}, nil
	}
	return nil, nil
}

func pageCSSContents(names []string, inputs map[string]cssInput, inlineStyle string) []byte {
	var builder strings.Builder
	for _, name := range names {
		input := inputs[name]
		builder.WriteString("/* gowdk css: ")
		builder.WriteString(name)
		builder.WriteString(" */\n")
		builder.Write(input.contents)
		if len(input.contents) == 0 || input.contents[len(input.contents)-1] != '\n' {
			builder.WriteString("\n")
		}
	}
	if strings.TrimSpace(inlineStyle) != "" {
		builder.WriteString("/* gowdk inline style */\n")
		builder.WriteString(strings.TrimSpace(inlineStyle))
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func pageCSSOutputPath(output gowdk.CSSOutputConfig, outputDir string, pageID string) (string, error) {
	assetPath := path.Join(cssOutputDir(output), pageID+".css")
	return cssOutputPath(outputDir, assetPath)
}

func pageCSSHref(output gowdk.CSSOutputConfig, pageID string) string {
	prefix := strings.TrimSpace(output.HrefPrefix)
	if prefix == "" {
		prefix = "/" + cssOutputDir(output)
	}
	return path.Join("/", strings.Trim(prefix, "/"), pageID+".css")
}

func cssOutputDir(output gowdk.CSSOutputConfig) string {
	dir := strings.Trim(strings.TrimSpace(output.Dir), "/")
	if dir == "" {
		return defaultPageCSSDir
	}
	return path.Clean(filepath.ToSlash(dir))
}

func cssOutputExcludePattern(root string, outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		return ""
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	absoluteOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(absoluteRoot, absoluteOutput)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel) + "/**"
}

func appendNonEmpty(values []string, patterns []string) []string {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		values = append(values, pattern)
	}
	return values
}

func cssSources(app manifest.Manifest) []gowdk.CSSSource {
	sources := make([]gowdk.CSSSource, 0, len(app.Pages)+len(app.Components))
	for _, page := range app.Pages {
		sources = append(sources, gowdk.CSSSource{
			Path:       page.Source,
			Kind:       "page",
			Name:       page.ID,
			CSSClasses: cssClassesFromViewBody(page.Blocks.ViewBody),
		})
	}
	for _, component := range app.Components {
		sources = append(sources, gowdk.CSSSource{
			Path:       component.Source,
			Kind:       "component",
			Name:       component.Name,
			CSSClasses: cssClassesFromViewBody(component.Blocks.ViewBody),
		})
	}
	return sources
}

func cssClassesFromViewBody(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	dependencies, err := view.ViewDependencies(body)
	if err != nil {
		return nil
	}
	return dependencies.CSSClasses
}

func nonEmptyStylesheets(stylesheets []gowdk.Stylesheet) []gowdk.Stylesheet {
	out := make([]gowdk.Stylesheet, 0, len(stylesheets))
	for _, stylesheet := range stylesheets {
		if strings.TrimSpace(stylesheet.Href) == "" {
			continue
		}
		out = append(out, stylesheet)
	}
	return out
}

func cssOutputPath(outputDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("css asset path is required")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("css asset path %q must be relative", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("css asset path %q must stay inside output directory", name)
	}
	return filepath.Join(outputDir, clean), nil
}
