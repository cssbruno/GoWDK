# Implementation Plan: Fast Watch Redeploy

## Context

Relevant spec: `.llm/features/fast-watch-redeploy.md`

## Assumptions

- The first slice uses polling because the current watcher is intentionally
  dependency-free.
- Restarting one generated binary is enough for the current module target model.
- Browser live reload can come later after generated client runtime stabilizes.
- Hashing watched inputs is acceptable for the current first slice because the
  source set is small and discovery already scopes the file list.
- Page-only incremental rendering is safe as a first slice because full manifest
  parsing and validation still run before selective output writes.

## Proposed Changes

- Add `watch --restart`.
- Infer the restart binary from ad hoc `--bin` or one selected/static build
  target with `Binary`.
- Add a small process runner that starts, interrupts, waits, and kills on
  timeout.
- Restart only after successful builds.
- Store watched input content hashes instead of modification times.
- Skip rewriting generated static files, route/asset manifests, generated app
  source, and unchanged embedded static files.
- Remove stale embedded static files when generated app output is synced.
- Add an incremental static render path for existing changed page sources.
- Fall back to full builds for source-set changes, component/layout/CSS/config
  changes, generated app output, binary output, build targets, and restart mode.
- Add parser/inference tests and update docs.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/engineering/operations.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/testing.md`

## Data And API Impact

- No new public Go API.
- Adds CLI flag `gowdk watch --restart`.

## Tests

- Unit: watch option parsing, restart binary inference, and content-hash input
  snapshot/change-diff behavior.
- Integration: focused `cmd/gowdk` tests for configured target inference, plus
  generator tests for no-op write preservation, stale embedded static cleanup,
  incremental static page rendering, fallback classification, and stale route
  output cleanup.
- End-to-end: manual local run of `watch --restart` once a sample binary target
  is configured.
- Manual: not required for CI.

## Verification Commands

```sh
go test ./cmd/gowdk
go test ./internal/staticgen
go test ./internal/appgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the `--restart` flag, process runner, content-hash snapshots, no-op
  write helpers, incremental static rendering, tests, and docs. Existing
  rebuild-only `watch` behavior can fall back to modification-time snapshots.

## Risks

- Process shutdown behavior can vary by platform. Use interrupt first and kill
  after a short timeout.
- Hashing many large inputs can become expensive later; add incremental file
  watching or dependency graph caching once the source set grows.
- Page-local incremental rendering still parses and validates all inputs; future
  work should cache parsed manifests and track page/component/CSS dependency
  edges.
