package appgen

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func resolveOptions(outputDir string, options Options) (Options, error) {
	resolved := options
	if !options.AutoRoutes {
		assignBackendAliases(&resolved)
		return resolved, nil
	}
	ir, err := optionsIR(options)
	if err != nil {
		return Options{}, err
	}

	actions, err := actionEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	apis, err := apiEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	ssrArtifacts, err := buildgen.SSRArtifactsFromIR(options.Config, ir, outputDir)
	if err != nil {
		return Options{}, err
	}

	resolved.Actions = append(append([]ActionEndpoint(nil), options.Actions...), actions...)
	resolved.APIs = append(append([]APIEndpoint(nil), options.APIs...), apis...)
	resolved.SSR = append(append([]SSRRoute(nil), options.SSR...), ssrRoutes(ssrArtifacts)...)
	assignBackendAliases(&resolved)
	return resolved, nil
}

func resolveBackendOptions(options Options) (Options, error) {
	resolved := options
	if !options.AutoRoutes {
		assignBackendAliases(&resolved)
		return resolved, nil
	}
	ir, err := optionsIR(options)
	if err != nil {
		return Options{}, err
	}
	actions, err := actionEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	apis, err := apiEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	resolved.Actions = append(append([]ActionEndpoint(nil), options.Actions...), actions...)
	resolved.APIs = append(append([]APIEndpoint(nil), options.APIs...), apis...)
	resolved.SSR = nil
	assignBackendAliases(&resolved)
	return resolved, nil
}

func optionsIR(options Options) (gwdkir.Program, error) {
	if options.IR != nil {
		return *options.IR, nil
	}
	return gwdkir.Program{}, fmt.Errorf("auto route detection requires compiler IR")
}

func assignBackendAliases(options *Options) {
	paths := map[string]string{}
	for _, action := range options.Actions {
		if action.Binding.Status == manifest.BackendBindingBound && action.Binding.ImportPath != "" {
			paths[action.Binding.ImportPath] = action.Binding.PackageName
		}
	}
	for _, api := range options.APIs {
		if api.Binding.Status == manifest.BackendBindingBound && api.Binding.ImportPath != "" {
			paths[api.Binding.ImportPath] = api.Binding.PackageName
		}
	}
	for _, route := range options.SSR {
		if route.LoadBinding.Status == manifest.BackendBindingBound && route.LoadBinding.ImportPath != "" {
			paths[route.LoadBinding.ImportPath] = route.LoadBinding.PackageName
		}
	}
	if len(paths) == 0 {
		return
	}
	importPaths := make([]string, 0, len(paths))
	for importPath := range paths {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	aliases := map[string]string{}
	used := map[string]int{}
	for _, importPath := range importPaths {
		base := safeImportAlias(paths[importPath])
		if base == "" {
			base = safeImportAlias(path.Base(importPath))
		}
		if base == "" {
			base = "feature"
		}
		used[base]++
		alias := base
		if used[base] > 1 {
			alias = fmt.Sprintf("%s%d", base, used[base])
		}
		aliases[importPath] = alias
	}
	for index := range options.Actions {
		options.Actions[index].BackendAlias = aliases[options.Actions[index].Binding.ImportPath]
	}
	for index := range options.APIs {
		options.APIs[index].BackendAlias = aliases[options.APIs[index].Binding.ImportPath]
	}
	for index := range options.SSR {
		options.SSR[index].LoadBackendAlias = aliases[options.SSR[index].LoadBinding.ImportPath]
	}
}

func safeImportAlias(value string) string {
	var builder strings.Builder
	for index, char := range strings.TrimSpace(value) {
		valid := char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)
		if !valid {
			continue
		}
		if index == 0 && unicode.IsDigit(char) {
			builder.WriteByte('p')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func ssrRoutes(artifacts []buildgen.SSRArtifact) []SSRRoute {
	routes := make([]SSRRoute, 0, len(artifacts))
	for _, artifact := range artifacts {
		routes = append(routes, SSRRoute{
			PageID:           artifact.PageID,
			Route:            artifact.Route,
			Render:           artifact.Render,
			Cache:            artifact.Cache,
			DynamicParams:    append([]string(nil), artifact.DynamicParams...),
			RouteParams:      append([]manifest.RouteParam(nil), artifact.RouteParams...),
			Guards:           append([]string(nil), artifact.Guards...),
			HasLoad:          artifact.HasLoad,
			LoadBinding:      artifact.LoadBinding,
			HTML:             artifact.HTML,
			Replacements:     ssrReplacements(artifact.Replacements),
			LoadReplacements: ssrLoadReplacements(artifact.LoadReplacements),
		})
	}
	return routes
}

func ssrReplacements(replacements []buildgen.SSRReplacement) []SSRReplacement {
	out := make([]SSRReplacement, 0, len(replacements))
	for _, replacement := range replacements {
		out = append(out, SSRReplacement{
			Param:       replacement.Param,
			Placeholder: replacement.Placeholder,
		})
	}
	return out
}

func ssrLoadReplacements(replacements []buildgen.SSRLoadReplacement) []SSRLoadReplacement {
	out := make([]SSRLoadReplacement, 0, len(replacements))
	for _, replacement := range replacements {
		out = append(out, SSRLoadReplacement{
			Path:        replacement.Path,
			Placeholder: replacement.Placeholder,
		})
	}
	return out
}
