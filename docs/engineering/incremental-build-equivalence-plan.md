# Implementation Plan: Incremental Build Equivalence

## Context

Spec: [Incremental Build Equivalence](incremental-build-equivalence-spec.md)

Issue: [#686](https://github.com/cssbruno/GoWDK/issues/686)

## Assumptions

- The first checked-in slice can live in `internal/buildgen` because current
  incremental SPA generation is implemented there.
- `gowdk-build-report.json` records progress-path information by design, so the
  equivalence helper compares artifact-relevant report events and exact bytes
  for every other file.

## Proposed Changes

- Add a reusable scenario helper for clean-versus-incremental output tree
  comparison.
- Compare served output plus sibling compiler report output.
- Add deterministic multi-step coverage for route moves, component changes,
  content-hashed assets, and layout changes.
- Remove stale asset-manifest files during incremental output publication.

## Files Expected To Change

- `internal/buildgen/equivalence_test.go`
- `internal/buildgen/build.go`
- `internal/buildgen/manifests.go`
- `docs/engineering/testing.md`
- `docs/engineering/incremental-build-equivalence-spec.md`
- `docs/engineering/incremental-build-equivalence-plan.md`

## Data And API Impact

- No public API changes.
- Incremental builds now remove stale generated asset files that were present in
  the previous asset manifest but are absent from the current generated asset
  set.

## Tests

- Unit: focused `internal/buildgen` equivalence scenario.
- Integration: existing generated-output determinism script remains separate and
  continues to cover clean-build determinism.
- End-to-end: generated app equivalence remains a follow-up.
- Manual: none.

## Verification Commands

```sh
go test ./internal/buildgen -run 'TestIncrementalBuildMatchesCleanBuildEquivalence|TestBuildIncremental' -count=1
go test ./internal/buildgen -count=1
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the stale asset cleanup and equivalence test if incremental publication
  needs to return to previous behavior.
- Existing clean build output remains unaffected by this slice.

## Risks

- The current report projection could miss a future artifact-relevant build
  report event. Add such event kinds to the projection with the generator change
  that introduces them.
- The deterministic matrix is not yet broad enough for generated app, split
  target, backend endpoint, config, or WASM output equivalence.
