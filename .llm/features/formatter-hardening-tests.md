# Feature Spec: Formatter Hardening Tests

## Problem

The current formatter is intentionally simple and line-oriented. M2 needs tests
that keep its supported behavior predictable before broader parser recovery or
parser-backed formatting work begins.

## Goals

- Add idempotence coverage for supported page, component, and endpoint shapes.
- Preserve current comment behavior where comments are supported.
- Prove formatting malformed source does not hide parser diagnostics.
- Document unsupported formatting cases.

## Non-Goals

- Replace the formatter with a parser-backed formatter.
- Change formatter output.
- Implement broad malformed-syntax recovery.

## Users And Permissions

- Primary users: contributors and early users running `gowdk fmt`.
- Roles or permissions: none.
- Data visibility rules: no user data impact.

## User Flow

1. A contributor changes formatter behavior.
2. Idempotence tests fail if a supported shape changes unexpectedly.
3. Parser diagnostic tests fail if formatting hides migration diagnostics.

## Requirements

### Functional

- `Format(Format(source))` equals `Format(source)` for supported shapes.
- Comments remain covered by existing golden tests.
- Old action/API syntax remains diagnosable after formatting.
- Docs state unsupported formatter cases.

### Non-Functional

- Performance: tests operate on small in-memory fixtures.
- Reliability: tests are deterministic and do not read project config.
- Accessibility: docs use direct bullets.
- Security/privacy: no runtime impact.
- Observability: not applicable.

## Acceptance Criteria

- [ ] Formatter idempotence tests exist.
- [ ] Malformed syntax remains diagnosable after formatting.
- [ ] Unsupported formatting cases are documented.
- [ ] Verification passes for `internal/lang` and repository gates.

## Edge Cases

- Current formatter counts braces without parsing. This remains documented as
  unsupported behavior rather than hidden behind brittle tests.

## Dependencies

- Internal: `internal/lang/format.go`, parser diagnostics.
- External: GitHub issue #79 in milestone M2.

## Open Questions

- Which source surfaces should be promoted to parser-backed formatting first?
