# Implementation Plan: Compiler Contract Completion

## Context

Feature spec: `.llm/features/compiler-contract-completion.md`

## Assumptions

- Standalone Go comment action endpoints are not page-owned forms; they do not
  infer `g:post` input schemas or fragments from page markup.
- Same-package build data functions are no-argument exported functions returning
  JSON-encodable structs/maps, matching the imported function contract.

## Proposed Changes

- Add manifest endpoint declarations for non-page endpoint metadata.
- Discover `//gowdk:act` and `//gowdk:api` comments from sibling Go files using
  standard Go parser comment maps.
- Merge comment endpoints into manifest data before validation.
- Bind generated handlers for both page-owned `.gwdk` endpoints and standalone
  Go comment endpoints.
- Replace narrow build data parsing with a small AST-backed declaration parser.
- Add diagnostics and language suggestions for endpoint-comment and build-data
  contract failures.

## Files Expected To Change

- `internal/manifest`
- `internal/compiler`
- `internal/gwdkanalysis`
- `internal/appgen`
- `internal/buildgen`
- `internal/lang`
- `cmd/gowdk`
- Docs and checklist

## Data And API Impact

- `manifest.Manifest` gains standalone endpoint declarations.
- `gwdkir.Endpoint.Source` is now active for both `.gwdk` and Go comment
  endpoint sources.

## Tests

- Unit: endpoint comment discovery, conflict diagnostics, IR source metadata,
  build data parser/evaluator.
- Integration: route metadata and generated app planning from Go comment
  endpoints.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/compiler ./internal/gwdkanalysis ./internal/buildgen ./internal/appgen ./internal/lang ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the manifest endpoint declarations and discovery call sites. Existing
  `.gwdk` endpoint declarations remain unchanged.

## Risks

- Generated handler names for standalone endpoints must remain deterministic and
  collision-free.
- Go comment endpoint discovery must not silently ignore malformed comments.
