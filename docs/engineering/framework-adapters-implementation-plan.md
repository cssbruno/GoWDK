# Implementation Plan: Framework Adapter Modules

## Context

Relevant issues: #166, #167, #168, and tracking issue #170.
Related foundation: #165 OpenAPI export from compiler IR.

## Assumptions

- Adapter packages live under `runtime/adapters/*`, matching current docs and
  dependency policy.
- Generated apps continue to expose `Handler()` / `ServeMux()` and no
  framework-specific generated code.
- OpenAPI paths are the conformance input for routable web surfaces.

## Proposed Changes

- Add dependency-free `runtime/adapters` route/spec helpers.
- Add nested `runtime/adapters/chi` module.
- Add route-aware `MountRoutes` and `MountOpenAPI` APIs to Echo and Gin.
- Add mount-prefix stripping and local generated URL rebasing for route-aware
  adapters.
- Add fallback mounting for page and asset routes that are not present in
  OpenAPI.
- Add original GOWDK route patterns to OpenAPI `x-gowdk.route` so rest params
  can be preserved by route-aware adapters.
- Add a shared conformance helper used by Chi, Echo, and Gin tests.
- Return clear Gin mount-time errors for ambiguous host route patterns.

## Files Expected To Change

- `runtime/adapters/`
- `runtime/adapters/chi/`
- `runtime/adapters/echo/`
- `runtime/adapters/gin/`
- `scripts/go-modules.sh`
- `docs/reference/framework-integrations.md`
- `docs/engineering/dependency-policy.md`
- `docs/engineering/testing.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- New public helper package: `github.com/cssbruno/gowdk/runtime/adapters`.
- New nested module: `github.com/cssbruno/gowdk/runtime/adapters/chi`.
- Echo and Gin keep existing `Mount` helpers and add `MountOpenAPI`,
  `MountRoutes`, and `WithPrefix`.
- Root module dependency graph is unchanged.

## Tests

- Unit: route extraction, prefix handling, route-pattern translation.
- Integration: adapter conformance tests for Chi, Echo, and Gin, including
  page/asset fallback mounting with non-empty OpenAPI reports.
- End-to-end: `scripts/test-go-modules.sh`.
- Manual: build and mount an example generated handler in a host framework app
  when preparing release docs.

## Verification Commands

```sh
go test ./runtime/adapters
(cd runtime/adapters/chi && go test ./...)
(cd runtime/adapters/echo && go test ./...)
(cd runtime/adapters/gin && go test ./...)
scripts/test-go-modules.sh
go build ./cmd/gowdk
```

## Rollback Plan

- Remove `runtime/adapters/chi`.
- Remove route-aware APIs from Echo/Gin while keeping the existing catch-all
  adapters.
- Remove Chi from `scripts/go-modules.sh`.
- Revert docs to describe catch-all mounting only.

## Risks

- OpenAPI path syntax does not preserve GOWDK rest-param details; route-aware
  APIs that receive direct `Route` metadata can translate rest params.
- Gin route conflict detection may reject some host-pattern combinations before
  Gin panics. That is intentional for clear mount-time failures.
