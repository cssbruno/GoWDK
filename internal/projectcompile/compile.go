// Package projectcompile owns the shared parse-independent project compiler
// pipeline used by the CLI, dev server, language tools, and future embedders.
package projectcompile

import (
	"context"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/contractscan"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type Mode uint8

const (
	ProjectMode Mode = iota
	SourceMode
)

type Options struct {
	ProjectRoot   string
	Mode          Mode
	ScanContracts bool
}

// Snapshot is one immutable-by-convention compilation result. Every consumer
// receives the same analyzed and validated phase values rather than rebuilding
// or relinking IR in a command-specific order.
type Snapshot struct {
	Analyzed  compiler.AnalyzedProgram
	Validated compiler.ValidatedProgram
	Contracts contractscan.Report
}

type Diagnostic struct {
	Stage    string
	Code     string
	Source   string
	Line     int
	Column   int
	Span     source.SourceSpan
	Related  []source.RelatedSpan
	Severity string
	Message  string
}

type Diagnostics []Diagnostic

func (items Diagnostics) Error() string {
	messages := make([]string, 0, len(items))
	for _, item := range items {
		messages = append(messages, item.Message)
	}
	return strings.Join(messages, "\n")
}

func (items Diagnostics) HasErrors() bool {
	for _, item := range items {
		if item.Severity == "" || item.Severity == "error" {
			return true
		}
	}
	return false
}

// Compile runs the canonical analyze -> contract link -> validate sequence.
func Compile(config gowdk.Config, sources gwdkanalysis.Sources, options Options) (Snapshot, Diagnostics, error) {
	return CompileContext(context.Background(), config, sources, options)
}

// CompileContext is Compile with cancellation between every owning phase.
func CompileContext(ctx context.Context, config gowdk.Config, sources gwdkanalysis.Sources, options Options) (Snapshot, Diagnostics, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, nil, err
	}
	analyzed, err := compiler.AnalyzeProgram(config, sources)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("analyze project: %w", err)
	}
	ir := analyzed.Program()
	snapshot := Snapshot{Analyzed: analyzed}
	var diagnostics Diagnostics

	if options.ScanContracts {
		if err := ctx.Err(); err != nil {
			return snapshot, diagnostics, err
		}
		root := strings.TrimSpace(options.ProjectRoot)
		if root == "" {
			root = "."
		}
		report, scanErr := contractscan.Scan(root)
		if scanErr != nil {
			return snapshot, diagnostics, fmt.Errorf("scan Go contracts: %w", scanErr)
		}
		snapshot.Contracts = report
		for _, item := range report.Diagnostics {
			diagnostics = append(diagnostics, Diagnostic{Stage: "contracts", Code: item.Code, Source: item.Source, Line: item.Line, Column: item.Column, Severity: item.Severity, Message: item.Message})
		}
		ir.ContractRefs = contractscan.LinkReferences(ir.ContractRefs, report)
		ir.RealtimeSubscriptions = contractscan.LinkRealtimeSubscriptions(ir.RealtimeSubscriptions, report)
		ir.QueryInvalidations = contractscan.LinkQueryInvalidations(ir.ContractRefs, report)
		diagnostics = appendCompilerDiagnostics(diagnostics, "contracts", compiler.ValidateContractReferences(ir.ContractRefs))
		diagnostics = appendCompilerDiagnostics(diagnostics, "contracts", compiler.ValidateRealtimeSubscriptionBindings(ir.RealtimeSubscriptions))
		diagnostics = appendCompilerDiagnostics(diagnostics, "contracts", compiler.ValidateQueryInvalidations(config, ir.QueryInvalidations))
		analyzed = compiler.AnalyzedProgramWithBindings(ir, analyzed.BackendBindings())
		snapshot.Analyzed = analyzed
	}

	var report compiler.ValidationErrors
	if err := ctx.Err(); err != nil {
		return snapshot, diagnostics, err
	}
	if options.Mode == SourceMode {
		report = compiler.ValidateSourceProgramReport(config, ir)
		report = append(report, compiler.BackendBindingDiagnostics(analyzed.BackendBindings())...)
	} else {
		snapshot.Validated, report = compiler.ValidateAnalyzedProgramReport(config, analyzed)
	}
	for _, item := range report {
		severity := "error"
		if item.Severity == compiler.SeverityWarning {
			severity = "warning"
		}
		diagnostics = append(diagnostics, Diagnostic{Stage: "validate", Code: item.Code, Source: item.Source, Line: item.Span.Start.Line, Column: item.Span.Start.Column, Span: item.Span, Related: append([]source.RelatedSpan(nil), item.Related...), Severity: severity, Message: item.Error()})
	}
	return snapshot, diagnostics, nil
}

func appendCompilerDiagnostics(diagnostics Diagnostics, stage string, err error) Diagnostics {
	if err == nil {
		return diagnostics
	}
	report, ok := err.(compiler.ValidationErrors)
	if !ok {
		return append(diagnostics, Diagnostic{Stage: stage, Severity: "error", Message: err.Error()})
	}
	for _, item := range report {
		severity := "error"
		if item.Severity == compiler.SeverityWarning {
			severity = "warning"
		}
		diagnostics = append(diagnostics, Diagnostic{
			Stage: stage, Code: item.Code, Source: item.Source,
			Line: item.Span.Start.Line, Column: item.Span.Start.Column,
			Span: item.Span, Related: append([]source.RelatedSpan(nil), item.Related...), Severity: severity, Message: item.Error(),
		})
	}
	return diagnostics
}

func Program(snapshot Snapshot) gwdkir.Program { return snapshot.Analyzed.Program() }
