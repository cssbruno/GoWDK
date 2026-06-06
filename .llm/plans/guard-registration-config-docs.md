# Implementation Plan: Guard Registration Configuration Docs

## Context

Relevant spec: `.llm/features/guard-registration-config-docs.md`

## Assumptions

- Guard functions stay in normal Go and are registered from generated app
  package initialization code or explicit app startup wiring.
- The generated app package remains the only package that imports generated
  symbols such as `RegisterGuards`.

## Proposed Changes

- Add generated-binary integration coverage for registered guards across SSR,
  action, and API routes.
- Document `RegisterGuards` usage in SSR and backend endpoint references.
- Update product planning docs to remove the guard success-path docs gap.

## Files Expected To Change

- `internal/appgen/appgen_test.go`
- `docs/language/ssr.md`
- `docs/language/actions.md`
- `docs/language/api.md`
- `docs/reference/config.md`
- `docs/product/roadmap.md`

## Data And API Impact

- No runtime API changes. This slice documents and verifies the existing
  generated `RegisterGuards(gowdkssr.GuardRegistry)` hook.

## Tests

- Unit: existing guard source-generation tests.
- Integration: generated binary with registered guard for SSR, action, and API.
- End-to-end: covered through HTTP requests to the generated binary.
- Manual: not required.

## Verification Commands

```sh
go test ./internal/appgen
go test ./...
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert the docs/spec updates and the generated-binary test. Existing guard
  fail-closed behavior is unchanged.

## Risks

- The example could encourage importing generated app packages from feature
  packages. Docs explicitly keep registration in the generated app package or
  startup layer instead.
