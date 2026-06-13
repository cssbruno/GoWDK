package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// Severity classifies a diagnostic as a hard error or a non-fatal warning.
type Severity int

const (
	// SeverityError is the default: it fails the build.
	SeverityError Severity = iota
	// SeverityWarning is surfaced to the author but does not fail the build.
	SeverityWarning
)

type ValidationError struct {
	Code          string
	PageID        string
	ComponentName string
	Source        string
	Span          source.SourceSpan
	// Related carries secondary source locations, such as the first declaration
	// that a conflict diagnostic also points at. It is optional and additive.
	Related  []source.RelatedSpan
	Message  string
	Severity Severity
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
// intermediary on this path. It returns a non-nil error only when at least one
// error-severity diagnostic is present; warning-only programs return nil.
func ValidateProgram(config gowdk.Config, ir gwdkir.Program) error {
	return asError(validateProgram(config, ir, true))
}

// ValidateSourceProgram checks a program built from a single in-memory source
// buffer. Cross-file checks (use packages and component references resolved
// against the discovered project) are skipped because sibling files are not
// present in the program.
func ValidateSourceProgram(config gowdk.Config, ir gwdkir.Program) error {
	return asError(validateProgram(config, ir, false))
}

// ValidateProgramReport returns every diagnostic, including warnings, so the
// caller can surface warnings while still gating the build on HasErrors.
func ValidateProgramReport(config gowdk.Config, ir gwdkir.Program) ValidationErrors {
	return validateProgram(config, ir, true)
}

// ValidateSourceProgramReport is the single-source counterpart to
// ValidateProgramReport.
func ValidateSourceProgramReport(config gowdk.Config, ir gwdkir.Program) ValidationErrors {
	return validateProgram(config, ir, false)
}

func asError(report ValidationErrors) error {
	if report.HasErrors() {
		return report
	}
	return nil
}

func validateProgram(config gowdk.Config, ir gwdkir.Program, crossFile bool) ValidationErrors {
	if err := gwdkir.CheckInvariants(ir); err != nil {
		return ValidationErrors{{Message: fmt.Sprintf("internal compiler error: %v", err)}}
	}
	var diagnostics ValidationErrors
	diagnostics = append(diagnostics, validateIRDiagnostics(ir.Diagnostics)...)
	diagnostics = append(diagnostics, validatePackages(ir)...)
	diagnostics = append(diagnostics, validateUniquePages(ir.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentEmits(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentGoContracts(ir.Components)...)
	diagnostics = append(diagnostics, validateComponentStoreUses(ir.Pages, ir.Components)...)
	diagnostics = append(diagnostics, validatePersistedStoreConflicts(ir.Pages)...)
	diagnostics = append(diagnostics, validateRedundantComponents(ir.Components)...)
	diagnostics = append(diagnostics, validateGOWDKUses(ir, crossFile)...)
	diagnostics = append(diagnostics, validatePageAssetUses(ir)...)
	diagnostics = append(diagnostics, validateUniqueLayouts(ir.Layouts)...)
	diagnostics = append(diagnostics, validateLayoutSlots(ir.Layouts)...)
	diagnostics = append(diagnostics, validateLayoutReferences(ir.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(ir.Pages, ir.Layouts)...)
	diagnostics = append(diagnostics, validateGoBlocks(config, ir)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(ir.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(ir.Pages, ir.GoEndpoints, ir.ContractRefs)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(ir.Pages, ir.GoEndpoints, ir.ContractRefs)...)
	diagnostics = append(diagnostics, validateStandaloneEndpoints(ir.GoEndpoints)...)
	diagnostics = append(diagnostics, validateContractReferenceRoutes(ir.ContractRefs)...)
	for _, page := range ir.Pages {
		diagnostics = append(diagnostics, ValidatePage(config, page)...)
	}
	return diagnostics
}

func validateIRDiagnostics(items []gwdkir.Diagnostic) []ValidationError {
	diagnostics := make([]ValidationError, 0, len(items))
	for _, item := range items {
		diagnostics = append(diagnostics, ValidationError{
			Code:    item.Code,
			Source:  item.Source,
			Span:    item.Span,
			Message: item.Message,
		})
	}
	return diagnostics
}
