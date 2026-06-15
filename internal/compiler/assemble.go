package compiler

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// EnrichProgram runs the IR enrichment phases that every codegen orchestrator
// needs, in the one order they must run: discover standalone Go endpoints, then
// bind backend handlers. It mutates program in place and returns the backend
// bindings.
//
// This is the single definition of the discover -> bind phase order. Codegen
// orchestrators must not re-sequence these phases by hand; doing so is how the
// source-taking buildgen entrypoints previously produced under-enriched IR (no
// discovered Go endpoints, empty bindings) by skipping these steps entirely.
//
// Contract linking and validation are deliberately left to the caller: they
// require a project root and a contract-scan report, and each orchestrator has
// a different error-handling contract (the CLI fails hard; the LSP/check path
// collects diagnostics).
func EnrichProgram(config gowdk.Config, program *gwdkir.Program) ([]source.BackendBinding, error) {
	if err := DiscoverGoEndpoints(config, program); err != nil {
		return nil, err
	}
	return BindBackendHandlers(program), nil
}

// AssembleProgram builds the canonical compiler IR from parsed sources and runs
// EnrichProgram on it, returning the enriched program together with its backend
// bindings. It is the single entrypoint for callers that start from sources
// rather than an already-assembled program, so they cannot accidentally build
// on a base program that skipped discovery and binding.
func AssembleProgram(config gowdk.Config, sources gwdkanalysis.Sources) (gwdkir.Program, []source.BackendBinding, error) {
	program := gwdkanalysis.BuildProgram(config, sources)
	bindings, err := EnrichProgram(config, &program)
	return program, bindings, err
}
