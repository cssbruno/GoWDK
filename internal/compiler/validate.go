package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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

// ValidateProgram checks render-mode invariants that must hold before codegen.
// The validators read the compiler IR directly; there is no manifest
// intermediary on this path.
func ValidateProgram(config gowdk.Config, ir gwdkir.Program) error {
	return validateProgram(config, ir, true)
}

// ValidateSourceProgram checks a program built from a single in-memory source
// buffer. Cross-file checks (use packages and component references resolved
// against the discovered project) are skipped because sibling files are not
// present in the program.
func ValidateSourceProgram(config gowdk.Config, ir gwdkir.Program) error {
	return validateProgram(config, ir, false)
}

func validateProgram(config gowdk.Config, ir gwdkir.Program, crossFile bool) error {
	if err := gwdkir.CheckInvariants(ir); err != nil {
		return fmt.Errorf("internal compiler error: %w", err)
	}
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validatePackages(ir)...)
	diagnostics = append(diagnostics, validateUniquePages(ir.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentEmits(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentGoContracts(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentStoreUses(ir.Pages, ir.Components)...)
	diagnostics = append(diagnostics, validateRedundantComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateGOWDKUses(ir, crossFile)...)
	diagnostics = append(diagnostics, validatePageAssetUses(ir)...)
	diagnostics = append(diagnostics, validateUniqueLayouts(ir.Layouts)...)
	diagnostics = append(diagnostics, validateLayoutReferences(ir.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(ir.Pages, ir.Layouts)...)
	diagnostics = append(diagnostics, validateGoBlocks(config, ir)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(ir.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(ir.Pages, ir.GoEndpoints)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(ir.Pages, ir.GoEndpoints)...)
	diagnostics = append(diagnostics, validateStandaloneEndpoints(ir.GoEndpoints)...)
	for _, page := range ir.Pages {
		diagnostics = append(diagnostics, ValidatePage(config, page)...)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}
