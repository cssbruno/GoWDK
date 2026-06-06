package buildgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
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
		if len(names) == 0 {
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
			contents:    pageCSSContents(names, inputs),
		})
		stylesheets[page.ID] = []gowdk.Stylesheet{{Href: logicalHref}}
	}
	return assets, stylesheets, failures
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

func pageCSSContents(names []string, inputs map[string]cssInput) []byte {
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
