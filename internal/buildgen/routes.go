package buildgen

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
)

type pageOutput struct {
	route string
	data  map[string]string
}

func pageRouteArtifacts(outputDir string, page gwdkir.Page) ([]Artifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]Artifact, 0, len(outputs))
	for _, output := range outputs {
		outputPath, err := outputPath(outputDir, output.route)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		artifacts = append(artifacts, Artifact{PageID: page.ID, Route: output.route, Path: outputPath, CachePolicy: page.CachePolicy()})
	}
	return artifacts, nil
}

func pageOutputArtifacts(config gowdk.Config, outputDir string, page gwdkir.Page, components map[string]view.Component, layouts map[string]gwdkir.Layout, stylesheets []gowdk.Stylesheet, actionFields map[string][]view.ActionInputField) ([]plannedArtifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]plannedArtifact, 0, len(outputs))
	for _, output := range outputs {
		buildData, err := parseBuildDataFromBlocks(page.Blocks, output.data, page.Imports, page.Source)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		data, err := mergeBuildData(buildData, output.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		html, err := renderPage(config, page, components, layouts, stylesheets, actionFields, data, renderModeSPA)
		if err != nil {
			return nil, err
		}
		outputPath, err := outputPath(outputDir, output.route)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		artifacts = append(artifacts, plannedArtifact{
			Artifact: Artifact{PageID: page.ID, Route: output.route, Path: outputPath, CachePolicy: page.CachePolicy()},
			contents: []byte(html),
		})
	}
	return artifacts, nil
}

func pageOutputs(page gwdkir.Page) ([]pageOutput, error) {
	params := page.DynamicParams()
	if len(params) == 0 {
		return []pageOutput{{route: page.Route}}, nil
	}

	declarations, err := parsePathDeclarationsFromBlocks(page.Blocks)
	if err != nil {
		return nil, err
	}
	if len(declarations) == 0 {
		return nil, fmt.Errorf("paths {} must declare at least one path")
	}

	required := map[string]bool{}
	for _, param := range params {
		required[param] = true
	}

	outputs := make([]pageOutput, 0, len(declarations))
	for index, declaration := range declarations {
		for _, param := range params {
			value, ok := declaration[param]
			if !ok {
				return nil, fmt.Errorf("paths declaration %d missing route param %q", index+1, param)
			}
			if err := validateRouteParamValue(param, value); err != nil {
				return nil, fmt.Errorf("paths declaration %d: %w", index+1, err)
			}
		}
		for name := range declaration {
			if !required[name] {
				return nil, fmt.Errorf("paths declaration %d declares unused route param %q", index+1, name)
			}
		}

		route := expandRouteTemplate(page.Route, declaration, func(value string) string {
			return value
		})
		outputs = append(outputs, pageOutput{
			route: route,
			data:  cloneStringMap(declaration),
		})
	}
	return outputs, nil
}

func outputPath(outputDir, route string) (string, error) {
	if !strings.HasPrefix(route, "/") {
		return "", fmt.Errorf("route %q must start with /", route)
	}
	if strings.ContainsAny(route, "?#") {
		return "", fmt.Errorf("route %q must not contain query or fragment", route)
	}
	if strings.Contains(route, "{") || strings.Contains(route, "}") {
		return "", fmt.Errorf("route %q is dynamic", route)
	}

	trimmed := strings.Trim(route, "/")
	if trimmed == "" {
		return filepath.Join(outputDir, "index.html"), nil
	}

	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("route %q contains unsafe path segment %q", route, segment)
		}
	}

	segments := strings.Split(path.Clean("/"+trimmed), "/")[1:]
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("route %q contains unsafe path segment %q", route, segment)
		}
	}
	parts := append([]string{outputDir}, segments...)
	parts = append(parts, "index.html")
	return filepath.Join(parts...), nil
}

func expandRouteTemplate(route string, data map[string]string, escape func(string) string) string {
	if len(data) == 0 || !strings.Contains(route, "{") {
		return route
	}
	var out strings.Builder
	for index := 0; index < len(route); {
		if route[index] != '{' {
			out.WriteByte(route[index])
			index++
			continue
		}
		end := strings.IndexByte(route[index:], '}')
		if end < 0 {
			out.WriteString(route[index:])
			break
		}
		end += index
		placeholder := route[index : end+1]
		name, ok := routeTemplateParamName(placeholder)
		if !ok {
			out.WriteString(placeholder)
			index = end + 1
			continue
		}
		value, ok := data[name]
		if !ok {
			out.WriteString(placeholder)
			index = end + 1
			continue
		}
		out.WriteString(escape(value))
		index = end + 1
	}
	return out.String()
}

func routeTemplateParamName(placeholder string) (string, bool) {
	params := gwdkir.RouteParamsFromPath(placeholder)
	if len(params) != 1 {
		return "", false
	}
	return params[0].Name, true
}
