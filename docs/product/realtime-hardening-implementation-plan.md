# Implementation Plan: Realtime Hardening

## Context

Relevant issue: <https://github.com/cssbruno/GoWDK/issues/635>

Relevant ADRs:

- ADR 0002: compile-first render model.
- ADR 0005: generated Go emission boundary.
- ADR 0006: compiler/runtime boundary.
- ADR 0007: static-first SPA navigation.
- ADR 0008: bounded client language.
- ADR 0012: realtime subscribe surface.

## Assumptions

- Existing SSE retry, replay, audience revocation, client patch versioning, and
  query-refresh code are the baseline.
- This slice should harden route/query refresh without adding source syntax.
- Protected, dynamic, fragment-specific, and API-specific query execution can
  fall back to current-document refresh until their renderer metadata is
  explicit.

## Proposed Changes

- Make `runtime/ssr.RegionRenderer` carry its owning route.
- Make `RenderInvalidatedRegions` use `?path=` when present so refresh patches
  are scoped by route and query type.
- Keep command single-flight query-only rendering only for one unambiguous
  eligible renderer.
- Emit route metadata into generated region registrations.
- Add tests for route-scoped refresh, ambiguous query fallback, generated route
  metadata, and client patch/error behavior already exposed by `gowdk.js`.
- Update docs and roadmap/requirements status if implementation reality changes.

## Files Expected To Change

- `runtime/ssr/regions.go`
- `runtime/ssr/regions_test.go`
- `internal/appgen/source_ssr_regions.go`
- `internal/appgen/appgen_test.go`
- `docs/reference/realtime.md`
- `docs/product/realtime-hardening-spec.md`
- `docs/product/realtime-hardening-implementation-plan.md`

## Data And API Impact

- `ssr.RegionRenderer` carries a `Route` field. Existing callers that omit it
  keep query-only behavior for unambiguous command single-flight refresh.
- Generated apps continue to expose `RealtimeEventsPath` and
  `RealtimeQueryRefreshPath`.
- Public manifest JSON shapes are unchanged.

## Tests

- Unit: `runtime/ssr` route-scoped region rendering.
- Generator: appgen generated source includes route-aware region registration.
- Browser runtime: existing DOM tests cover query-refresh URL, stale/unsupported
  patch rejection, route refresh fallback, and command fallback coordination.
- Integration: focused appgen binary realtime tests.

## Verification Commands

```sh
go test ./runtime/ssr ./runtime/contracts/sse ./internal/clientrt
go test ./internal/appgen -run 'TestGenerate|TestGeneratedBinaryCommandEmbedsSingleFlightRegionHTML|TestGeneratedBinaryRealtime'
go build ./cmd/gowdk
```

## Rollback Plan

- Remove route matching in `runtime/ssr.RenderInvalidatedRegions`.
- Stop emitting `Route` in generated region registrations.
- Revert docs to query-only refresh wording.

## Risks

- Route normalization mismatches could cause patch misses. The current generated
  client sends `window.location.pathname`, matching eligible parameterless route
  declarations.
- Query-only command patches for ambiguous renderers are intentionally skipped;
  users see the existing browser fallback instead of an unsafe wrong-page patch.
