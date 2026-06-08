# Feature Spec: Diagnostic Explain Command

## Problem

Diagnostic codes are now registered, but users still need to know what a code
means and what to do next without searching source or docs manually.

## Goals

- Add `gowdk explain <diagnostic-code>`.
- Support `--json` for editor and tooling integrations.
- Explain code area, stability, summary, details, next steps, and examples when
  available.
- Return non-zero for unknown codes with close-code suggestions.

## Non-Goals

- Replace full diagnostics documentation.
- Add warnings or new diagnostic schema fields.
- Provide detailed examples for every code in the first slice.

## Users And Permissions

- Primary users: CLI users, editor integrations, contributors, and addon
  authors.
- Roles or permissions: none.
- Data visibility rules: no source files are read by this command.

## User Flow

1. A user sees a diagnostic code from `gowdk check --json`.
2. They run `gowdk explain missing_ssr_addon`.
3. The CLI prints what the code means and the next practical fix.

## Requirements

### Functional

- Plain-text output is human-readable.
- JSON output includes stable field names.
- Unknown codes fail with suggestions.
- Initial detailed explanations cover common stable parser/render/page codes.

### Non-Functional

- Performance: registry lookup is in-memory.
- Reliability: command does not depend on project config or source discovery.
- Accessibility: plain-text output uses short sections and bullets.
- Security/privacy: command reads no user source files.
- Observability: not applicable.

## Acceptance Criteria

- [ ] `gowdk explain <code>` works.
- [ ] `gowdk explain --json <code>` works.
- [ ] Unknown codes return non-zero with suggestions.
- [ ] CLI and docs are updated.
- [ ] Tests cover text, JSON, and unknown-code paths.

## Edge Cases

- Addon-specific custom diagnostic codes can still be unknown to the core
  registry. The command should suggest close core codes but cannot explain
  external addon codes until the addon exposes registry metadata.

## Dependencies

- Internal: `internal/diagnostics` registry.
- External: GitHub issue #77 in milestone M2.

## Open Questions

- Should addons be able to register explanation metadata for custom codes?
