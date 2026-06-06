// Package parser turns .gwdk source files into syntax trees.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

var (
	annotationPattern       = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)\s*(.*)$`)
	blockPattern            = regexp.MustCompile(`^(paths|build|load|view)\s*\{`)
	packagePattern          = regexp.MustCompile(`^package\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	importPattern           = regexp.MustCompile(`^import(?:\s+([A-Za-z_][A-Za-z0-9_]*))?\s+"([^"]+)"$`)
	usePattern              = regexp.MustCompile(`^use\s+([A-Za-z_][A-Za-z0-9_]*)\s+"([A-Za-z_][A-Za-z0-9_]*)"$`)
	buildCallPattern        = regexp.MustCompile(`^=>\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	actionEndpointPattern   = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Z]+)\s+"([^"]*)"$`)
	apiEndpointPattern      = regexp.MustCompile(`^api\s+([A-Za-z_][A-Za-z0-9_]*)\s+(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"$`)
	fragmentEndpointPattern = regexp.MustCompile(`^fragment\s+([A-Za-z_][A-Za-z0-9_]*)\s+(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"\s+"([^"]*)"\s*\{$`)
	actionPattern           = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*\{`)
	apiPattern              = regexp.MustCompile(`^api(?:\s+([A-Za-z_][A-Za-z0-9_.-]*))?\s*\{`)
	propPattern             = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	emitPattern             = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)$`)
	identifierPattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	componentTypePattern    = regexp.MustCompile(`^(props|state)\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)(?:\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\))?$`)
	storePattern            = regexp.MustCompile(`^store\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	actionInputPattern      = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*form\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionValidPattern      = regexp.MustCompile(`^valid\(([A-Za-z_][A-Za-z0-9_]*)\)\?$`)
	actionRedirectPattern   = regexp.MustCompile(`^->\s*"([^"]*)"$`)
	actionFragmentPattern   = regexp.MustCompile(`^fragment\s+"([^"]*)"\s*\{$`)
	apiRoutePattern         = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"$`)
	routeParamPattern       = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::([A-Za-z_][A-Za-z0-9_]*))?\}`)
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
	var fragmentBody []string
	capturedFragment := -1
	seenDeclaration := false

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if capturedFragment >= 0 {
			if line == "}" {
				page.Blocks.Fragments[capturedFragment].Body = strings.TrimSpace(strings.Join(fragmentBody, "\n"))
				capturedFragment = -1
				fragmentBody = nil
				continue
			}
			fragmentBody = append(fragmentBody, rawLine)
			continue
		}
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

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return manifest.Page{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			page.Package = match[1]
			page.Spans.Package = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return manifest.Page{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

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
		if match := usePattern.FindStringSubmatch(line); match != nil {
			page.Uses = append(page.Uses, manifest.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return manifest.Page{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
		}

		if match := storePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			page.Stores = append(page.Stores, manifest.Store{
				Name: match[1],
				Type: manifest.GoTypeRef{Alias: match[2], Name: match[3], Span: span},
				Init: manifest.GoFuncRef{Alias: match[4], Name: match[5], Span: span},
				Span: span,
			})
			continue
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

		if match := actionEndpointPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			method := match[2]
			route := match[3]
			span := sourceLineSpan(lineNumber, rawLine)
			if !isExportedIdentifier(name) {
				return manifest.Page{}, fmt.Errorf("line %d: action handler %q must be an exported Go identifier", lineNumber, name)
			}
			if method != "POST" {
				return manifest.Page{}, fmt.Errorf("line %d: action %s uses unsupported method %s; actions currently require POST", lineNumber, name, method)
			}
			page.Blocks.Actions = append(page.Blocks.Actions, manifest.Action{
				Name:        name,
				Method:      method,
				Route:       route,
				Span:        span,
				RouteSpan:   span,
				RouteParams: routeParamSpans(route, lineNumber, rawLine),
			})
			page.Blocks.Spans.Actions = append(page.Blocks.Spans.Actions, manifest.NamedSpan{Name: name, Span: span})
			continue
		}
		if match := actionPattern.FindStringSubmatch(line); match != nil {
			return manifest.Page{}, fmt.Errorf("line %d: old action block syntax is not supported; use `act %s POST \"<path>\"` and move behavior to Go", lineNumber, exportedIdentifierSuggestion(match[1]))
		}

		if match := apiEndpointPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			method := match[2]
			route := match[3]
			span := sourceLineSpan(lineNumber, rawLine)
			if !isExportedIdentifier(name) {
				return manifest.Page{}, fmt.Errorf("line %d: API handler %q must be an exported Go identifier", lineNumber, name)
			}
			page.Blocks.APIs = append(page.Blocks.APIs, manifest.API{
				Name:        name,
				Method:      method,
				Route:       route,
				Span:        span,
				RouteSpan:   span,
				RouteParams: routeParamSpans(route, lineNumber, rawLine),
			})
			page.Blocks.Spans.APIs = append(page.Blocks.Spans.APIs, manifest.NamedSpan{Name: name, Span: span})
			continue
		}
		if match := apiPattern.FindStringSubmatch(line); match != nil {
			return manifest.Page{}, fmt.Errorf("line %d: old API block syntax is not supported; use `api %s GET \"<path>\"` and move behavior to Go", lineNumber, exportedIdentifierSuggestion(match[1]))
		}

		if match := fragmentEndpointPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			method := match[2]
			route := match[3]
			target := match[4]
			span := sourceLineSpan(lineNumber, rawLine)
			if method != "GET" {
				return manifest.Page{}, fmt.Errorf("line %d: fragment %s uses unsupported method %s; fragments currently require GET", lineNumber, name, method)
			}
			if err := validateFragmentTarget(target); err != nil {
				return manifest.Page{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			page.Blocks.Fragments = append(page.Blocks.Fragments, manifest.FragmentEndpoint{
				Name:        name,
				Method:      method,
				Route:       route,
				Target:      target,
				Span:        span,
				RouteSpan:   span,
				TargetSpan:  span,
				RouteParams: routeParamSpans(route, lineNumber, rawLine),
			})
			page.Blocks.Spans.Fragments = append(page.Blocks.Spans.Fragments, manifest.NamedSpan{Name: name, Span: span})
			capturedFragment = len(page.Blocks.Fragments) - 1
			fragmentBody = nil
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
	if capturedFragment >= 0 {
		return manifest.Page{}, fmt.Errorf("fragment %s block missing closing }", page.Blocks.Fragments[capturedFragment].Name)
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
		return fmt.Errorf("fragment target %q must be a literal id selector", value)
	}
	if strings.ContainsAny(value, "{}") {
		return fmt.Errorf("fragment target %q must be literal", value)
	}
	return nil
}

// ParseComponent extracts component metadata and top-level block declarations.
func ParseComponent(source []byte) (manifest.Component, error) {
	var component manifest.Component
	var viewBody []string
	inView := false
	inProps := false
	inExports := false
	inEmits := false
	var clientBody []string
	inClient := false
	clientDepth := 0
	seenDeclaration := false

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inClient {
			if line == "}" && clientDepth == 1 {
				component.Blocks.ClientBody = strings.TrimSpace(strings.Join(clientBody, "\n"))
				inClient = false
				clientBody = nil
				clientDepth = 0
				continue
			}
			clientBody = append(clientBody, rawLine)
			clientDepth += braceDelta(rawLine)
			if clientDepth < 1 {
				return manifest.Component{}, fmt.Errorf("line %d: client block closed unexpectedly", lineNumber)
			}
			continue
		}
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
		if inExports {
			if line == "}" {
				inExports = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			match := propPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: invalid export declaration %q", lineNumber, line)
			}
			if !supportedScalarType(match[2]) {
				return manifest.Component{}, fmt.Errorf("line %d: export %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Exports = append(component.Exports, manifest.Export{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		if inEmits {
			if line == "}" {
				inEmits = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			event, err := parseEmitDeclaration(line, lineNumber, rawLine)
			if err != nil {
				return manifest.Component{}, err
			}
			component.Emits = append(component.Emits, event)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return manifest.Component{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			component.Package = match[1]
			component.PackageSpan = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return manifest.Component{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

		if match := importPattern.FindStringSubmatch(line); match != nil {
			component.Imports = append(component.Imports, manifest.Import{
				Alias: match[1],
				Path:  match[2],
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedImport(line) {
			return manifest.Component{}, fmt.Errorf("line %d: malformed import %q", lineNumber, line)
		}
		if match := usePattern.FindStringSubmatch(line); match != nil {
			component.Uses = append(component.Uses, manifest.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return manifest.Component{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
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

		if match := componentTypePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			kind := match[1]
			typeRef := manifest.GoTypeRef{Alias: match[2], Name: match[3], Span: span}
			initRef := manifest.GoFuncRef{Alias: match[4], Name: match[5], Span: span}
			switch kind {
			case "props":
				if initRef.Name != "" {
					return manifest.Component{}, fmt.Errorf("line %d: props contract must not declare an init function", lineNumber)
				}
				if component.PropsType.Name != "" || len(component.Props) > 0 {
					return manifest.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
				}
				component.PropsType = typeRef
			case "state":
				if initRef.Name == "" {
					return manifest.Component{}, fmt.Errorf("line %d: state contract requires an init function", lineNumber)
				}
				if component.State.Type.Name != "" {
					return manifest.Component{}, fmt.Errorf("line %d: component declares multiple state contracts", lineNumber)
				}
				component.State = manifest.StateContract{Type: typeRef, Init: initRef, Span: span}
			}
			continue
		}

		switch line {
		case "props {":
			if component.PropsType.Name != "" || len(component.Props) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
			}
			inProps = true
			continue
		case "exports {":
			if len(component.Exports) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple exports blocks", lineNumber)
			}
			component.Blocks.Spans.Exports = sourceLineSpan(lineNumber, rawLine)
			inExports = true
			continue
		case "client {":
			if component.Blocks.Client {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple client blocks", lineNumber)
			}
			component.Blocks.Client = true
			component.Blocks.Spans.Client = sourceLineSpan(lineNumber, rawLine)
			inClient = true
			clientDepth = 1
			continue
		case "emits {":
			if len(component.Emits) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple emits blocks", lineNumber)
			}
			component.Blocks.Spans.Emits = sourceLineSpan(lineNumber, rawLine)
			inEmits = true
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
	if inExports {
		return manifest.Component{}, fmt.Errorf("exports block missing closing }")
	}
	if inEmits {
		return manifest.Component{}, fmt.Errorf("emits block missing closing }")
	}
	if inClient {
		return manifest.Component{}, fmt.Errorf("client block missing closing }")
	}
	if component.Name == "" {
		return manifest.Component{}, fmt.Errorf("missing @component")
	}
	return component, nil
}

func supportedScalarType(value string) bool {
	return value == "string" || value == "int" || value == "float" || value == "bool"
}

func parseEmitDeclaration(line string, lineNumber int, rawLine string) (manifest.Emit, error) {
	match := emitPattern.FindStringSubmatch(line)
	if match == nil {
		return manifest.Emit{}, fmt.Errorf("line %d: invalid emit declaration %q", lineNumber, line)
	}
	params, err := parseEmitParams(match[2], lineNumber, rawLine)
	if err != nil {
		return manifest.Emit{}, err
	}
	return manifest.Emit{Name: match[1], Params: params, Span: sourceLineSpan(lineNumber, rawLine)}, nil
}

func parseEmitParams(source string, lineNumber int, rawLine string) ([]manifest.EmitParam, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	parts := strings.Split(source, ",")
	params := make([]manifest.EmitParam, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		fields := strings.Fields(item)
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: emit parameter %q must use `name type`", lineNumber, item)
		}
		name, typ := fields[0], fields[1]
		if !identifierPattern.MatchString(name) {
			return nil, fmt.Errorf("line %d: invalid emit parameter name %q", lineNumber, name)
		}
		if !supportedScalarType(typ) {
			return nil, fmt.Errorf("line %d: emit parameter %s uses unsupported type %q", lineNumber, name, typ)
		}
		if seen[name] {
			return nil, fmt.Errorf("line %d: duplicate emit parameter %q", lineNumber, name)
		}
		seen[name] = true
		params = append(params, manifest.EmitParam{Name: name, Type: typ, Span: sourceLineSpan(lineNumber, rawLine)})
	}
	return params, nil
}

// ParseLayout extracts layout metadata and top-level block declarations.
func ParseLayout(source []byte) (manifest.Layout, error) {
	var layout manifest.Layout
	var viewBody []string
	inView := false
	seenDeclaration := false

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

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return manifest.Layout{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			layout.Package = match[1]
			layout.PackageSpan = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return manifest.Layout{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

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

		if match := usePattern.FindStringSubmatch(line); match != nil {
			layout.Uses = append(layout.Uses, manifest.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return manifest.Layout{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
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
		route, params, spans, err := parseRouteDeclaration(trimQuotes(value), lineNumber, rawLine)
		if err != nil {
			return err
		}
		page.Route = route
		page.RouteParams = params
		page.Spans.Route = span
		page.Spans.RouteParams = spans
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
	case "cache":
		policy, err := cachePolicyValue(value)
		if err != nil {
			return err
		}
		page.Cache = policy
		page.Spans.Cache = span
	case "revalidate":
		seconds, err := revalidateSecondsValue(value)
		if err != nil {
			return err
		}
		page.Revalidate = seconds
		page.Spans.Revalidate = span
	case "error":
		errorPage, err := manifest.ErrorPagePath(trimQuotes(value))
		if err != nil {
			return err
		}
		page.ErrorPage = errorPage
		page.Spans.ErrorPage = span
	case "title":
		title, err := annotationText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Title = title
		page.Spans.Title = span
	case "description":
		description, err := annotationText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Description = description
		page.Spans.Description = span
	case "canonical":
		canonical, err := annotationText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Canonical = canonical
		page.Spans.Canonical = span
	case "image":
		image, err := annotationText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Image = image
		page.Spans.Image = span
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

func annotationText(name, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("@%s requires a value", name)
	}
	text := strings.TrimSpace(trimQuotes(value))
	if text == "" {
		return "", fmt.Errorf("@%s requires a non-empty value", name)
	}
	return text, nil
}

func cachePolicyValue(value string) (string, error) {
	policy := strings.TrimSpace(trimQuotes(value))
	if policy == "" {
		return "", fmt.Errorf("@cache requires a value")
	}
	if strings.ContainsAny(policy, "\r\n") {
		return "", fmt.Errorf("@cache must stay on one line")
	}
	return policy, nil
}

func revalidateSecondsValue(value string) (string, error) {
	raw := strings.TrimSpace(trimQuotes(value))
	if raw == "" {
		return "", fmt.Errorf("@revalidate requires a value")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("@revalidate must stay on one line")
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return "", fmt.Errorf("@revalidate requires a positive duration")
		}
		return strconv.Itoa(seconds), nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("@revalidate requires a positive duration such as 60s, 5m, or 1h")
	}
	if duration%time.Second != 0 {
		return "", fmt.Errorf("@revalidate must resolve to whole seconds")
	}
	return strconv.FormatInt(int64(duration/time.Second), 10), nil
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
	case "wasm":
		if value == "" {
			return fmt.Errorf("@wasm requires a package path")
		}
		component.WASM = manifest.WASMContract{
			Package: trimQuotes(value),
			Span:    sourceLineSpan(lineNumber, rawLine),
		}
	case "css":
		if value == "" {
			return fmt.Errorf("@css requires a value")
		}
		component.CSS = splitCSSList(value)
		component.Spans.CSS = namedValueSpans(component.CSS, lineNumber, rawLine)
	case "asset":
		if value == "" {
			return fmt.Errorf("@asset requires a value")
		}
		component.Assets = splitCSSList(value)
		component.Spans.Assets = namedValueSpans(component.Assets, lineNumber, rawLine)
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

func parseRouteDeclaration(route string, lineNumber int, rawLine string) (string, []manifest.RouteParam, []manifest.NamedSpan, error) {
	matches := routeParamPattern.FindAllStringSubmatchIndex(route, -1)
	if len(matches) == 0 {
		return route, nil, nil, nil
	}
	routeStart := strings.Index(rawLine, route)
	if routeStart < 0 {
		routeStart = 0
	}
	var normalized strings.Builder
	normalized.Grow(len(route))
	last := 0
	params := make([]manifest.RouteParam, 0, len(matches))
	spans := make([]manifest.NamedSpan, 0, len(matches))
	for _, match := range matches {
		name := route[match[2]:match[3]]
		paramType := "string"
		if match[4] >= 0 && match[5] >= 0 {
			paramType = route[match[4]:match[5]]
		}
		if !isSupportedRouteParamType(paramType) {
			return "", nil, nil, fmt.Errorf("unsupported route parameter type %q for %s; supported types: string, int, int64, uint, uint64, bool, float64", paramType, name)
		}
		start := routeStart + match[0]
		end := routeStart + match[1]
		span := manifest.SourceSpan{
			Start: manifest.SourcePosition{Line: lineNumber, Column: start + 1},
			End:   manifest.SourcePosition{Line: lineNumber, Column: end + 1},
		}
		params = append(params, manifest.RouteParam{Name: name, Type: paramType, Span: span})
		spans = append(spans, manifest.NamedSpan{
			Name: name,
			Span: span,
		})
		normalized.WriteString(route[last:match[0]])
		normalized.WriteString("{")
		normalized.WriteString(name)
		normalized.WriteString("}")
		last = match[1]
	}
	normalized.WriteString(route[last:])
	return normalized.String(), params, spans, nil
}

func routeParamSpans(route string, lineNumber int, rawLine string) []manifest.NamedSpan {
	_, _, spans, _ := parseRouteDeclaration(route, lineNumber, rawLine)
	return spans
}

func isSupportedRouteParamType(value string) bool {
	switch value {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64":
		return true
	default:
		return false
	}
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

func braceDelta(line string) int {
	delta := 0
	for _, r := range line {
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

func isMalformedImport(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "import"
}

func isMalformedUse(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "use"
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

func isExportedIdentifier(value string) bool {
	if !identifierPattern.MatchString(value) {
		return false
	}
	for _, r := range value {
		return r >= 'A' && r <= 'Z'
	}
	return false
}

func exportedIdentifierSuggestion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Handler"
	}
	var builder strings.Builder
	upperNext := true
	for _, r := range value {
		if !isIdentStart(r) && (r < '0' || r > '9') {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && r >= '0' && r <= '9' {
			builder.WriteByte('X')
		}
		if upperNext {
			if r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			}
			upperNext = false
		}
		builder.WriteRune(r)
	}
	if builder.Len() == 0 {
		return "Handler"
	}
	return builder.String()
}
