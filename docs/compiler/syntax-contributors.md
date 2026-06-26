# Syntax Contributor Checklist

Use this checklist when adding or changing `.gwdk` syntax, compiler metadata,
diagnostics, generated output, or editor behavior. Keep each change scoped to one
language contract.

## Required Path

1. Update the source contract first:
   - `docs/language/` for public syntax or semantics.
   - `docs/reference/diagnostic-codes.md` for new or changed diagnostic codes.
   - `docs/compiler/generated-output.md` for generated artifact shape changes.
2. Update the AST and parser:
   - `internal/gwdkast` for source nodes and spans.
   - `internal/parser` for parsing and lowering into `internal/gwdkir`.
   - Parser goldens under `internal/parser/testdata/golden/` when the AST
     contract changes.
3. Update IR and invariants:
   - `internal/gwdkir` type fields.
   - `gwdkir.CheckInvariants` for closed enums, ordering, and cross-slice
     references.
   - `internal/gwdkanalysis` lowering and ordering.
   - `gowdk inspect ir` goldens when the public debug shape changes.
4. Update validation and diagnostics:
   - `internal/compiler` for semantic checks.
   - `internal/diagnostics/registry.go` for every emitted diagnostic code.
   - Source spans should target the smallest useful declaration or token.
5. Update generated-output consumers when needed:
   - `internal/buildgen` for SPA/build-output, CSS/assets, SSR artifacts, and
     build reports.
   - `internal/appgen` for generated Go adapters, route registrations, action,
     API, fragment, contract, guard, rate-limit, SSR, and backend output.
   - Generated Go must use Go AST/printer/format as described in
     `docs/engineering/generated-code-policy.md`.
6. Update language tooling:
   - `internal/lang` for formatting, manifest/site-map JSON, completions, and
     diagnostics output.
   - `internal/lsp` and `editors/vscode` when editor features need the new
     syntax.
7. Add focused tests:
   - Parser/AST: `go test ./internal/parser`.
   - Diagnostics/validation: `go test ./internal/compiler ./internal/diagnostics`.
   - Generated output: `go test ./internal/buildgen ./internal/appgen`.
   - LSP/editor: `go test ./internal/lsp` plus editor checks when touched.
   - CLI report changes: update `internal/gowdkcmd/testdata/*_golden` and run
     `go test ./cmd/gowdk`.
8. Add a conformance corpus case:
   - Accepted syntax: an `accept/` file under
     `internal/lang/testdata/conformance/` that exercises it.
   - A rejection or new diagnostic: a `reject/` file with a leading
     `// expect: <code>` directive. See `docs/language/conformance.md`.

## Guardrails

- Do not add `@` metadata syntax back as canonical public syntax.
- Dynamic SPA routes still require `paths {}` unless a page uses request-time
  SSR.
- Actions, APIs, and fragments remain endpoint metadata, not page render modes.
- Generated JavaScript is enhancement only; it must not own routing truth,
  auth, server validation, business logic, server state, or cache policy.
- Public generated-output shape changes need docs and deterministic tests in the
  same change.

## Minimum Handoff

Before handing off, state:

- The syntax or behavior changed.
- The AST/IR fields and diagnostics touched.
- The generated files, JSON shapes, or reports affected.
- The docs and examples updated.
- The exact verification commands run.
