# Implementation Plan: Playground Hosted Execution And Export

## Context

Relevant spec: `docs/product/playground-hosted-execution-spec.md`.

Relevant issue: [#421](https://github.com/cssbruno/GoWDK/issues/421).

## Assumptions

- Hosted execution remains optional and disabled by default.
- The repository should ship a local contract and bridge before any website
  service runs user code.
- Exported projects are source archives, not generated deployment bundles.

## Proposed Changes

- Add `internal/playground` for sandbox policy, safe file collection, source
  export, workspace staging, and environment sanitization.
- Add `gowdk playground policy`, `gowdk playground export`, and
  `gowdk playground run`.
- Gate local execution behind `--allow-hosted-execution`.
- Update product, CLI, release, and security docs.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/playground.go`
- `cmd/gowdk/main_test.go`
- `internal/playground/playground.go`
- `internal/playground/playground_test.go`
- `docs/product/playground.md`
- `docs/product/playground-hosted-execution-spec.md`
- `docs/engineering/playground-hosted-execution-implementation-plan.md`
- `docs/reference/cli.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/getting-started.md`
- `docs/engineering/release-plan.md`
- `docs/engineering/security-threat-model.md`

## Data And API Impact

- New CLI surface: `gowdk playground`.
- New internal package: `internal/playground`.
- No public runtime API or generated output contract changes.

## Tests

- Unit: playground file filtering, export archives, workspace staging, policy
  JSON, and environment rejection.
- Integration: CLI policy/export/run behavior through `cmd/gowdk` tests.
- End-to-end: opt-in local sandbox run builds from a staged workspace.
- Manual: inspect JSON policy and export a sample project archive.

## Verification Commands

```sh
go test ./internal/playground ./cmd/gowdk
go run ./cmd/gowdk playground policy --json
go build ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
git diff --check
```

## Rollback Plan

- Remove `gowdk playground` dispatch and the `internal/playground` package.
- Revert docs to the docs-only playground contract.
- No migration is required because no persisted runtime data is introduced.

## Risks

- Local sandboxing is a defense-in-depth bridge, not a full production isolation
  boundary.
- Offline Go dependency resolution may reject projects that rely on uncached
  third-party dependencies.
- A future hosted service still needs process, CPU, memory, network, rate-limit,
  log, and cleanup enforcement outside this repository.
