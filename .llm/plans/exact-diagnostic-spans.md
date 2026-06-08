# Implementation Plan: Exact Diagnostic Spans

## Context

Relevant spec, issue, ADR, or discussion:

- `.llm/features/exact-diagnostic-spans.md`
- GitHub issue #78, M2 diagnostic span coverage.

## Assumptions

- Existing manifest spans are the source of truth for compiler diagnostics.
- This slice can stay in compiler validation without changing parser APIs.

## Proposed Changes

- Track the repeated occurrence number when `parseRoute` emits a
  `duplicate_route_param` issue.
- Resolve route issue spans by parameter occurrence when provided.
- Add regression tests for page, action, API, and fragment route diagnostics.

## Files Expected To Change

- `internal/compiler/routes.go`
- `internal/compiler/validate_spans.go`
- `internal/compiler/validate_test.go`
- `.llm/features/exact-diagnostic-spans.md`
- `.llm/plans/exact-diagnostic-spans.md`

## Data And API Impact

- No public API or persisted data changes.
- Diagnostic spans become more precise for duplicate route parameters.

## Tests

- Unit: compiler validation route span tests.
- Integration: not required for this slice.
- End-to-end: covered by existing language tooling diagnostics tests.
- Manual: not required.

## Verification Commands

```sh
go test ./internal/compiler -run 'TestValidatePage(RouteDiagnosticsUseExactSpans|RejectsDuplicateRouteParams)$'
go test ./internal/compiler
git diff --check
```

## Rollback Plan

- Revert the occurrence-aware route issue fields and tests.

## Risks

- Parameter occurrence lookup must preserve existing fallback behavior when
  spans are missing.
