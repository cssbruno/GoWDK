# Feature Spec: Diagnostic Code Registry

## Problem

M2 needs stable diagnostic names before exact spans, formatter recovery, LSP
explain output, and broader parser diagnostics can be reviewed safely. Codes
are currently emitted as string literals across compiler, parser, build, and
contract paths, and docs can drift from implementation.

## Goals

- Inventory current public diagnostic codes in one source of truth.
- Mark each code with an area and stability level.
- Add a test that fails when implementation emits a new code without registry
  coverage.
- Document naming, severity, and stability conventions.

## Non-Goals

- Replace all string literals with constants in this slice.
- Implement `gowdk explain`.
- Change diagnostic JSON shape.
- Change emitted diagnostic behavior.

## Users And Permissions

- Primary users: CLI users, editor integrations, contributors, and addon
  authors.
- Roles or permissions: none.
- Data visibility rules: no user data impact.

## User Flow

1. A contributor adds or renames a diagnostic code.
2. `go test ./internal/diagnostics` fails if the registry is not updated.
3. The contributor updates the registry and docs in the same change.

## Requirements

### Functional

- Registry entries include code, area, stability, and summary.
- Stability supports stable, experimental, and addon-owned codes.
- Tests scan non-test Go source for emitted diagnostic-code literals.
- Docs link to the registry as the source of truth.

### Non-Functional

- Performance: tests are local source scans only.
- Reliability: registry tests must be deterministic.
- Accessibility: docs use grouped code lists and plain language.
- Security/privacy: no runtime data impact.
- Observability: not applicable.

## Acceptance Criteria

- [ ] `internal/diagnostics/registry.go` exists.
- [ ] Registry tests fail on unregistered emitted code literals.
- [ ] Diagnostics docs describe naming, severity, and stability conventions.
- [ ] Verification passes for diagnostics tests and repository gates.

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
