package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

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
		if !isCSSInputName(name) {
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
	contents = rewriteCSSKeyframeDeclarations(contents, scopeID, renames)
	if len(renames) == 0 {
		return contents
	}
	return rewriteCSSAnimationDeclarations(contents, renames)
}

func rewriteCSSKeyframeDeclarations(contents string, scopeID string, renames map[string]string) string {
	var builder strings.Builder
	cursor := 0
	for cursor < len(contents) {
		at := strings.IndexByte(contents[cursor:], '@')
		if at < 0 {
			builder.WriteString(contents[cursor:])
			break
		}
		at += cursor
		nameStart, nameEnd, ok := cssKeyframeNameRange(contents, at)
		if !ok {
			builder.WriteString(contents[cursor : at+1])
			cursor = at + 1
			continue
		}
		name := contents[nameStart:nameEnd]
		scoped := name + "-" + scopeID
		renames[name] = scoped
		builder.WriteString(contents[cursor:nameStart])
		builder.WriteString(scoped)
		cursor = nameEnd
	}
	return builder.String()
}

func cssKeyframeNameRange(contents string, at int) (int, int, bool) {
	cursor := at + 1
	if cursor < len(contents) && contents[cursor] == '-' {
		cursor++
		for cursor < len(contents) && isCSSLetter(contents[cursor]) {
			cursor++
		}
		if cursor >= len(contents) || contents[cursor] != '-' {
			return 0, 0, false
		}
		cursor++
	}
	if !hasCSSWordAt(contents, cursor, "keyframes") {
		return 0, 0, false
	}
	cursor += len("keyframes")
	if cursor >= len(contents) || !isCSSSpace(contents[cursor]) {
		return 0, 0, false
	}
	for cursor < len(contents) && isCSSSpace(contents[cursor]) {
		cursor++
	}
	if cursor >= len(contents) || !isCSSNameStart(contents[cursor]) {
		return 0, 0, false
	}
	start := cursor
	cursor++
	for cursor < len(contents) && isCSSNamePart(contents[cursor]) {
		cursor++
	}
	return start, cursor, true
}

func rewriteCSSAnimationDeclarations(contents string, renames map[string]string) string {
	var builder strings.Builder
	cursor := 0
	for cursor < len(contents) {
		colon := strings.IndexByte(contents[cursor:], ':')
		if colon < 0 {
			builder.WriteString(contents[cursor:])
			break
		}
		colon += cursor
		propStart := cssDeclarationPropertyStart(contents, colon)
		property := strings.TrimSpace(contents[propStart:colon])
		if !isCSSAnimationProperty(property) {
			builder.WriteString(contents[cursor : colon+1])
			cursor = colon + 1
			continue
		}
		valueEnd := cssDeclarationValueEnd(contents, colon+1)
		value := replaceCSSAnimationNames(contents[colon+1:valueEnd], renames)
		builder.WriteString(contents[cursor : colon+1])
		builder.WriteString(value)
		cursor = valueEnd
	}
	return builder.String()
}

func cssDeclarationPropertyStart(contents string, colon int) int {
	start := colon
	for start > 0 {
		switch contents[start-1] {
		case '{', '}', ';':
			return start
		default:
			start--
		}
	}
	return start
}

func cssDeclarationValueEnd(contents string, start int) int {
	for cursor := start; cursor < len(contents); cursor++ {
		switch contents[cursor] {
		case ';', '}':
			return cursor
		case '\'', '"':
			cursor = cssStringEnd(contents, cursor)
		}
	}
	return len(contents)
}

func replaceCSSAnimationNames(value string, renames map[string]string) string {
	var builder strings.Builder
	for cursor := 0; cursor < len(value); {
		if !isCSSNameStart(value[cursor]) {
			builder.WriteByte(value[cursor])
			cursor++
			continue
		}
		start := cursor
		cursor++
		for cursor < len(value) && isCSSNamePart(value[cursor]) {
			cursor++
		}
		token := value[start:cursor]
		if scoped, ok := renames[token]; ok {
			builder.WriteString(scoped)
		} else {
			builder.WriteString(token)
		}
	}
	return builder.String()
}

