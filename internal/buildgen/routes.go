package buildgen

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

type pageOutput struct {
	route string
	data  map[string]string
}

func pageRouteArtifacts(outputDir string, page manifest.Page) ([]Artifact, error) {
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
		artifacts = append(artifacts, Artifact{PageID: page.ID, Route: output.route, Path: outputPath})
	}
	return artifacts, nil
}

func pageOutputArtifacts(config gowdk.Config, outputDir string, page manifest.Page, components map[string]view.Component, layouts map[string]manifest.Layout, stylesheets []gowdk.Stylesheet) ([]plannedArtifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]plannedArtifact, 0, len(outputs))
	for _, output := range outputs {
		buildData, err := parseBuildData(page.Blocks.BuildBody, output.data, page.Imports)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		data, err := mergeBuildData(buildData, output.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		html, err := renderPage(config, page, components, layouts, stylesheets, data, renderModeSPA)
		if err != nil {
			return nil, err
		}
		outputPath, err := outputPath(outputDir, output.route)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		artifacts = append(artifacts, plannedArtifact{
			Artifact: Artifact{PageID: page.ID, Route: output.route, Path: outputPath},
			contents: []byte(html),
		})
	}
	return artifacts, nil
}

func pageOutputs(page manifest.Page) ([]pageOutput, error) {
	params := page.DynamicParams()
	if len(params) == 0 {
		return []pageOutput{{route: page.Route}}, nil
	}

	declarations, err := parsePathDeclarations(page.Blocks.PathsBody)
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

		route := page.Route
		for name, value := range declaration {
			route = strings.ReplaceAll(route, "{"+name+"}", value)
		}
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
