# Implementation Plan: Bounded Await Blocks

## Context

Relevant issue: https://github.com/cssbruno/GoWDK/issues/502

## Assumptions

- This PR completes the bounded `fetchJSON[T](urlExpr)` form for client islands.
- `g:await`, `g:async`, arbitrary promises, and value-returning async helpers
  remain out of scope.

## Proposed Changes

- Add an `AwaitBlock` view node and parser support for `{#await}` branches.
- Render await blocks as `gowdk-await` markers with pending/then/catch
  templates inside JS islands.
- Add runtime fetch execution, cancellation, branch swapping, and re-binding.
- Update diagnostics and language docs to describe the supported slice.

## Files Expected To Change

- `internal/viewmodel/`
- `internal/viewparse/`
- `internal/viewrender/`
- `internal/clientlang/`
- `internal/clientrt/assets/island.js`
- `docs/language/`
- focused tests under `internal/viewrender`, `internal/clientrt`, and
  `internal/buildgen` as needed.

## Data And API Impact

- Adds generated `gowdk-await` markup and `data-gowdk-await-*` attributes inside
  JS islands.
- Does not change Go public APIs or endpoint contracts.

## Tests

- Unit: parser/model/render validation and diagnostics.
- Integration: generated runtime snippets.
- End-to-end: browser island await success/error behavior when feasible.
- Manual: none expected.

## Verification Commands

```sh
go test ./internal/viewrender ./internal/clientrt ./internal/buildgen ./internal/compiler ./internal/lang
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the parser/model/render/runtime changes and restore the unsupported
  await diagnostic text.

## Risks

- Nested branch content must be rebound after swaps.
- Stale fetch results must not update detached or superseded await blocks.
- Await-generated islands must mount even when a component has no explicit
  state or event handlers.
