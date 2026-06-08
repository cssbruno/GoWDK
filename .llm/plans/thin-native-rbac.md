# Implementation Plan: Thin Native RBAC

## Context

Relevant spec: `.llm/features/thin-native-rbac.md`

## Assumptions

- The first slice should reuse `@guard` instead of adding parser syntax.
- Page access is default-deny at source validation time: public pages must say
  `@guard public`; protected pages must declare non-public guard IDs.
- Non-public page guards need request-time rendering for frontend page access;
  static SPA output cannot enforce that gate.
- Native guard IDs are `role:<name>` and `permission:<name>`.
- Application Go owns principal extraction.
- Native RBAC is defense-in-depth route/page access redundancy and never
  replaces Go backend authorization.

## Proposed Changes

- Add `Principal`, `Provider`, and native guard-prefix helpers to
  `runtime/auth`.
- Keep guard execution in the existing generated guard path for this slice.
- Generate required backing hook references in guarded app packages.
- Update generated guard execution to pass the configured auth provider.
- Add validation for missing `@guard`, mixed `public` guard metadata, and
  protected guards on build-time page routes.
- Add unit and generated-app tests.
- Update hooks/reference docs and product status docs.

## Files Expected To Change

- `addons/ssr/guards.go`
- `addons/ssr/ssr_test.go`
- `internal/appgen/source_guards.go`
- `internal/appgen/appgen_test.go`
- `docs/reference/hooks.md`
- Product/architecture docs that mention guards.

## Data And API Impact

- Adds runtime API in `runtime/auth`.
- Adds generated app requirements for `GOWDKAuthProvider() auth.Provider` when
  native RBAC guards exist and `GOWDKGuardRegistry() ssr.GuardRegistry` when
  custom guards exist.
- Public pages report `guard: ["public"]`; protected pages continue reporting
  declared guard IDs.

## Tests

- Unit: native role/permission checks in `addons/ssr`.
- Integration: generated app source includes required backing hook calls.
- End-to-end: generated binary allows a native RBAC guard when a provider
  returns a matching principal.

## Verification Commands

```sh
go test ./addons/ssr ./internal/appgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the new `runtime/auth` provider API and generated backing hook calls.
- Existing custom guard behavior remains the fallback.

## Risks

- Guard error wording can affect tests. Keep existing custom guard errors stable
  and wrap native failures through the same `RunGuards` path.
