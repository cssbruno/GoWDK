# Implementation Plan: WASM Deploy Artifact

## Context

Relevant spec: `.llm/features/wasm-deploy-artifact.md`

## Assumptions

- The first deployable slice is a Go `js/wasm` build of the generated embedded
  app.
- Browser WASM island assets are separate from the generated app WASM deploy
  artifact. The production browser-side Go island ABI remains future work.

## Proposed Changes

- Extend `gowdk.BuildTargetConfig` with `WASM string`.
- Parse `WASM` from literal `gowdk.config.go` build targets.
- Add `--wasm <file>` and `--wasm=<file>` to `gowdk build`.
- Require generated app output for WASM builds.
- Add `appgen.BuildWASM` that runs `go build` with `GOOS=js` and
  `GOARCH=wasm`.
- Route configured target `WASM` values into the existing build flow.
- Update docs and tests.

## Files Expected To Change

- `gowdk.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`
- `docs/engineering/operations.md`
- `docs/engineering/testing.md`

## Data And API Impact

- Public config API adds `BuildTargetConfig.WASM`.
- CLI adds `gowdk build --wasm`.
- No persisted data migration is required.

## Tests

- Unit: config literal parsing covers target `WASM`.
- Integration: appgen compiles a generated app to a non-empty `.wasm` file.
- Command: CLI rejects `--wasm` without `--app` and emits WASM artifacts from
  ad hoc and configured-target builds.
- Manual: not required for this slice.

## Verification Commands

```sh
go test ./internal/project -run TestLoadConfigFileReadsLiteralSourceAndBuildFields -count=1
go test ./internal/appgen -run 'TestBuildWASMCompilesGeneratedApp|TestBuildBinaryCompilesGeneratedApp' -count=1
go test ./cmd/gowdk -run 'TestBuildCommandBuildsWASMArtifact|TestBuildCommandRunsConfiguredWASMTarget|TestBuildCommandBinRequiresGeneratedApp' -count=1
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove `--wasm` parsing and build flow wiring.
- Remove `BuildTargetConfig.WASM` and config parsing support.
- Remove docs and tests that reference WASM deploy artifacts.

## Risks

- Generated server-style apps may not be useful in every WASM host. Docs must
  present this as a generic Go `js/wasm` artifact, not browser hydration.
- Cross-compiling may fail if future generated app dependencies do not support
  `GOOS=js GOARCH=wasm`; tests cover the current generated app shape.
