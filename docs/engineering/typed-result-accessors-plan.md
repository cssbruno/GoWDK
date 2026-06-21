# Implementation Plan: Typed Result Accessors

## Context

Spec: `docs/product/typed-result-accessors.md`

Issue: `cssbruno/GoWDK#509`

## Assumptions

- The first stable slice should cover SSR load data because `server {}` already
  declares field paths in page source.
- Action-result accessors need a separate result contract; current actions
  return `runtime/response.Response`.

## Proposed Changes

- Extend backend binding metadata with result type and result fields.
- Classify typed load signatures from `go/types` and inline `go server {}` AST.
- Validate typed `server {}` declarations against result field metadata.
- Generate direct struct-to-map SSR adapter glue for full page loads and
  standalone region renderers.
- Expose result metadata through inspect/manifest JSON.

## Files Expected To Change

- `internal/source`, `internal/gwdkir`, `internal/gwdkanalysis`
- `internal/compiler`
- `internal/appgen`
- `internal/lang`
- `docs/product`, `docs/engineering`, `docs/language`, `docs/reference`,
  `docs/compiler`

## Data And API Impact

- Adds `load_struct` and `load_struct_error` backend signature kinds.
- Adds result type and result field metadata to backend binding JSON.
- Keeps existing `map[string]any` load behavior compatible.

## Tests

- Unit: typed load signature classification and result-field validation.
- Integration: generated SSR app source assertions.
- End-to-end: generated binary serves typed load struct data over HTTP.
- Manual: run focused tests and full repository gates.

## Verification Commands

```sh
go test ./internal/compiler ./internal/appgen
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the typed load signature kinds and result metadata.
- Existing map-based SSR load handlers continue to work without migration.

## Risks

- Field naming can surprise users without `json` tags; docs state that untagged
  fields use the Go exported field name.
- The action-result portion remains intentionally deferred until the action
  response contract can expose typed data without overloading `Response`.
