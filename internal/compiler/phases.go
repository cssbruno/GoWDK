package compiler

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// AnalyzedProgram is the output of source lowering plus compiler enrichment.
// It is intentionally distinct from ValidatedProgram: analyzed IR may still
// contain authoring errors and must not be used by emission fast paths.
type AnalyzedProgram struct {
	ir       gwdkir.Program
	bindings []source.BackendBinding
}

// ValidatedProgram is the compiler-owned phase token for IR that passed
// semantic validation and backend binding diagnostics.
type ValidatedProgram struct {
	ir       gwdkir.Program
	bindings []source.BackendBinding
	report   ValidationErrors
	valid    bool
}

// AnalyzeProgram builds and enriches compiler IR from parsed sources.
func AnalyzeProgram(config gowdk.Config, sources gwdkanalysis.Sources) (AnalyzedProgram, error) {
	ir, bindings, err := AssembleProgram(config, sources)
	if err != nil {
		return AnalyzedProgram{}, err
	}
	return AnalyzedProgram{ir: ir, bindings: append([]source.BackendBinding(nil), bindings...)}, nil
}

// AnalyzedProgramFromIR wraps an existing IR value for callers that already own
// the parse/analyze phase. It does not validate the program.
func AnalyzedProgramFromIR(ir gwdkir.Program) AnalyzedProgram {
	return AnalyzedProgram{ir: ir, bindings: BackendBindingsFromIR(ir)}
}

// AnalyzedProgramWithBindings wraps IR with a caller-owned backend binding
// projection for tests and orchestrators that already performed binding.
func AnalyzedProgramWithBindings(ir gwdkir.Program, bindings []source.BackendBinding) AnalyzedProgram {
	return AnalyzedProgram{ir: ir, bindings: append([]source.BackendBinding(nil), bindings...)}
}

// Program returns the underlying compiler IR by value.
func (program AnalyzedProgram) Program() gwdkir.Program {
	return program.ir
}

// BackendBindings returns a copy of the backend binding projection.
func (program AnalyzedProgram) BackendBindings() []source.BackendBinding {
	return append([]source.BackendBinding(nil), program.bindings...)
}

// ValidateAnalyzedProgram validates analyzed IR and returns an opaque phase
// token accepted by generated-output fast paths.
func ValidateAnalyzedProgram(config gowdk.Config, program AnalyzedProgram) (ValidatedProgram, error) {
	validated, report := ValidateAnalyzedProgramReport(config, program)
	if report.HasErrors() {
		return ValidatedProgram{}, report
	}
	return validated, nil
}

// ValidateAnalyzedProgramReport is the reporting form of
// ValidateAnalyzedProgram. Warning-only reports still return a ValidatedProgram.
func ValidateAnalyzedProgramReport(config gowdk.Config, program AnalyzedProgram) (ValidatedProgram, ValidationErrors) {
	report := validateProgram(config, program.ir, true)
	report = append(report, BackendBindingDiagnostics(program.bindings)...)
	report = normalizeValidationErrors(report)
	if report.HasErrors() {
		return ValidatedProgram{}, report
	}
	return ValidatedProgram{
		ir:       program.ir,
		bindings: program.BackendBindings(),
		report:   append(ValidationErrors(nil), report...),
		valid:    true,
	}, report
}

// Valid reports whether this value was returned by compiler validation.
func (program ValidatedProgram) Valid() bool {
	return program.valid
}

// ValidateIR validates an existing compiler IR value and returns the validated
// phase token. New code should prefer AnalyzeProgram when starting from source.
func ValidateIR(config gowdk.Config, ir gwdkir.Program) (ValidatedProgram, error) {
	return ValidateAnalyzedProgram(config, AnalyzedProgramFromIR(ir))
}

// Program returns the validated compiler IR by value.
func (program ValidatedProgram) Program() gwdkir.Program {
	return program.ir
}

// BackendBindings returns a copy of the backend binding projection that was
// validated with this program.
func (program ValidatedProgram) BackendBindings() []source.BackendBinding {
	return append([]source.BackendBinding(nil), program.bindings...)
}

// Report returns the warning-only validation report, if any.
func (program ValidatedProgram) Report() ValidationErrors {
	return append(ValidationErrors(nil), program.report...)
}
