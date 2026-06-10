package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func validatePageAssetUses(ir gwdkir.Program) []ValidationError {
	sourcePackages := gowdkSourcePackages(ir)
	var diagnostics []ValidationError
	for _, page := range ir.Pages {
		usesByAlias := map[string]gwdkir.Use{}
		for _, use := range page.Uses {
			if _, exists := usesByAlias[use.Alias]; !exists {
				usesByAlias[use.Alias] = use
			}
		}
		for _, css := range page.CSS {
			alias, _, ok := strings.Cut(css, ".")
			if !ok {
				continue
			}
			use, exists := usesByAlias[alias]
			if !exists {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_gowdk_use_alias",
					PageID: page.ID,
					Source: page.Source,
					Span:   spanForName(page.Spans.CSS, css, page.Spans.Page),
					Message: fmt.Sprintf(
						"%s selects CSS asset %q, but alias %q is not declared. Add `use %s \"<package>\"` before @css",
						page.ID,
						css,
						alias,
						alias,
					),
				})
				continue
			}
			if !sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_gowdk_use_package",
					PageID: page.ID,
					Source: page.Source,
					Span:   spanForName(page.Spans.CSS, css, page.Spans.Page),
					Message: fmt.Sprintf(
						"%s selects CSS asset %q through alias %q, but no discovered .gwdk file declares package %s",
						page.ID,
						css,
						alias,
						use.Package,
					),
				})
			}
		}
	}
	return diagnostics
}

func gowdkSourcePackages(ir gwdkir.Program) map[string]bool {
	sourcePackages := map[string]bool{}
	for _, page := range ir.Pages {
		if page.Package != "" {
			sourcePackages[page.Package] = true
		}
	}
	for _, component := range ir.Components {
		if component.Package != "" {
			sourcePackages[component.Package] = true
		}
	}
	for _, layout := range ir.Layouts {
		if layout.Package != "" {
			sourcePackages[layout.Package] = true
		}
	}
	return sourcePackages
}
