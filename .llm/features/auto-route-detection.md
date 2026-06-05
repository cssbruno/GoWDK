# Feature Spec: Auto Route Detection

## Problem

`gowdk build --app` should not need a separate hand-wiring step for generated
request-time handlers. The compiler already has a parsed manifest and SPA
output context, so generated app creation should detect the supported action and
first-slice SSR routes from that source of truth.

## Goals

- Detect generated action routes from the parsed manifest during generated app creation.
- Detect first-slice SSR routes from the parsed manifest and generated SPA output context.
- Keep explicit `appgen.Options.Actions` and `appgen.Options.SSR` usable for tests and lower-level callers.
- Keep unsupported API, `load {}`, guard, and full user logic routing out of this slice.

## Non-Goals

- Generate API handlers.
- Execute user action logic.
- Execute SSR `load {}` blocks.
- Infer routes from file paths.
- Replace user-owned external router integration.

## Users And Permissions

- Primary users: GOWDK app authors using `gowdk build --app`, `--bin`, or `--wasm`.
- Roles or permissions: no new runtime permissions.
- Data visibility rules: generated app code should not expose additional source data.

## User Flow

1. A user builds SPA output and a generated app with `gowdk build --app`.
2. The CLI passes the parsed manifest to app generation.
3. App generation detects supported action and SSR routes and emits the handler hooks.

## Requirements

### Functional

- `appgen.GenerateWithOptions` supports `AutoRoutes` with a parsed manifest.
- Auto detection emits the same action handlers as explicit `Actions`.
- Auto detection emits the same first-slice SSR handlers as explicit `SSR`.
- Missing manifest input fails with a clear error.

### Non-Functional

- Performance: route detection should reuse existing manifest/buildgen helpers.
- Reliability: existing validation must still reject invalid generated routes.
- Accessibility: no UI impact.
- Security/privacy: no new generated route kinds beyond already supported handlers.
- Observability: generated code remains deterministic.

## Acceptance Criteria

- [x] `gowdk build --app` uses auto route detection.
- [x] Explicit appgen route options still work.
- [x] Auto route detection is covered by focused appgen tests.

## Edge Cases

- Auto routes without a manifest must return a clear error.
- Explicit and auto-detected duplicate routes are rejected by existing validators.
- SSR pages with unsupported `load {}` still fail before handler generation.

## Dependencies

- Internal: `internal/manifest`, `internal/buildgen`, existing appgen route validators.
- External: none.

## Open Questions

- Should API handler generation join the same auto-detection path once API
  execution exists?
