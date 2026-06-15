# Implementation Plan: Addon Registry Metadata And Gated Discovery

## Context

Relevant spec: `docs/product/addon-registry-spec.md`.

## Assumptions

- Local metadata is safe to expose before remote discovery.
- `gowdk add <name>` remains limited to built-ins the CLI can edit safely.
- External addons stay explicit Go imports until trust and compatibility
  policies are enforceable.

## Proposed Changes

- Add `internal/addonregistry` with embedded JSON metadata and validation.
- Add bundled metadata for current built-in addon packages.
- Add `gowdk add --list --registry` and `--json`.
- Keep existing `gowdk add --list` output focused on addable built-ins.
- Document metadata fields, trust boundaries, and external installation rules.
- Update product requirements, release checklist, architecture, README, and CLI
  reference.

## Files Expected To Change

- `internal/addonregistry/registry.go`
- `internal/addonregistry/registry.json`
- `internal/addonregistry/registry_test.go`
- `cmd/gowdk/add.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `docs/reference/addons.md`
- `docs/reference/cli.md`
- `README.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/engineering/release-plan.md`

## Data And API Impact

- Adds a local JSON metadata contract with schema version `1`.
- Adds CLI flags under `gowdk add --list`.
- Does not change public addon constructors or config syntax.

## Tests

- Unit: registry validation rejects missing fields and invalid addability.
- CLI: default add list, registry table, registry JSON, and category rendering.
- Integration: existing `gowdk add` config rewrite tests continue to cover
  addable built-ins.
- End-to-end: not needed; this slice does not affect builds.
- Manual: inspect `gowdk add --list --registry`.

## Verification Commands

```sh
go test ./internal/addonregistry ./cmd/gowdk
go build ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
git diff --check
```

## Rollback Plan

- Remove `internal/addonregistry`, new `gowdk add --list` flags, tests, and
  docs updates.
- Keep existing `gowdk add --list` built-in behavior.

## Risks

- Metadata can be mistaken for endorsement of third-party code. Mitigate by
  keeping trust notes explicit and keeping external addons non-addable.
- Registry fields can drift from addon code. Mitigate with validation tests and
  docs that make the registry the discovery source of truth.
