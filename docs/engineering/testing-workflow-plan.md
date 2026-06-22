# Implementation Plan: First-Class Testing Workflow

## Context

Relevant issues: [#646](https://github.com/cssbruno/GoWDK/issues/646) and
[#648](https://github.com/cssbruno/GoWDK/issues/648).

Spec: `docs/product/testing-workflow-spec.md`.

Relevant ADRs: 0005 generated Go boundary, 0006 compiler/runtime boundary, and
0015 generated binary lifecycle services.

## Assumptions

- The first slice should be useful without a browser or Node.
- `gowdk test` should orchestrate existing Go tooling instead of replacing it.
- Generated app artifacts for tests should be temporary by default.
- The focused race job should cover root-module runtime packages only.

## Proposed Changes

- Add `scripts/test-runtime-race.sh` with an explicit package list and a
  preflight that rejects packages with no tests.
- Add Linux CI job `Runtime race detector`.
- Add concurrent tests for runtime metrics, contract registry/seen-store,
  worker retry, SSE disconnects, rate-limit expiration, and testkit cookies.
- Add `runtime/testkit` request/client/response helpers beside the existing
  scenario API.
- Add `gowdk test` with `unit`, `app`, `binary`, and explicit external-command
  `browser` stages.
- Update `gowdk init --tests` to scaffold a non-skipping test and a minimal app
  module for Go test execution.
- Update CLI/testing/CI/product docs.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/test.go`
- `cmd/gowdk/init.go`
- `cmd/gowdk/main_test.go`
- `runtime/testkit/testkit.go`
- runtime package tests under `runtime/`
- `scripts/test-runtime-race.sh`
- `.github/workflows/ci.yml`
- `docs/reference/testing.md`
- `docs/engineering/testing.md`
- `docs/engineering/ci.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `README.md`

## Data And API Impact

- New CLI command: `gowdk test`.
- New scaffolded environment contract: `GOWDK_TEST_OUTPUT_DIR`,
  `GOWDK_TEST_APP_DIR`, `GOWDK_TEST_BINARY`, `GOWDK_TEST_BASE_URL`,
  `GOWDK_TEST_WORKDIR`, and `GOWDK_TEST_STAGE`.
- New additive `runtime/testkit` helpers. Existing names remain compatible.

## Tests

- Unit: parser/option tests for `gowdk test`, testkit helper tests, and runtime
  concurrency tests.
- Integration: scaffolded `gowdk init --tests` app runs through `gowdk test`.
- End-to-end: generated binary starts on an ephemeral address for the binary
  stage.
- Manual: `scripts/test-runtime-race.sh`.

## Verification Commands

```sh
go test ./runtime/testkit ./runtime/app ./runtime/contracts ./runtime/contracts/sse ./runtime/ratelimit ./cmd/gowdk
scripts/test-runtime-race.sh
go build ./cmd/gowdk
```

## Rollback Plan

- Remove `gowdk test` dispatch and docs.
- Revert scaffolded test files to the previous smoke test.
- Remove the CI race job and script if the race budget is too high.

## Risks

- Generated binary startup can be slow on cold caches.
- A scaffolded `go.mod` pins the current GOWDK version and must be kept in sync
  with release version bumps.
- The race job may expose existing races in packages outside the new tests; fix
  the race rather than weakening the package list.