func isCSSAnimationProperty(property string) bool {
	return strings.EqualFold(property, "animation") || strings.EqualFold(property, "animation-name")
}

func hasCSSWordAt(contents string, start int, word string) bool {
	if start+len(word) > len(contents) || !strings.EqualFold(contents[start:start+len(word)], word) {
		return false
	}
	return start+len(word) == len(contents) || !isCSSNamePart(contents[start+len(word)])
}

func isCSSNameStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isCSSNamePart(char byte) bool {
	return isCSSNameStart(char) || char == '-' || (char >= '0' && char <= '9')
}

func isCSSLetter(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isCSSSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func cssStringEnd(contents string, quote int) int {
	cursor := quote + 1
	for cursor < len(contents) {
		if contents[cursor] == '\\' {
			cursor += 2
			continue
		}
		if contents[cursor] == contents[quote] {
			return cursor
		}
		cursor++
	}
	return len(contents) - 1
}

func componentCSSScopeSelector(scopeID string) string {
	return `:where([data-gowdk-scope~="` + scopeID + `"])`
}

func scopeCSSRules(contents string, scopeSelector string) string {
	parts := make([]string, 0, 8)
	for cursor := 0; cursor < len(contents); {
		open := strings.IndexByte(contents[cursor:], '{')
		if open < 0 {
			parts = append(parts, contents[cursor:])
			break
		}
		open += cursor
		close := matchingCSSBrace(contents, open)
		if close < 0 {
			parts = append(parts, contents[cursor:])
			break
		}
		prefix := contents[cursor:open]
		body := contents[open+1 : close]
		selector := strings.TrimSpace(prefix)
		switch {
		case selector == "":
			parts = append(parts, prefix, "{", body, "}")
		case strings.HasPrefix(selector, "@"):
			parts = append(parts, prefix, "{")
			if cssAtRuleHasNestedRules(selector) {
				parts = append(parts, scopeCSSRules(body, scopeSelector))
			} else {
				parts = append(parts, body)
			}
			parts = append(parts, "}")
		default:
			leading := prefix[:len(prefix)-len(strings.TrimLeft(prefix, " \n\r\t\f"))]
			trailing := prefix[len(strings.TrimRight(prefix, " \n\r\t\f")):]
			parts = append(parts, leading, scopeCSSSelectorList(selector, scopeSelector), trailing, "{", body, "}")
		}
		cursor = close + 1
	}
	return strings.Join(parts, "")
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
	out := make([]rune, 0, len(contents))
	inString := rune(0)
	escaped := false
	pendingSpace := false
	last := rune(0)
	runes := []rune(string(contents))
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if inString != 0 {
			out = append(out, current)
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
				out = append(out, ' ')
			}
			pendingSpace = false
			out = append(out, current)
			inString = current
			last = current
			continue
		}
		if isCSSWhitespace(current) {
			pendingSpace = true
			continue
		}
		if isCSSPunctuation(current) {
			out = trimTrailingCSSSpace(out)
			pendingSpace = false
			out = append(out, current)
			last = current
			continue
		}
		if pendingSpace && cssNeedsSpaceBefore(last, current) {
			out = append(out, ' ')
		}
		pendingSpace = false
		out = append(out, current)
		last = current
	}
	return []byte(strings.TrimSpace(string(out)))
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

func trimTrailingCSSSpace(values []rune) []rune {
	for len(values) > 0 && isCSSWhitespace(values[len(values)-1]) {
		values = values[:len(values)-1]
	}
	return values
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
	var out []byte
	for _, name := range names {
		input := inputs[name]
		out = append(out, "/* gowdk css: "...)
		out = append(out, name...)
		out = append(out, " */\n"...)
		out = append(out, input.contents...)
		if len(input.contents) == 0 || input.contents[len(input.contents)-1] != '\n' {
			out = append(out, '\n')
		}
	}
	if strings.TrimSpace(inlineStyle) != "" {
		out = append(out, "/* gowdk inline style */\n"...)
		out = append(out, strings.TrimSpace(inlineStyle)...)
		out = append(out, '\n')
	}
	return out
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
