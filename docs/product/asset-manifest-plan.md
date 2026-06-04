# Implementation Plan: Static Asset Manifest

## Context

Relevant spec: `docs/product/asset-manifest-spec.md`.

## Assumptions

- The first generated asset manifest only records CSS processor assets.
- The logical asset name is the relative emitted CSS path until a richer asset
  naming model exists.
- `gowdk-assets.json` lives next to `gowdk-routes.json` in the output root.

## Proposed Changes

- Extend `runtime/asset.Manifest` with a schema version field.
- Add `gowdk-assets.json` emission to `internal/staticgen`.
- Expose the asset manifest path on `staticgen.Result`.
- Print the generated asset manifest path from `gowdk build`.
- Update docs and the missing-implementation checklist.

## Files Expected To Change

- `runtime/asset/asset.go`
- `runtime/asset/asset_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `cmd/gowdk/main.go`
- `docs/compiler/manifest.md`
- `docs/compiler/generated-output.md`
- `docs/reference/manifest.md`
- `docs/reference/css.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- Generated build output gains `gowdk-assets.json`.
- `staticgen.Result` gains `AssetManifestPath`.
- `runtime/asset.Manifest` gains `Version`.

## Tests

- Unit: runtime asset manifest resolution remains compatible.
- Unit: staticgen writes `gowdk-assets.json` for CSS assets.
- Unit: staticgen writes an empty asset manifest when no CSS assets exist.
- Integration: existing CLI build tests continue to pass with the new artifact.
- End-to-end: no browser behavior in this slice.
- Manual: inspect a generated build output directory.

## Verification Commands

```sh
go test ./runtime/asset
go test ./internal/staticgen
go test ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove `gowdk-assets.json` emission and the `AssetManifestPath` result field.
- Keep CSS processor asset writing unchanged.

## Risks

- Future manifest needs may outgrow the map shape. Versioning keeps room for a
  later schema migration.
