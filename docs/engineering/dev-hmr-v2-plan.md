# Implementation Plan: Development Update Protocol V2

## Context

Implements `../product/dev-hmr-v2-spec.md` and GitHub issue #639.

## Assumptions

- Static SPA routes remain real HTML URLs.
- Full body replacement is acceptable only after compatibility checks and a
  fresh-document fetch.
- WASM state is opaque and is never transferred.

## Proposed Changes

- Extend dev update payloads and dependency attribution for v2.
- Emit page/store compatibility markers in generated development HTML.
- Add browser patch logic for managed head/body replacement, state carryover,
  island cleanup/remount, focus restoration, and one-shot reload fallback.
- Route page/layout/component changes to patch/remount/reload decisions.
- Extend browser and Go tests; update dev and product requirement docs.

## Files Expected To Change

- `internal/gowdkcmd/dev_loop.go`, `internal/gowdkcmd/serve.go`
- `internal/buildgen` development markers/runtime tests
- `docs/reference/dev.md`, `docs/product/requirements.md`

## Data And API Impact

- Dev-only SSE protocol advances from version 1 to version 2.
- Production manifest and runtime contracts do not change.

## Tests

- Unit: payload decisions, dependency attribution, route scoping.
- Integration: stale output cleanup and last-good output after failure.
- End-to-end: browser patch/remount/reload and production equivalence.
- Manual: run `gowdk dev` against page/layout/JS/WASM changes.

## Verification Commands

```sh
go test ./internal/gowdkcmd ./internal/buildgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert emitted updates to version 1 component remount/full reload behavior;
  production output is unaffected.

## Risks

- Replacing body content can lose focus or event listeners if cleanup/remount
  ordering is wrong.
- Over-broad state preservation can retain values across incompatible shapes.
