# Implementation Plan: General Fragment Routes

## Context

Spec: `.llm/features/general-fragment-routes.md`

## Assumptions

- The first standalone fragment route slice is static generated markup.
- Fragment endpoints are backend endpoint metadata, not page route kinds.
- Page guards and optional rate limiting should apply to generated fragment
  endpoints.

## Proposed Changes

- Add fragment endpoint structs to manifest and IR.
- Parse `fragment Name GET "/path" "#target" { ... }` in page and syntax
  parsers.
- Lower fragments through analyzer and expose generated endpoint metadata.
- Add `FragmentEndpoint` to appgen options, route planning, backend adapter IR,
  imports, split proxy detection, and generated handler source.
- Update docs and examples for implemented fragment route support.

## Files Expected To Change

- `internal/parser/*`
- `internal/manifest/*`
- `internal/gwdkast/*`
- `internal/gwdkir/*`
- `internal/gwdkanalysis/*`
- `internal/appgen/*`
- `docs/language/partials.md`
- `docs/product/requirements.md`

## Data And API Impact

- Adds a `.gwdk` syntax form and internal metadata fields.
- Does not add public Go runtime dependencies.

## Tests

- Unit: parser, analyzer, adapter IR, appgen source.
- Integration: generated binary returns a fragment response.
- End-to-end: existing full repo tests.
- Manual: not required for this slice.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/parser ./internal/gwdkanalysis ./internal/appgen
go test ./...
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Remove the fragment declarations, appgen plumbing, docs, and tests.

## Risks

- Syntax could conflict with future user-owned fragment handler semantics.
- Static fragment body rendering is intentionally narrower than full component
  expansion or request-time fragment data.
