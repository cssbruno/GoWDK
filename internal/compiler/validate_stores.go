package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func validateComponentStoreUses(pages []gwdkir.Page, components []gwdkir.Component) []ValidationError {
	declared := declaredStoreNamesByPackage(pages)
	if len(declared) == 0 {
		return validateStoreUsesAgainst(nil, components)
	}
	return validateStoreUsesAgainst(declared, components)
}

func declaredStoreNamesByPackage(pages []gwdkir.Page) map[string]map[string]bool {
	declared := map[string]map[string]bool{}
	for _, page := range pages {
		for _, store := range page.Stores {
			if store.Name == "" {
				continue
			}
			if declared[page.Package] == nil {
				declared[page.Package] = map[string]bool{}
			}
			declared[page.Package][store.Name] = true
		}
	}
	return declared
}

func validateStoreUsesAgainst(declared map[string]map[string]bool, components []gwdkir.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
			continue
		}
		program, err := clientlang.Parse(component.Blocks.ClientBody)
		if err != nil {
			continue
		}
		usesByAlias := componentUsesByAlias(component)
		for _, use := range program.Uses {
			if use.PackageAlias != "" {
				gowdkUse, exists := usesByAlias[use.PackageAlias]
				if !exists {
					diagnostics = append(diagnostics, ValidationError{
						Code:          "unknown_gowdk_use_alias",
						ComponentName: component.Name,
						Source:        component.Source,
						Span:          clientSpan(component, use.Span),
						Message: fmt.Sprintf(
							"component %s uses store %q, but alias %q is not declared. Add `use %s \"<package>\"` before the client block",
							component.Name,
							use.Name,
							use.PackageAlias,
							use.PackageAlias,
						),
					})
					continue
				}
				if declared[gowdkUse.Package][use.StoreName] {
					continue
				}
				diagnostics = append(diagnostics, ValidationError{
					Code:          "unknown_component_store",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          clientSpan(component, use.Span),
					Message: fmt.Sprintf(
						"component %s uses store %q through alias %q, but GOWDK package %s does not declare store %s",
						component.Name,
						use.Name,
						use.PackageAlias,
						gowdkUse.Package,
						use.StoreName,
					),
				})
				continue
			}
			if declared[component.Package][use.Name] || declared[""][use.Name] {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "unknown_component_store",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientSpan(component, use.Span),
				Message: fmt.Sprintf(
					"component %s uses store %q, but no same-package page declares store %s. For cross-package stores, add `use alias \"package\"` and write `use alias.%s`",
					component.Name,
					use.Name,
					use.Name,
					use.Name,
				),
			})
		}
	}
	return diagnostics
}

func componentUsesByAlias(component gwdkir.Component) map[string]gwdkir.Use {
	usesByAlias := map[string]gwdkir.Use{}
	for _, use := range component.Uses {
		if _, exists := usesByAlias[use.Alias]; !exists {
			usesByAlias[use.Alias] = use
		}
	}
	return usesByAlias
}
