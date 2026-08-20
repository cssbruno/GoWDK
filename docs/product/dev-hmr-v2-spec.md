# Feature Spec: Development Update Protocol V2

## Problem

The current dev-update v1 protocol preserves compatible JavaScript island
state only for mapped component edits. Page, layout, source-set, WASM, and
broader dependency changes reload the entire document, and the protocol does
not describe patch/remount compatibility or stale cleanup explicitly.

## Goals

- Patch compatible page and layout generations without stale DOM/head state.
- Preserve page-store and JavaScript island state only across compatible
  shapes.
- Remount WASM islands without transferring opaque WASM state.
- Fall back to one deterministic reload for unsupported or unattributed edits.
- Keep all HMR behavior development-only.

## Non-Goals

- Production hydration or a browser-owned routing model.
- State transfer across incompatible schemas or WASM instances.
- HMR for generated runtime/ABI changes.

## Users And Permissions

- Primary users: developers running `gowdk dev`.
- Roles or permissions: local development only.
- Data visibility rules: update payloads contain generated route/component
  identifiers and shape hashes, not application data.

## User Flow

1. A successful incremental rebuild attributes changed inputs to routes.
2. The server emits a v2 update with a patch, remount, or reload decision.
3. The browser checks protocol and state-shape compatibility, fetches the fresh
   document, synchronizes managed head/body content, cleans old islands, and
   remounts current JavaScript/WASM roots.
4. Any failed check performs one full reload.

## Requirements

### Functional

- Version 2 payloads name `patch`, `component-remount`, or `reload` actions,
  affected routes, preservation policy, and compatibility boundaries.
- Page and layout patches replace stale body and managed head metadata.
- Compatible page-store and JavaScript island state is carried forward.
- Incompatible stores/islands remount from fresh seeds or reload as declared.
- WASM roots remount without state transfer.
- Added/removed/renamed components and generated assets disappear after the
  committed rebuild.
- Imported components, component CSS/assets/stores, and layouts participate in
  dependency attribution where the compiler IR provides ownership.
- Build failures keep the last committed output and use the existing overlay.

### Non-Functional

- Performance: one fresh-document fetch per patch update.
- Reliability: a patch failure triggers exactly one reload.
- Accessibility: focus is restored by stable element ID/name when possible.
- Security/privacy: HMR stays injected by the dev server and is absent from
  production assets.
- Observability: stable `gowdk:dev-update`, `gowdk:component-hmr`, and
  `gowdk:page-hmr` events describe outcomes.

## Acceptance Criteria

- [x] Compatible component, page, and layout edits preserve documented state.
- [x] Incompatible/unattributed edits reliably reload once.
- [x] Removed DOM, head metadata, components, and assets do not remain active.
- [x] WASM roots rebuild/remount without state transfer.
- [x] Browser tests cover preservation, incompatible shapes, cleanup,
  navigation, WASM fallback/remount, overlay recovery, and forced reload.
- [x] Production-generated output is byte-identical with or without dev HMR.

## Edge Cases

- Current tab is outside all affected routes.
- Duplicate component roots, removed root, renamed component, missing stable
  focus target, malformed fresh HTML, and fetch failure.
- Store shape changes while island shape stays the same and vice versa.
- Unknown protocol version or runtime ABI marker.

## Dependencies

- Internal: incremental dependency graph, transactional publication, generated
  island/store markers, dev SSE bridge, browser test harness.
- External: none in production; browser tests use the existing Node tooling.

## Open Questions

- None for v2. Cross-navigation state preservation remains out of scope.
