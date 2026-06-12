# Implementation Plan: M4 Go Interop

## Context

Relevant sources:

- GitHub milestone: M4 - Go Interop.
- Issues: #20, #23, #60, #81, #82, #160.
- Spec: `docs/engineering/go-interop-m4-spec.md`.
- Existing spec: `docs/compiler/endpoint-binding-inspection.md`.

## Assumptions

- Existing `internal/compiler` backend binding records are the source of truth
  for action/API/fragment/load binding status.
- Stub generation should start with missing action/API handlers only.
- Typed route-param access is currently stable through `app.Params(ctx)`,
  `app.TypedParams(ctx)`, and `runtime/route` helpers; generated per-route
  structs remain deferred to #23.
- App-wide middleware stays normal `net/http` wrapping of generated
  `Handler()` or `ServeMux()`; route rewriting and response transformation
  hooks remain deferred to #20.

## Proposed Changes

- Add `gowdk inspect go-bindings`.
- Add `gowdk generate stubs`.
- Fix build-data execution to parse stdout only and preserve stderr for failure
  messages.
- Support build helpers returning `(T, error)` by trying the error-return runner
  before falling back to the legacy one-return shape.
- Add a dedicated Go interop reference page and update CLI/language/compiler
  docs.

## Files Expected To Change

- `cmd/gowdk/*`
- `internal/buildgen/build_data_runner.go`
- `internal/buildgen/build_data_routes_test.go`
- `examples/go-interop/*`
- `docs/reference/*`
- `docs/language/*`
- `docs/compiler/*`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/engineering/release-plan.md`

## Data And API Impact

- New CLI report schema: `inspect go-bindings`, version `1`.
- New CLI writer: `generate stubs`.
- No generated app runtime contract change.
- No persisted app data change.

## Tests

- Unit:
  - build helpers with stderr.
  - build helpers returning `(T, error)`.
- Integration:
  - `inspect go-bindings` report over a page with build, load, action, and API
    declarations.
  - `generate stubs` writes normal Go code and subsequent inspection reports
    bound action/API handlers.
- End-to-end:
  - existing release/example gates cover build, reports, and generated app
    smoke paths.
- Manual:
  - run `gowdk inspect go-bindings --ssr` on examples.

## Verification Commands

```sh
go test ./cmd/gowdk ./internal/buildgen -run 'InspectGoBindings|GenerateStubs|BuildUsesImportedGoBuildData|BuildUsesInlineGoBlockGoBuildData|BuildUsesDefaultGoBlockGoBuildData'
go test ./cmd/gowdk ./internal/buildgen ./internal/compiler
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the `generate` command wiring and `inspect go-bindings` target.
- Revert build-data runner to the legacy one-return execution path.
- Revert docs/status updates.

## Risks

- Build-helper fallback adds one compile attempt for legacy one-return helpers.
- Stub generation can create files in packages that still have unrelated Go
  compile errors; the command intentionally does not mask those errors.
- The first report schema is versioned but still experimental in 0.x.
