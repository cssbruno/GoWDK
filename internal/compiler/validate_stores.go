package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

// validatePersistedStoreConflicts warns when two pages persist a store under
// the same name but with different struct shapes. Persistence keys on the store
// name, so they share one browser storage slot; their differing schema hashes
// then discard each other's saved value on every navigation between the pages.
func validatePersistedStoreConflicts(pages []gwdkir.Page) []ValidationError {
	type seenStore struct {
		page      gwdkir.Page
		store     gwdkir.Store
		signature string
	}
	seen := map[string]seenStore{}
	var diagnostics []ValidationError
	for _, page := range pages {
		for _, store := range page.Stores {
			if store.Name == "" || (store.Persist != "local" && store.Persist != "session") {
				continue
			}
			signature := persistedStoreSignature(page, store)
			if signature == "" {
				continue // unresolvable types are already reported as page_store_error
			}
			prior, exists := seen[store.Name]
			if !exists {
				seen[store.Name] = seenStore{page: page, store: store, signature: signature}
				continue
			}
			if prior.signature == signature {
				continue // same shape sharing one key across routes is intended
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:     "page_store_persist_key_conflict",
				PageID:   page.ID,
				Source:   page.Source,
				Span:     firstSpan(store.Span, page.Spans.Page),
				Severity: SeverityWarning,
				Related:  relatedSpan(prior.page.Source, prior.store.Span, fmt.Sprintf("store %q first persisted here on page %s", prior.store.Name, prior.page.ID)),
				Message: fmt.Sprintf(
					"persisted store %q has different shapes on pages %s and %s but shares browser storage key %q; navigating between them discards each other's saved data. Rename one store or give them matching shapes",
					store.Name, prior.page.ID, page.ID, "gowdk:store:"+store.Name,
				),
			})
		}
	}
	return diagnostics
}

func persistedStoreSignature(page gwdkir.Page, store gwdkir.Store) string {
	resolved, err := gotypes.ResolveStruct(page.Imports, store.Type)
	if err != nil {
		return ""
	}
	parts := make([]string, 0, len(resolved.Fields))
	for _, field := range resolved.Fields {
		parts = append(parts, field.Name+":"+field.Type)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

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
