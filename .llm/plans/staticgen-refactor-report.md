# Implementation Plan: Staticgen Refactor And Build Report

## Context

Relevant spec: `.llm/features/static-build-report.md`

`internal/staticgen/staticgen.go` mixes build orchestration, CSS planning,
route/path parsing, runtime asset generation, component registry construction,
manifest writing, and HTML rendering in one large file.

## Assumptions

- Public `staticgen.Build`, `BuildMemory`, and `BuildIncremental` signatures stay
  stable.
- Reports can be added to `Result` without breaking callers.
- CLI debug output should go to stderr so artifact path stdout stays useful for
  scripts.
- No production logging dependency is needed.

## Proposed Changes

- Split `internal/staticgen` into package files by responsibility:
  - `build.go`
  - `types.go`
  - `report.go`
  - `data.go`
  - `css.go`
  - `manifests.go`
  - `runtime_assets.go`
  - `components.go`
  - `render.go`
  - `ssr.go`
- Keep only constants and shared regex declarations in `staticgen.go`.
- Add `BuildReport`, `BuildEvent`, and `BuildError`.
- Populate report events in the three static build entrypoints.
- Emit `gowdk-build-report.json` for successful disk and memory builds.
- Add `--debug` to `gowdk build` and reuse it through forwarded watch/dev build
  args.
- Update docs for build reports and staticgen package layout.

## Files Expected To Change

- `internal/staticgen/*`
- `internal/staticgen/staticgen_test.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `docs/compiler/*`
- `.llm/features/static-build-report.md`
- `.llm/plans/staticgen-refactor-report.md`

## Data And API Impact

- `staticgen.Result` gains `Report BuildReport` and `BuildReportPath string`.
- Staticgen entrypoint errors may wrap `*staticgen.BuildError`; `Error()` and
  `Unwrap()` preserve normal error behavior.
- `gowdk build --debug` prints structured report events as readable text.

## Tests

- Unit:
  - successful build report includes validation, planning, writes, manifests,
    and completion.
  - failed build returns `*BuildError` with an error event.
  - `BuildMemory` report records memory collection rather than disk writes.
- Integration:
  - CLI `build --debug` prints report events to stderr and artifact paths to
    stdout.
- End-to-end:
  - covered by `go test ./...`.
- Manual:
  - optional `go run ./cmd/gowdk build --debug --out /tmp/gowdk-build ...`.

## Verification Commands

```sh
gofmt -w internal/staticgen/*.go cmd/gowdk/*.go
go test ./internal/staticgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Recombine split files if necessary; Go package boundaries are unchanged.
- Remove `Result.Report`, `BuildError`, and `--debug` if the report contract is
  rejected.

## Risks

- A mechanical split can accidentally drop declarations; mitigate with full test
  and build gates.
- Debug output could break scripts if printed to stdout; keep it on stderr.
- Report events can become noisy; keep default CLI output unchanged.
