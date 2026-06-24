# Implementation Plan: Accessibility Diagnostics

## Context

Relevant spec: [accessibility-diagnostics.md](../product/accessibility-diagnostics.md)

Tracking issue: `#638`

## Assumptions

- The compiler should only warn when literal markup makes the issue provable.
- Cross-component internals are a future analysis pass.
- Existing accessibility codes remain stable.

## Proposed Changes

- Extend `internal/lang/accessibility.go` with literal ID/reference collection.
- Add bounded ARIA role/attribute validation and custom interaction checks.
- Add accessible-name, landmark-name, and focusability warnings.
- Register new codes and update diagnostic documentation.
- Cover warning and non-warning cases in language-tool tests.

## Files Expected To Change

- `internal/lang/accessibility.go`
- `internal/lang/tools_test.go`
- `internal/diagnostics/registry.go`
- `docs/reference/diagnostic-codes.md`
- `docs/reference/testing.md`

## Data And API Impact

- No runtime API impact.
- `gowdk check --json` and LSP output can include new warning codes.

## Tests

- Unit: `internal/lang` accessibility diagnostics.
- Integration: existing `gowdk check` JSON coverage.
- End-to-end: none for this compiler-only slice.
- Manual: run `gowdk check` on a page fixture with the new warnings.

## Verification Commands

```sh
go test ./internal/lang ./internal/diagnostics
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the new warning emitters and registry entries. Existing accessibility
  warnings continue to work.

## Risks

- ARIA validation can be noisy if it tries to model dynamic browser behavior.
  Keep the first slice conservative and literal-only.
