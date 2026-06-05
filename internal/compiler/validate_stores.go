package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func validateComponentStoreUses(pages []manifest.Page, components []manifest.Component) []ValidationError {
	declared := declaredStoreNames(pages)
	if len(declared) == 0 {
		return validateStoreUsesAgainst(nil, components)
	}
	return validateStoreUsesAgainst(declared, components)
}

func declaredStoreNames(pages []manifest.Page) map[string]bool {
	declared := map[string]bool{}
	for _, page := range pages {
		for _, store := range page.Stores {
			if store.Name != "" {
				declared[store.Name] = true
			}
		}
	}
	return declared
}

func validateStoreUsesAgainst(declared map[string]bool, components []manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
			continue
		}
		program, err := clientlang.Parse(component.Blocks.ClientBody)
		if err != nil {
			continue
		}
		for _, use := range program.Uses {
			if declared[use.Name] {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "unknown_component_store",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientSpan(component, use.Span),
				Message:       fmt.Sprintf("component %s uses store %q, but no page declares store %s", component.Name, use.Name, use.Name),
			})
		}
	}
	return diagnostics
}
