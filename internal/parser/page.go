// Package parser turns .gwdk source files into syntax trees.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

var (
	annotationPattern     = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)\s*(.*)$`)
	blockPattern          = regexp.MustCompile(`^(paths|build|load|view)\s*\{`)
	importPattern         = regexp.MustCompile(`^import(?:\s+([A-Za-z_][A-Za-z0-9_]*))?\s+"([^"]+)"$`)
	buildCallPattern      = regexp.MustCompile(`^=>\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	actionPattern         = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*\{`)
	apiPattern            = regexp.MustCompile(`^api(?:\s+([A-Za-z_][A-Za-z0-9_.-]*))?\s*\{`)
	propPattern           = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionInputPattern    = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*form\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionValidPattern    = regexp.MustCompile(`^valid\(([A-Za-z_][A-Za-z0-9_]*)\)\?$`)
	actionRedirectPattern = regexp.MustCompile(`^->\s*"([^"]*)"$`)
	actionFragmentPattern = regexp.MustCompile(`^fragment\s+"([^"]*)"\s*\{$`)
	apiRoutePattern       = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"$`)
	routeParamPattern     = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// ParsePage extracts page metadata and top-level block declarations.
func ParsePage(source []byte) (manifest.Page, error) {
	var page manifest.Page
	var blockBody []string
	capturedBlock := ""
	var actionBody []string
	capturedAction := -1
	actionDepth := 0
	var apiBody []string
	capturedAPI := -1

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if capturedAPI >= 0 {
			if line == "}" {
				api, err := parseAPIBody(page.Blocks.APIs[capturedAPI], apiBody)
				if err != nil {
					return manifest.Page{}, fmt.Errorf("line %d: %w", lineNumber, err)
				}
				page.Blocks.APIs[capturedAPI] = api
				capturedAPI = -1
				apiBody = nil
				continue
			}
			apiBody = append(apiBody, rawLine)
			continue
		}
		if capturedAction >= 0 {
			if line == "}" {
				actionDepth--
				if actionDepth == 0 {
					action, err := parseActionBody(page.Blocks.Actions[capturedAction], actionBody)
					if err != nil {
						return manifest.Page{}, fmt.Errorf("line %d: %w", lineNumber, err)
					}
					page.Blocks.Actions[capturedAction] = action
					capturedAction = -1
					actionBody = nil
					continue
				}
				actionBody = append(actionBody, rawLine)
				continue
			}
			if actionFragmentPattern.FindStringSubmatch(line) != nil {
				actionDepth++
				actionBody = append(actionBody, rawLine)
				continue
			}
			actionBody = append(actionBody, rawLine)
			continue
		}
		if capturedBlock != "" {
			if line == "}" {
				applyBlockBody(&page, capturedBlock, blockBody)
				capturedBlock = ""
				blockBody = nil
				continue
			}
			blockBody = append(blockBody, rawLine)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Page{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyAnnotation(&page, match[1], match[2], lineNumber, rawLine); err != nil {
				return manifest.Page{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		if match := importPattern.FindStringSubmatch(line); match != nil {
			page.Imports = append(page.Imports, manifest.Import{
				Alias: match[1],
				Path:  match[2],
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedImport(line) {
			return manifest.Page{}, fmt.Errorf("line %d: malformed import %q", lineNumber, line)
		}

		if match := blockPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			applyBlock(&page, name)
			applyBlockSpan(&page.Blocks, name, lineNumber, rawLine)
			if capturesBlockBody(name) {
				capturedBlock = name
			}
			continue
		}

		if match := actionPattern.FindStringSubmatch(line); match != nil {
			page.Blocks.Actions = append(page.Blocks.Actions, manifest.Action{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			page.Blocks.Spans.Actions = append(page.Blocks.Spans.Actions, manifest.NamedSpan{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			capturedAction = len(page.Blocks.Actions) - 1
			actionDepth = 1
			continue
		}

		if match := apiPattern.FindStringSubmatch(line); match != nil {
			page.Blocks.APIs = append(page.Blocks.APIs, manifest.API{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			page.Blocks.Spans.APIs = append(page.Blocks.Spans.APIs, manifest.NamedSpan{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			capturedAPI = len(page.Blocks.APIs) - 1
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return manifest.Page{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return manifest.Page{}, err
	}
	if capturedBlock != "" {
		return manifest.Page{}, fmt.Errorf("%s block missing closing }", capturedBlock)
	}
	if capturedAction >= 0 {
		return manifest.Page{}, fmt.Errorf("act %s block missing closing }", page.Blocks.Actions[capturedAction].Name)
	}
	if capturedAPI >= 0 {
		return manifest.Page{}, fmt.Errorf("api %s block missing closing }", page.Blocks.APIs[capturedAPI].Name)
	}

	if page.ID == "" {
		return manifest.Page{}, fmt.Errorf("missing @page")
	}
	if page.Route == "" {
		return manifest.Page{}, fmt.Errorf("%s missing @route", page.ID)
	}
	return page, nil
}

func parseActionBody(action manifest.Action, body []string) (manifest.Action, error) {
	action.Body = strings.TrimSpace(strings.Join(body, "\n"))
	baseLine := action.Span.Start.Line + 1
	for index := 0; index < len(body); index++ {
		rawLine := body[index]
		line := strings.TrimSpace(rawLine)
		span := sourceLineSpan(baseLine+index, rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := actionFragmentPattern.FindStringSubmatch(line); match != nil {
			fragment, nextIndex, err := parseActionFragment(action.Name, body, index, match[1], baseLine)
			if err != nil {
				return manifest.Action{}, err
			}
			action.Fragments = append(action.Fragments, fragment)
			index = nextIndex
			continue
		}
		if match := actionInputPattern.FindStringSubmatch(line); match != nil {
			if action.InputName != "" {
				return manifest.Action{}, fmt.Errorf("action %s line %d declares multiple form inputs", action.Name, index+1)
			}
			action.InputName = match[1]
			action.InputType = match[2]
			action.InputSpan = span
			continue
		}
		if match := actionValidPattern.FindStringSubmatch(line); match != nil {
			if action.InputName == "" {
				return manifest.Action{}, fmt.Errorf("action %s line %d validates before declaring form input", action.Name, index+1)
			}
			if match[1] != action.InputName {
				return manifest.Action{}, fmt.Errorf("action %s line %d validates %q but input is %q", action.Name, index+1, match[1], action.InputName)
			}
			action.ValidatesInput = true
			action.ValidationSpan = span
			continue
		}
		if match := actionRedirectPattern.FindStringSubmatch(line); match != nil {
			if action.Redirect != "" {
				return manifest.Action{}, fmt.Errorf("action %s line %d declares multiple redirects", action.Name, index+1)
			}
			redirect := match[1]
			if err := validateActionRedirect(redirect); err != nil {
				return manifest.Action{}, fmt.Errorf("action %s line %d: %w", action.Name, index+1, err)
			}
			action.Redirect = redirect
			action.RedirectSpan = span
			continue
		}
		return manifest.Action{}, fmt.Errorf("action %s line %d has unsupported syntax %q", action.Name, index+1, line)
	}
	return action, nil
}

func parseActionFragment(actionName string, body []string, start int, target string, baseLine int) (manifest.Fragment, int, error) {
	if err := validateFragmentTarget(target); err != nil {
		return manifest.Fragment{}, start, fmt.Errorf("action %s line %d: %w", actionName, start+1, err)
	}
	var fragmentBody []string
	for index := start + 1; index < len(body); index++ {
		line := strings.TrimSpace(body[index])
		if line == "}" {
			return manifest.Fragment{
				Target: target,
				Body:   strings.TrimSpace(strings.Join(fragmentBody, "\n")),
				Span:   sourceLineSpan(baseLine+start, body[start]),
			}, index, nil
		}
		fragmentBody = append(fragmentBody, body[index])
	}
	return manifest.Fragment{}, start, fmt.Errorf("action %s line %d fragment %q missing closing }", actionName, start+1, target)
}

func parseAPIBody(api manifest.API, body []string) (manifest.API, error) {
	baseLine := api.Span.Start.Line + 1
	for index, rawLine := range body {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := apiRoutePattern.FindStringSubmatch(line); match != nil {
			if api.Method != "" || api.Route != "" {
				return manifest.API{}, fmt.Errorf("api %s line %d declares multiple routes", api.Name, index+1)
			}
			api.Method = match[1]
			api.Route = match[2]
			api.RouteSpan = sourceLineSpan(baseLine+index, rawLine)
			api.RouteParams = routeParamSpans(api.Route, baseLine+index, rawLine)
			continue
		}
		return manifest.API{}, fmt.Errorf("api %s line %d has unsupported syntax %q", api.Name, index+1, line)
	}
	return api, nil
}

func validateActionRedirect(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("redirect %q must be a local absolute path", value)
	}
	if strings.HasPrefix(value, "//") {
		return fmt.Errorf("redirect %q must not be protocol-relative", value)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("redirect %q must not contain newlines", value)
	}
	return nil
}

func validateFragmentTarget(value string) error {
	if value == "" {
		return fmt.Errorf("fragment target is required")
	}
	if strings.ContainsAny(value, "\r\n\t ") {
		return fmt.Errorf("fragment target %q must not contain whitespace", value)
	}
	if !strings.HasPrefix(value, "#") || strings.TrimPrefix(value, "#") == "" {
		return fmt.Errorf("fragment target %q must be a static id selector", value)
	}
	if strings.ContainsAny(value, "{}") {
		return fmt.Errorf("fragment target %q must be static", value)
	}
	return nil
}

// ParseComponent extracts component metadata and top-level block declarations.
func ParseComponent(source []byte) (manifest.Component, error) {
	var component manifest.Component
	var viewBody []string
	inView := false
	inProps := false

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inView {
			if line == "}" {
				component.Blocks.ViewBody = strings.TrimSpace(strings.Join(viewBody, "\n"))
				inView = false
				viewBody = nil
				continue
			}
			viewBody = append(viewBody, rawLine)
			continue
		}
		if inProps {
			if line == "}" {
				inProps = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			match := propPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: invalid prop declaration %q", lineNumber, line)
			}
			if match[2] != "string" {
				return manifest.Component{}, fmt.Errorf("line %d: prop %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Props = append(component.Props, manifest.Prop{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyComponentAnnotation(&component, match[1], match[2], lineNumber, rawLine); err != nil {
				return manifest.Component{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		switch line {
		case "props {":
			inProps = true
			continue
		case "view {":
			component.Blocks.View = true
			component.Blocks.Spans.View = sourceLineSpan(lineNumber, rawLine)
			inView = true
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return manifest.Component{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return manifest.Component{}, err
	}
	if inView {
		return manifest.Component{}, fmt.Errorf("view block missing closing }")
	}
	if inProps {
		return manifest.Component{}, fmt.Errorf("props block missing closing }")
	}
	if component.Name == "" {
		return manifest.Component{}, fmt.Errorf("missing @component")
	}
	return component, nil
}

// ParseLayout extracts layout metadata and top-level block declarations.
func ParseLayout(source []byte) (manifest.Layout, error) {
	var layout manifest.Layout
	var viewBody []string
	inView := false

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inView {
			if line == "}" {
				layout.Blocks.View = true
				layout.Blocks.ViewBody = strings.TrimSpace(strings.Join(viewBody, "\n"))
				inView = false
				viewBody = nil
				continue
			}
			viewBody = append(viewBody, rawLine)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Layout{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyLayoutAnnotation(&layout, match[1], match[2], lineNumber, rawLine); err != nil {
				return manifest.Layout{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		switch line {
		case "view {":
			layout.Blocks.Spans.View = sourceLineSpan(lineNumber, rawLine)
			inView = true
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return manifest.Layout{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return manifest.Layout{}, err
	}
	if inView {
		return manifest.Layout{}, fmt.Errorf("view block missing closing }")
	}
	if layout.ID == "" {
		return manifest.Layout{}, fmt.Errorf("missing @layout")
	}
	return layout, nil
}

func applyAnnotation(page *manifest.Page, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	span := sourceLineSpan(lineNumber, rawLine)
	switch name {
	case "page":
		if value == "" {
			return fmt.Errorf("@page requires a value")
		}
		page.ID = value
		page.Spans.Page = span
	case "route":
		if value == "" {
			return fmt.Errorf("@route requires a value")
		}
		page.Route = trimQuotes(value)
		page.Spans.Route = span
		page.Spans.RouteParams = routeParamSpans(page.Route, lineNumber, rawLine)
	case "layout":
		if value == "" {
			return fmt.Errorf("@layout requires a value")
		}
		page.Layouts = splitList(value)
		page.Spans.Layouts = namedValueSpans(page.Layouts, lineNumber, rawLine)
	case "render":
		mode, err := gowdk.ParseRenderMode(value)
		if err != nil {
			return err
		}
		page.Render = mode
		page.Spans.Render = span
	case "guard":
		if value == "" {
			return fmt.Errorf("@guard requires a value")
		}
		page.Guard = splitList(value)
		page.Spans.Guard = namedValueSpans(page.Guard, lineNumber, rawLine)
	case "css":
		if value == "" {
			return fmt.Errorf("@css requires a value")
		}
		page.CSS = splitCSSList(value)
		page.Spans.CSS = namedValueSpans(page.CSS, lineNumber, rawLine)
	default:
		return fmt.Errorf("unsupported annotation @%s", name)
	}
	return nil
}

func applyLayoutAnnotation(layout *manifest.Layout, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "layout":
		if value == "" {
			return fmt.Errorf("@layout requires a value")
		}
		layout.ID = trimQuotes(value)
		layout.Span = sourceLineSpan(lineNumber, rawLine)
	default:
		return fmt.Errorf("unsupported annotation @%s", name)
	}
	return nil
}

func applyComponentAnnotation(component *manifest.Component, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "component":
		if value == "" {
			return fmt.Errorf("@component requires a value")
		}
		component.Name = value
		component.Span = sourceLineSpan(lineNumber, rawLine)
	default:
		return fmt.Errorf("unsupported annotation @%s", name)
	}
	return nil
}

func applyBlock(page *manifest.Page, name string) {
	switch name {
	case "paths":
		page.Paths = true
	case "build":
		page.Blocks.Build = true
	case "load":
		page.Blocks.Load = true
	case "view":
		page.Blocks.View = true
	}
}

func applyBlockSpan(blocks *manifest.Blocks, name string, lineNumber int, rawLine string) {
	span := sourceLineSpan(lineNumber, rawLine)
	switch name {
	case "paths":
		blocks.Spans.Paths = span
	case "build":
		blocks.Spans.Build = span
	case "load":
		blocks.Spans.Load = span
	case "view":
		blocks.Spans.View = span
	}
}

func capturesBlockBody(name string) bool {
	return name == "paths" || name == "build" || name == "load" || name == "view"
}

func applyBlockBody(page *manifest.Page, name string, body []string) {
	text := strings.TrimSpace(strings.Join(body, "\n"))
	switch name {
	case "paths":
		page.Blocks.PathsBody = text
	case "build":
		page.Blocks.BuildBody = text
	case "load":
		page.Blocks.LoadBody = text
	case "view":
		page.Blocks.ViewBody = text
	}
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSSList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func sourceLineSpan(lineNumber int, rawLine string) manifest.SourceSpan {
	startColumn := 1
	for _, r := range rawLine {
		if r != ' ' && r != '\t' {
			break
		}
		startColumn++
	}
	endColumn := len([]rune(rawLine)) + 1
	if endColumn <= startColumn {
		endColumn = startColumn + 1
	}
	return manifest.SourceSpan{
		Start: manifest.SourcePosition{Line: lineNumber, Column: startColumn},
		End:   manifest.SourcePosition{Line: lineNumber, Column: endColumn},
	}
}

func namedValueSpans(values []string, lineNumber int, rawLine string) []manifest.NamedSpan {
	if len(values) == 0 {
		return nil
	}
	spans := make([]manifest.NamedSpan, 0, len(values))
	searchStart := 0
	for _, value := range values {
		if value == "" {
			continue
		}
		index := strings.Index(rawLine[searchStart:], value)
		if index < 0 {
			spans = append(spans, manifest.NamedSpan{Name: value, Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		start := searchStart + index
		end := start + len([]rune(value))
		spans = append(spans, manifest.NamedSpan{
			Name: value,
			Span: manifest.SourceSpan{
				Start: manifest.SourcePosition{Line: lineNumber, Column: start + 1},
				End:   manifest.SourcePosition{Line: lineNumber, Column: end + 1},
			},
		})
		searchStart = end
	}
	return spans
}

func routeParamSpans(route string, lineNumber int, rawLine string) []manifest.NamedSpan {
	matches := routeParamPattern.FindAllStringSubmatchIndex(route, -1)
	if len(matches) == 0 {
		return nil
	}
	routeStart := strings.Index(rawLine, route)
	if routeStart < 0 {
		routeStart = 0
	}
	spans := make([]manifest.NamedSpan, 0, len(matches))
	for _, match := range matches {
		name := route[match[2]:match[3]]
		start := routeStart + match[0]
		end := routeStart + match[1]
		spans = append(spans, manifest.NamedSpan{
			Name: name,
			Span: manifest.SourceSpan{
				Start: manifest.SourcePosition{Line: lineNumber, Column: start + 1},
				End:   manifest.SourcePosition{Line: lineNumber, Column: end + 1},
			},
		})
	}
	return spans
}

func trimQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func unsupportedTopLevelBlockName(line string) string {
	if !strings.HasSuffix(line, "{") {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	name := fields[0]
	if !isBlockName(name) {
		return ""
	}
	return name
}

func isMalformedImport(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "import"
}

func isBlockName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !isIdentStart(r) {
				return false
			}
			continue
		}
		if !isBlockNamePart(r) {
			return false
		}
	}
	return true
}

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isBlockNamePart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9') || r == '.' || r == '-'
}
