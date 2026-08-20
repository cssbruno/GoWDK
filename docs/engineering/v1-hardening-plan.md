# V1 Contract Hardening Plan

## Context

Implements the contract in
[`docs/product/v1-hardening-spec.md`](../product/v1-hardening-spec.md) for issues
#771, #758, #726, #724, #722, #721, #720, #717, #716, #714, #697, and #695.

## Assumptions

- The current build-iteration example already matches the documented bounded
  build-expression contract; it needs verification coverage, not broader
  syntax.
- Existing public names remain source compatible where a safe migration path
  exists. Pre-v1 magic hooks and inferred directive lanes may become explicit
  migration diagnostics.
- No new production dependency is required.

## Proposed Changes

1. Harden config decoding and introduce immutable project environment overlays.
2. Split and validate the public config surface; add explicit provider refs.
3. Add the recursive CLI schema and multi-module static-analysis scripts.
4. Document and machine-check the v1 language budget; add explicit directive
   lanes and explicit load/guard/auth bindings through AST, IR, diagnostics,
   inspect/LSP, generated output, examples, and migration docs.
5. Make native/WASM packaging read-only, path-independent, atomic, and
   reportable.
6. Bound LSP framing and document retention.
7. Add the catalog-backed coded-error bridge and use it across generated
   request-time lanes.

## Data And API Impact

- New config/provider and runtime coded-error types are additive where possible.
- Generated Go changes registration and error-writing calls but remains normal
  formatted adapter code.
- LSP embedding gains per-server `Limits`; defaults remain internal and finite.
- Build reports gain versioned packaging metadata without secrets or absolute
  cache/work paths.

## Tests

- Unit: config AST failures, overlays, command schema, language budget, directive
  lanes, binding resolution, coded errors, framing, and document accounting.
- Integration: generated app handlers, native/WASM packaging non-mutation and
  failure preservation, CLI help/completions, examples.
- End-to-end: representative generated binary, LSP session, and identical
  builds from different absolute roots in the supported reproducible lane.

## Verification Commands

```sh
go test ./internal/project ./internal/gowdkcmd ./internal/lang ./internal/compiler ./internal/lsp
go test ./internal/buildgen ./internal/appgen ./runtime/...
go build ./cmd/gowdk
scripts/check-static-analysis.sh
scripts/test-go-modules.sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

## Rollback Plan

- Revert each vertical slice independently. Generated artifacts are published
  transactionally, so failed packaging does not require restoring output files.
- Keep migration diagnostics local to the compiler so a rollback does not alter
  user Go packages or persisted data.

## Risks

- Language-lane migration touches many fixtures and generated-output goldens.
- Config compatibility needs strict tests so fail-closed behavior does not turn
  supported executable configs into false errors.
- Reproducibility claims must stay inside the documented Go toolchain/target
  envelope.
