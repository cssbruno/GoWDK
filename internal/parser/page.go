// Package parser turns .gwdk source files into syntax trees.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/manifest"
)

var (
	annotationPattern     = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)\s*(.*)$`)
	blockPattern          = regexp.MustCompile(`^(paths|build|load|view)\s*\{`)
	actionPattern         = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*\{`)
	apiPattern            = regexp.MustCompile(`^api(?:\s+([A-Za-z_][A-Za-z0-9_.-]*))?\s*\{`)
	propPattern           = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionInputPattern    = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*form\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionValidPattern    = regexp.MustCompile(`^valid\(([A-Za-z_][A-Za-z0-9_]*)\)\?$`)
	actionRedirectPattern = regexp.MustCompile(`^->\s*"([^"]*)"$`)
)

// ParsePage extracts page metadata and top-level block declarations.
func ParsePage(source []byte) (manifest.Page, error) {
	var page manifest.Page
	var blockBody []string
	capturedBlock := ""
	var actionBody []string
	capturedAction := -1

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if capturedAction >= 0 {
			if line == "}" {
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

		if match := annotationPattern.FindStringSubmatch(line); match != nil {
			if err := applyAnnotation(&page, match[1], match[2]); err != nil {
				return manifest.Page{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		if match := blockPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			applyBlock(&page, name)
			if capturesBlockBody(name) {
				capturedBlock = name
			}
			continue
		}

		if match := actionPattern.FindStringSubmatch(line); match != nil {
			page.Blocks.Actions = append(page.Blocks.Actions, manifest.Action{Name: match[1]})
			capturedAction = len(page.Blocks.Actions) - 1
			continue
		}

		if match := apiPattern.FindStringSubmatch(line); match != nil {
			page.Blocks.APIs = append(page.Blocks.APIs, manifest.API{Name: match[1]})
			continue
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
	for index, rawLine := range body {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := actionInputPattern.FindStringSubmatch(line); match != nil {
			if action.InputName != "" {
				return manifest.Action{}, fmt.Errorf("action %s line %d declares multiple form inputs", action.Name, index+1)
			}
			action.InputName = match[1]
			action.InputType = match[2]
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
			continue
		}
		return manifest.Action{}, fmt.Errorf("action %s line %d has unsupported syntax %q", action.Name, index+1, line)
	}
	return action, nil
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
			component.Props = append(component.Props, manifest.Prop{Name: match[1], Type: match[2]})
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := annotationPattern.FindStringSubmatch(line); match != nil {
			if err := applyComponentAnnotation(&component, match[1], match[2]); err != nil {
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
			inView = true
			continue
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

func applyAnnotation(page *manifest.Page, name, rawValue string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "page":
		page.ID = value
	case "route":
		page.Route = trimQuotes(value)
	case "layout":
		page.Layouts = splitList(value)
	case "render":
		mode, err := gowdk.ParseRenderMode(value)
		if err != nil {
			return err
		}
		page.Render = mode
	case "guard":
		page.Guard = splitList(value)
	default:
		return nil
	}
	return nil
}

func applyComponentAnnotation(component *manifest.Component, name, rawValue string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "component":
		component.Name = value
	default:
		return nil
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

func capturesBlockBody(name string) bool {
	return name == "paths" || name == "build" || name == "view"
}

func applyBlockBody(page *manifest.Page, name string, body []string) {
	text := strings.TrimSpace(strings.Join(body, "\n"))
	switch name {
	case "paths":
		page.Blocks.PathsBody = text
	case "build":
		page.Blocks.BuildBody = text
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

func trimQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}
