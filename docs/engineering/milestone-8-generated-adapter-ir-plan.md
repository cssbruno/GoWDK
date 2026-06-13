# Implementation Plan: Milestone 8 Generated Adapter IR

## Context

Spec: `docs/engineering/milestone-8-generated-adapter-ir-spec.md`

Roadmap step 8: Generated adapter IR.

Relevant ADRs: 0002 compile-first render model, 0005 generated Go emission
boundary, 0006 GOWDK Compiler and Runtime boundary.

## Assumptions

- Existing endpoint slices remain the accepted public `appgen.Options` input for
  now, but generator internals should lower them into `BackendAdapterIR` once
  and use the IR for backend route decisions.
- SSR route generation can stay in its current path because roadmap step 12 owns
  request-time page rendering.

## Proposed Changes

- Expand `BackendAdapterIR` with endpoint guard/import metadata and helpers for
  routable registrations, dynamic registrations, backend imports, and guard
  names.
- Update generated backend router and split-proxy route matching to consume
  adapter IR registrations instead of raw action/API/fragment slices.
- Update backend-sensitive import, CSRF, guard, rate-limit, and backend route
  presence checks to consume adapter IR where practical.
- Add focused tests for adapter IR metadata and split-proxy route matching.
- Update generated app golden output only if generated source ordering changes.
- Mark roadmap step 8 as implemented when the acceptance criteria are covered.

## Files Expected To Change

- `internal/appgen/adapter_ir.go`
- `internal/appgen/source.go`
- `internal/appgen/source_backend.go`
- `internal/appgen/source_guards.go`
- `internal/appgen/source_rate_limit.go`
- `internal/appgen/adapter_ir_test.go`
- `internal/appgen/appgen_test.go`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- No public API change.
- No manifest JSON shape change.
- Generated Go behavior should stay compatible.

## Tests

- Unit: `go test ./internal/appgen`
- Integration: `go test ./internal/compiler ./internal/buildgen ./internal/appgen`
- End-to-end: `go run ./cmd/gowdk build --out /tmp/gowdk-m8-build --app /tmp/gowdk-m8-app examples/pages/*.gwdk`
- Manual: inspect generated app source for backend router registration through
  `runtime/app.BackendRouter`.

## Verification Commands

```sh
go test ./internal/appgen
go test ./internal/compiler ./internal/buildgen ./internal/appgen
go build ./cmd/gowdk
go run ./cmd/gowdk build --out /tmp/gowdk-m8-build --app /tmp/gowdk-m8-app examples/pages/*.gwdk
scripts/test-go-modules.sh
```

## Rollback Plan

- Revert the adapter IR migration commits. Existing endpoint-slice generation
  paths are preserved by tests and can be restored without data migration.

## Risks

- Import pruning can accidentally drop runtime packages needed only by generated
  proxy or compatibility paths.
- Route matching order can shift for split proxy output if registrations are not
  sorted consistently.
- Broad helper changes can affect generated app goldens outside the intended
  backend route path.
