package compiler

import (
	"fmt"
	"github.com/cssbruno/gowdk"
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

// ValidateManifest checks render-mode invariants that must hold before codegen.
func ValidateManifest(config gowdk.Config, app manifest.Manifest) error {
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validatePackages(app)...)
	diagnostics = append(diagnostics, validateUniquePages(app.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(app.Components)...)
	diagnostics = append(diagnostics, validateComponentEmits(app.Components)...)
	diagnostics = append(diagnostics, validateComponentGoContracts(app.Components)...)
	diagnostics = append(diagnostics, validateComponentStoreUses(app.Pages, app.Components)...)
	diagnostics = append(diagnostics, validateRedundantComponents(app.Components)...)
	diagnostics = append(diagnostics, validateGOWDKUses(app)...)
	diagnostics = append(diagnostics, validatePageAssetUses(app)...)
	diagnostics = append(diagnostics, validateUniqueLayouts(app.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(app.Pages, app.Layouts)...)
	diagnostics = append(diagnostics, validateGoBlocks(config, app)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(app.Pages, app.Endpoints)...)
	diagnostics = append(diagnostics, validateStandaloneEndpoints(app.Endpoints)...)
	for _, page := range app.Pages {
		diagnostics = append(diagnostics, ValidatePage(config, page)...)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}

// ValidateProgram checks the same render-mode invariants as ValidateManifest
// against an IR-first build path. It reconstructs the manifest from IR via
// ManifestFromIR so generated-output packages no longer need to carry their own
// IR->manifest converter. As validators move to read IR directly, the converter
// shrinks until this entrypoint reads IR with no manifest intermediary.
func ValidateProgram(config gowdk.Config, ir gwdkir.Program) error {
	return ValidateManifest(config, ManifestFromIR(ir))
}
