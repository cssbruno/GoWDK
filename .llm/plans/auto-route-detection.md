# Implementation Plan: Auto Route Detection

## Context

Spec: `.llm/features/auto-route-detection.md`

## Assumptions

- Auto route detection means compiler-owned detection from declared `.gwdk`
  routes, not folder-based route inference.
- API routes remain planned until generated API execution exists.

## Proposed Changes

- Add route-resolution support to `internal/appgen.Options`.
- Resolve action and first-slice SSR routes inside `appgen.GenerateWithOptions`.
- Update `gowdk build --app` to request auto routes instead of manually passing route slices.
- Add appgen tests for auto-detected action/SSR routes and missing manifest input.
- Update generated-output docs and the missing checklist.

## Files Expected To Change

- `internal/appgen/types.go`
- `internal/appgen/appgen.go`
- `internal/appgen/auto_routes.go`
- `internal/appgen/appgen_test.go`
- `cmd/gowdk/main.go`
- `docs/compiler/generated-output.md`
- `MISSING_CHECKLIST.md`

## Data And API Impact

- Adds internal `appgen.Options.AutoRoutes`, `Config`, and `Manifest`.
- No public CLI flag changes.
- No persisted data changes.

## Tests

- Unit: `go test ./internal/appgen`
- Integration: `go test ./cmd/gowdk`
- End-to-end: covered by `go test ./...`
- Manual: not required for this slice.

## Verification Commands

```sh
gofmt -w internal/appgen/types.go internal/appgen/auto_routes.go internal/appgen/appgen.go internal/appgen/appgen_test.go cmd/gowdk/main.go
go test ./cmd/gowdk ./internal/appgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert CLI use of `AutoRoutes`.
- Remove `AutoRoutes`, `Config`, and `Manifest` from `appgen.Options`.
- Keep explicit `Actions` and `SSR` route paths unchanged.

## Risks

- `internal/appgen` now depends on `internal/buildgen` for SSR artifact
  detection. If that coupling becomes too broad, move the resolver to a smaller
  compiler-owned route planning package.
