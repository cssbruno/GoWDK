# Implementation Plan: API CORS Policy

## Context

Spec: `docs/product/api-cors.md`.
Issue: `#507` `[API] CORS policy for generated API endpoints`.
Relevant ADRs: 0005 generated Go emission boundary, 0006 compiler/runtime
boundary.

## Assumptions

- Config-level policy is enough for the first slice.
- CORS applies to generated API, command, and query routes.
- Actions, fragments, SSR, and static pages stay out of scope.

## Proposed Changes

- Add `gowdk.CORSConfig` under `BuildConfig`.
- Parse and validate literal `Build.CORS` in `internal/project`.
- Add runtime CORS helpers to `runtime/app.BackendRouter`.
- Emit `BackendRouter.SetCORSPolicy` from `internal/appgen` when CORS is enabled
  and relevant routes exist.
- Update API/config/generated-output docs and requirements.

## Files Expected To Change

- `gowdk.go`
- `runtime/app/*`
- `internal/project/config.go`
- `internal/appgen/*`
- `docs/product/requirements.md`
- `docs/reference/config.md`
- `docs/language/api.md`
- `docs/compiler/generated-output.md`

## Data And API Impact

- New public config type: `gowdk.CORSConfig`.
- New runtime type and method: `runtime/app.CORSPolicy` and
  `(*BackendRouter).SetCORSPolicy`.
- No generated manifest shape changes.

## Tests

- Unit: runtime router CORS preflight/actual headers and invalid policy.
- Unit: config parser and generated source wiring.
- Integration: focused appgen package tests.

## Verification Commands

```sh
go test ./runtime/app ./internal/appgen ./internal/project
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove `Build.CORS` parsing and generated `SetCORSPolicy` emission.
- Keep runtime helper removable because no generated route depends on it once
  appgen emission is reverted.

## Risks

- Header policy defaults can surprise users if they forget `Content-Type` for
  JSON APIs. Docs call out explicit headers.
- Per-endpoint CORS needs a separate syntax/design pass if route-specific policy
  becomes necessary.
