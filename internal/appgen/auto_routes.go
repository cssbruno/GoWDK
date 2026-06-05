package appgen

import (
	"fmt"

	"github.com/cssbruno/gowdk/internal/buildgen"
)

func resolveOptions(outputDir string, options Options) (Options, error) {
	resolved := options
	if !options.AutoRoutes {
		return resolved, nil
	}
	if options.Manifest == nil {
		return Options{}, fmt.Errorf("auto route detection requires a parsed manifest")
	}

	actions, err := ActionRoutes(*options.Manifest)
	if err != nil {
		return Options{}, err
	}
	ssrArtifacts, err := buildgen.SSRArtifacts(options.Config, *options.Manifest, outputDir)
	if err != nil {
		return Options{}, err
	}

	resolved.Actions = append(append([]ActionRoute(nil), options.Actions...), actions...)
	resolved.SSR = append(append([]SSRRoute(nil), options.SSR...), ssrRoutes(ssrArtifacts)...)
	return resolved, nil
}

func ssrRoutes(artifacts []buildgen.SSRArtifact) []SSRRoute {
	routes := make([]SSRRoute, 0, len(artifacts))
	for _, artifact := range artifacts {
		routes = append(routes, SSRRoute{
			PageID:       artifact.PageID,
			Route:        artifact.Route,
			HTML:         artifact.HTML,
			Replacements: ssrReplacements(artifact.Replacements),
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
