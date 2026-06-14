---
name: gowdk-compiler-internal
description: Change GOWDK compiler internals without changing public language behavior. Use for AST shape, analyzer passes, IR (`gwdkir`) structure and invariants, lowering, diagnostics registry plumbing, validation order, or compiler performance work.
---

# GOWDK Compiler Internal

Internal contracts are pinned by goldens and invariant checks. Change them
intentionally, never as a side effect.

## Baselines

- Pipeline: `internal/parser` (AST: `gwdkast.File`, `PageDecl`,
  `ComponentDecl`, `LayoutDecl`, `Block`, `Endpoint`) →
  `gwdkanalysis.BuildProgram(config, sources)` in
  `internal/gwdkanalysis/ir_builder.go` (lowering; helpers in
  `ir_bindings.go`, `ir_contracts.go`, `routes.go`) →
  `compiler.DiscoverGoEndpoints` → `compiler.BindBackendHandlers` →
  `compiler.ValidateProgramReport` (CLI build path, `cmd/gowdk/build.go`;
  the `internal/lang/tools.go` editor path validates before binding).
- IR root type: `gwdkir.Program` (Pages, Components, Layouts, Routes with
  `RouteStatic`/`RouteSPA`/`RouteSSR`/`RouteHybrid`, Endpoints, GoEndpoints,
  Templates, ContractRefs, ClientBehaviors, Assets, Diagnostics).
- Structural invariants: `CheckInvariants(program)` in
  `internal/gwdkir/invariants.go` — deterministic ordering, closed enums,
  cross-slice references, and required template bodies. It does NOT cover
  user-facing problems (duplicates/conflicts); those belong in
  `internal/compiler` validators.
- Diagnostics registry: `internal/diagnostics/registry.go`, single
  `Registry = []Code{...}` with snake_case codes, `Area`,
  `Stability{Stable,Experimental,Addon}`, `Severity{Error,Warning,Info}`,
  optional `Fix`. `go test ./internal/diagnostics` scans source for emitted
  codes and fails on unregistered ones. Machine-applied fixes live in
  `internal/diagnosticfix/fix.go` (`Edits(fix, source, diagnostic)`).
- Goldens are static and hand-updated: `internal/parser/testdata/golden/`,
  `internal/appgen/testdata/generated_go_golden/app.go.golden`,
  `internal/buildgen/testdata/`.
- Architecture and phase docs: `docs/engineering/architecture.md`,
  `docs/compiler/pipeline.md`.

## Core Workflow

1. Read `docs/compiler/pipeline.md` and the package you are changing before
   editing.
2. Confirm the change has no public syntax, semantics, or generated-output
   impact. If it does, use `gowdk-language-change` or `gowdk-generated-output`
   instead.
3. When changing IR shape: update `gwdkir` types, `CheckInvariants`, the
   lowering in `gwdkanalysis`, and every consumer (`compiler`, `buildgen`,
   `appgen`, `lsp`) in the same change.
4. Update golden files deliberately — a golden diff is a reviewed contract
   change, not noise to regenerate blindly.
5. Run the focused packages first, then the downstream consumers:

```bash
go test ./internal/gwdkast ./internal/gwdkanalysis ./internal/gwdkir
go test ./internal/compiler ./internal/diagnostics ./internal/diagnosticfix
go test ./internal/buildgen ./internal/appgen ./internal/lsp
go build ./cmd/gowdk && go run ./cmd/gowdk inspect ir examples/pages/home.page.gwdk
```

## Guardrails

- Keep diagnostic codes and messages stable; route any change through the
  registry and `docs/reference/diagnostic-codes.md`.
- Keep IR output deterministic (ordering invariants are checked).
- Do not let generators reach around the IR to read source or AST directly —
  the IR is the only handoff (see `docs/engineering/decisions/0006`).
- Keep data flow easy to trace; no new factories/registries/indirection until
  complexity proves the need.

## Report

Name the IR types, passes, or invariants that changed, which goldens were
updated and why, and which tests prove downstream consumers still work.
