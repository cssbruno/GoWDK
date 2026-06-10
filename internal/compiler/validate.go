package compiler

import (
	"fmt"
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/source"
)

type ValidationError struct {
	Code          string
	PageID        string
	ComponentName string
	Source        string
	Span          source.SourceSpan
	Message       string
}

func (err ValidationError) Error() string {
	if err.PageID == "" {
		if err.ComponentName != "" {
			return fmt.Sprintf("%s: %s", err.ComponentName, err.Message)
		}
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.PageID, err.Message)
}

// ValidateProgram is the IR-native structural validation. It checks the
// render-mode invariants that must hold before codegen by calling the migrated
// validators directly on the IR, with no manifest intermediary.
func ValidateProgram(config gowdk.Config, ir gwdkir.Program) error {
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validatePackages(ir)...)
	diagnostics = append(diagnostics, validateUniquePages(ir.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentEmits(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentGoContracts(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentStoreUses(ir.Pages, ir.Components)...)
	diagnostics = append(diagnostics, validateRedundantComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateGOWDKUses(ir)...)
	diagnostics = append(diagnostics, validatePageAssetUses(ir)...)
	diagnostics = append(diagnostics, validateUniqueLayouts(ir.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(ir.Pages, ir.Layouts)...)
	diagnostics = append(diagnostics, validateGoBlocks(config, ir)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(ir.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(ir.Pages)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(ir.Pages, ir.GoEndpoints)...)
	diagnostics = append(diagnostics, validateStandaloneEndpoints(ir.GoEndpoints)...)
	for _, page := range ir.Pages {
		diagnostics = append(diagnostics, validatePageIR(config, page)...)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}

// ValidateManifest validates a parsed manifest by lowering it to IR first.
func ValidateManifest(config gowdk.Config, app manifest.Manifest) error {
	return ValidateProgram(config, gwdkanalysis.BuildIR(config, app))
}

// ValidatePage stays exported (tests call it with a manifest.Page); it lowers a
// single page to IR and validates it.
func ValidatePage(config gowdk.Config, page manifest.Page) []ValidationError {
	ir := gwdkanalysis.BuildIR(config, manifest.Manifest{Pages: []manifest.Page{page}})
	if len(ir.Pages) == 0 {
		return nil
	}
	return validatePageIR(config, ir.Pages[0])
}
