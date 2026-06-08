# Feature Spec: Manifest Compatibility Inventory

## Problem

M2 needs downstream generation paths to move toward `internal/gwdkir.Program`,
but the repository still has legitimate manifest compatibility users. Without a
current inventory, migration work can either remove compatibility too early or
add new manifest dependencies in long-term generation code.

## Goals

- List the remaining manifest compatibility users by subsystem.
- State which uses are public compatibility, transitional compiler plumbing, or
  test fixture convenience.
- Define the migration order for removing long-term generation dependencies.

## Non-Goals

- Remove manifest usage in this slice.
- Change generated output, CLI JSON, or public Go APIs.
- Rewrite buildgen or appgen entrypoints.

## Users And Permissions

- Primary users: compiler contributors and release maintainers.
- Roles or permissions: none.
- Data visibility rules: no user data impact.

## User Flow

1. A contributor starts an IR migration change.
2. They check the architecture compatibility table.
3. They update the listed owner or add an explicit compatibility note when a
   manifest dependency remains.

## Requirements

### Functional

- Architecture docs identify concrete remaining manifest users.
- Docs distinguish source-of-truth IR paths from compatibility adapters.
- Migration order is explicit enough to split future commits.

### Non-Functional

- Performance: documentation-only.
- Reliability: avoids hidden migration assumptions.
- Accessibility: table format is concise and scannable.
- Security/privacy: no runtime data impact.
- Observability: not applicable.

## Acceptance Criteria

- [x] Remaining manifest compatibility users are listed.
- [x] The preferred new generated-output boundary is stated.
- [x] Migration order is documented.

## Edge Cases

- Tests may continue constructing manifest records until their subject under
  test has a native IR helper.

## Dependencies

- Internal: `internal/gwdkir`, `internal/manifest`, compiler generation paths.
- External: GitHub issue #74 in milestone M2.

## Open Questions

- Which public CLI JSON outputs should remain manifest-shaped after the internal
  pipeline is fully IR-backed?
