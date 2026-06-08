# Implementation Plan: Current .gwdk Language Spec

## Context

Spec: `.llm/features/current-gwdk-language-spec.md`

Issue: https://github.com/cssbruno/GoWDK/issues/80

Milestone: M2 - Compiler + Language Contract

## Assumptions

- This is the first M2 vertical slice because it anchors parser, formatter,
  diagnostic, and IR work without changing compiler behavior.
- The language spec should be concise and link to existing detailed docs.
- Behavior that is partial remains labeled partial.

## Proposed Changes

- Add `docs/language/spec.md` as the current language contract overview.
- Link the spec from `docs/language/README.md`.
- Keep `syntax.md`, `grammar.md`, `semantics.md`, and feature pages as the
  deeper references.
- Add a short compatibility policy for deprecations and unsupported syntax.

## Files Expected To Change

- `.llm/features/current-gwdk-language-spec.md`
- `.llm/plans/current-gwdk-language-spec.md`
- `docs/language/spec.md`
- `docs/language/README.md`

## Data And API Impact

- No code, data, API, CLI, or generated-output behavior changes.

## Tests

- Unit: not applicable.
- Integration: not applicable.
- End-to-end: not applicable.
- Manual: read the docs for consistency against README, requirements, roadmap,
  and architecture.

## Verification Commands

```sh
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert the docs/spec additions and README link if the M2 language-spec scope
  needs a different structure.

## Risks

- The spec can drift if future language behavior changes without docs updates.
  AGENTS.md requires docs updates in the same change that makes them stale.
