# Feature Spec: Diagnostic Registry Severity And Fixes

## Problem

The diagnostic registry has stable codes, areas, summaries, and stability
levels, but severity and machine-readable fixes can still drift between CLI,
LSP, and future browser overlay consumers. Existing quick fixes are hardcoded
in the LSP path, and there is no headless `gowdk fix` command for safe
single-file rewrites.

## Goals

- Inventory current public diagnostic codes in one source of truth.
- Mark each code with an area and stability level.
- Mark each code with default severity.
- Store safe fix metadata in the registry.
- Apply registry fixes through `gowdk fix`.
- Serve LSP code actions from registry fix metadata.
- Include severity and fix metadata in JSON/explain outputs.
- Add a test that fails when implementation emits a new code without registry
  coverage.
- Document naming, severity, and stability conventions.

## Non-Goals

- Replace all string literals with constants in this slice.
- Multi-file refactors.
- Guessing rewrites for old endpoint blocks that still contain behavior.
- New diagnostic codes.

## Users And Permissions

- Primary users: CLI users, editor integrations, contributors, and addon
  authors.
- Roles or permissions: none.
- Data visibility rules: no user data impact.

## User Flow

1. A contributor adds or renames a diagnostic code.
2. `go test ./internal/diagnostics` fails if the registry is not updated.
3. The contributor records severity and optional fix metadata in the registry.
4. Users run `gowdk check --json`, `gowdk explain`, LSP quick fixes, or
   `gowdk fix` and see the same registry-backed metadata.

## Requirements

### Functional

- Registry entries include code, area, stability, severity, and summary.
- Stability supports stable, experimental, and addon-owned codes.
- Severity supports error, warning, and info.
- Fix metadata includes a title, description, and named safe rewriter.
- `gowdk fix [--dry-run] [--code <code>]` applies registered single-file
  fixes and rejects ambiguous edits.
- LSP code actions are selected from registry fix metadata rather than
  diagnostic-code switches.
- `gowdk check --json` includes `fix` metadata when a diagnostic has a
  registered fix.
- `gowdk explain` includes severity and available fix metadata.
- `gowdk check --warnings-as-errors` fails when warning diagnostics are
  present.
- Tests scan non-test Go source for emitted diagnostic-code literals.
- Docs link to the registry as the source of truth.

### Non-Functional

- Performance: tests are local source scans only.
- Reliability: registry tests must be deterministic.
- Accessibility: docs use grouped code lists and plain language.
- Security/privacy: no runtime data impact.
- Observability: not applicable.

## Acceptance Criteria

- [x] `internal/diagnostics/registry.go` records severity for every code.
- [x] Registry tests fail on unregistered emitted code literals and invalid
  severity/fix metadata.
- [x] `gowdk fix` migrates empty old endpoint syntax through registry fixes.
- [x] LSP quick fixes for old endpoints and missing use aliases are
  registry-driven.
- [x] `check --json` and `gowdk explain` expose fix metadata.
- [x] Diagnostics docs describe naming, severity, stability, and fixes.
- [ ] Full repository verification passes.

## Edge Cases

- Addons can emit custom `gowdk.GoBlockDiagnostic` codes. The registry covers
  the fallback `addon_go_block_diagnostic`; addon-owned custom codes are
  allowed to remain addon-specific.

## Dependencies

- Internal: parser, compiler, buildgen, contractscan, lang diagnostics.
- External: GitHub issue #75 in milestone M2.

## Open Questions

- Should a later slice move emitted codes from string literals to exported
  constants in `internal/diagnostics`?
- Should browser overlay payloads receive full structured diagnostics instead
  of formatted build-error strings?
