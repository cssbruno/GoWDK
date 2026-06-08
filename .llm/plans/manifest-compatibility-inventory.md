# Implementation Plan: Manifest Compatibility Inventory

## Context

Relevant spec, issue, ADR, or discussion:

- `.llm/features/manifest-compatibility-inventory.md`
- GitHub issue #74, M2 compiler-spine migration.

## Assumptions

- `internal/gwdkir.Program` remains the preferred internal handoff for new
  generated-output work.
- Public manifest/site-map JSON compatibility stays until a separate migration
  decision exists.

## Proposed Changes

- Expand `docs/engineering/architecture.md` with a concrete compatibility-user
  table.
- Add a short migration order for reducing manifest dependencies.

## Files Expected To Change

- `docs/engineering/architecture.md`
- `.llm/features/manifest-compatibility-inventory.md`
- `.llm/plans/manifest-compatibility-inventory.md`

## Data And API Impact

- No code, API, generated-output, or persisted data changes.

## Tests

- Unit: not applicable.
- Integration: not applicable.
- End-to-end: not applicable.
- Manual: review rendered Markdown and run whitespace validation.

## Verification Commands

```sh
git diff --check
```

## Rollback Plan

- Revert the documentation commit.

## Risks

- The inventory can drift if future IR migrations do not update the architecture
  table in the same change.
