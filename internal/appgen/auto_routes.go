package appgen

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
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
	fragments, err := fragmentEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	ssrArtifacts, err := buildgen.SSRArtifactsFromIR(options.Config, ir, outputDir)
	if err != nil {
		return Options{}, err
	}

	resolved.Actions = append(append([]ActionEndpoint(nil), options.Actions...), actions...)
	resolved.APIs = append(append([]APIEndpoint(nil), options.APIs...), apis...)
	resolved.Fragments = append(append([]FragmentEndpoint(nil), options.Fragments...), fragments...)
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
	fragments, err := fragmentEndpointsFromIR(ir)
	if err != nil {
		return Options{}, err
	}
	resolved.Actions = append(append([]ActionEndpoint(nil), options.Actions...), actions...)
	resolved.APIs = append(append([]APIEndpoint(nil), options.APIs...), apis...)
	resolved.Fragments = append(append([]FragmentEndpoint(nil), options.Fragments...), fragments...)
	resolved.SSR = nil
	assignBackendAliases(&resolved)
	return resolved, nil
}

func optionsIR(options Options) (gwdkir.Program, error) {
	if options.IR != nil {
		if err := gwdkir.CheckInvariants(*options.IR); err != nil {
			return gwdkir.Program{}, fmt.Errorf("invalid compiler IR: %w", err)
		}
		return *options.IR, nil
	}
	return gwdkir.Program{}, fmt.Errorf("auto route detection requires compiler IR")
}

func assignBackendAliases(options *Options) {
	paths := map[string]string{}
	for _, action := range options.Actions {
		if action.Binding.Status == source.BackendBindingBound && action.Binding.ImportPath != "" {
			paths[action.Binding.ImportPath] = action.Binding.PackageName
		}
	}
	for _, api := range options.APIs {
		if api.Binding.Status == source.BackendBindingBound && api.Binding.ImportPath != "" {
			paths[api.Binding.ImportPath] = api.Binding.PackageName
		}
	}
	for _, fragment := range options.Fragments {
		if fragment.Binding.Status == source.BackendBindingBound && fragment.Binding.ImportPath != "" {
			paths[fragment.Binding.ImportPath] = fragment.Binding.PackageName
		}
	}
	for _, route := range options.SSR {
		if route.LoadBinding.Status == source.BackendBindingBound && route.LoadBinding.ImportPath != "" {
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
	used := generatedImportAliasUseCounts()
	for _, importPath := range importPaths {
		base := safeImportAlias(paths[importPath])
		if base == "" {
			base = safeImportAlias(path.Base(importPath))
		}
		if base == "" {
			base = "feature"
		}
		aliases[importPath] = nextImportAlias(base, used)
	}
	for index := range options.Actions {
		options.Actions[index].BackendAlias = aliases[options.Actions[index].Binding.ImportPath]
	}
	for index := range options.APIs {
		options.APIs[index].BackendAlias = aliases[options.APIs[index].Binding.ImportPath]
	}
	for index := range options.Fragments {
		options.Fragments[index].BackendAlias = aliases[options.Fragments[index].Binding.ImportPath]
	}
	for index := range options.SSR {
		options.SSR[index].LoadBackendAlias = aliases[options.SSR[index].LoadBinding.ImportPath]
	}
}

func generatedImportAliasUseCounts() map[string]int {
	used := map[string]int{}
	for _, alias := range []string{
		"context",
		"embed",
		"errors",
		"fmt",
		"fs",
		"gowdkactions",
		"gowdkauth",
		"gowdkcontracts",
		"gowdkform",
		"gowdkguard",
		"gowdkhtml",
		"gowdkpartial",
		"gowdkratelimit",
		"gowdkresponse",
		"gowdkruntime",
		"gowdkroute",
		"gowdkssr",
		"gowdkvalidation",
		"http",
		"httputil",
		"neturl",
		"os",
		"path",
		"strings",
		"sync",
		"utf8",
	} {
		used[alias] = 1
	}
	return used
}

func nextImportAlias(base string, used map[string]int) string {
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s%d", base, used[base])
}

func safeImportAlias(value string) string {
	out := make([]rune, 0, len(value))
	for index, char := range strings.TrimSpace(value) {
		valid := char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)
		if !valid {
			continue
		}
		if index == 0 && unicode.IsDigit(char) {
			out = append(out, 'p')
		}
		out = append(out, char)
	}
	return string(out)
}

func ssrRoutes(artifacts []buildgen.SSRArtifact) []SSRRoute {
	routes := make([]SSRRoute, 0, len(artifacts))
	for _, artifact := range artifacts {
		routes = append(routes, SSRRoute{
			PageID:           artifact.PageID,
			Route:            artifact.Route,
			Render:           artifact.Render,
			Cache:            artifact.Cache,
			ErrorPage:        artifact.ErrorPage,
			DynamicParams:    append([]string(nil), artifact.DynamicParams...),
			RouteParams:      append([]source.RouteParam(nil), artifact.RouteParams...),
			Layouts:          append([]string(nil), artifact.Layouts...),
			Guards:           append([]string(nil), artifact.Guards...),
			HasLoad:          artifact.HasLoad,
			LoadBinding:      artifact.LoadBinding,
			HTML:             artifact.HTML,
			Replacements:     append([]SSRReplacement(nil), artifact.Replacements...),
			LoadReplacements: append([]SSRLoadReplacement(nil), artifact.LoadReplacements...),
			ListSpecs:        append([]SSRListSpec(nil), artifact.ListSpecs...),
			CondSpecs:        append([]SSRCondSpec(nil), artifact.CondSpecs...),
			QueryRegions:     append([]SSRQueryRegion(nil), artifact.QueryRegions...),
		})
	}
	return routes
}
