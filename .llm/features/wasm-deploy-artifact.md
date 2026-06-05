# Feature Spec: WASM Deploy Artifact

## Problem

GOWDK can generate static output and package that output into a generated Go app
or local binary. Users also want a deployment option that emits a Go WebAssembly
artifact from the same selected module output. This generated app artifact is
separate from explicit browser island assets emitted by `g:island="wasm"`.

## Goals

- Add an ad hoc `gowdk build --wasm <file>` option.
- Add static `Build.Targets[].WASM` config so each target can choose whether it
  emits a WASM artifact.
- Keep module selection behavior consistent across `--out`, `--app`, `--bin`,
  and `--wasm`.
- Document that this is a generated Go `js/wasm` deploy artifact, not the
  browser island ABI.

## Non-Goals

- Browser island ABI or client-side component hydration.
- Runtime-specific deployment adapters beyond producing the `.wasm` artifact.
- Restarting a watched WASM artifact as a local process.

## Users And Permissions

- Primary users: GOWDK app developers configuring static deploy outputs.
- Roles or permissions: no new roles.
- Data visibility rules: generated artifacts contain the selected module output
  and must not include unrelated modules.

## User Flow

1. The user chooses source files or configured modules.
2. The user supplies `--app <dir>` and `--wasm <file>`, or configures a build
   target with `App` and `WASM`.
3. GOWDK renders static output, generates the embedded app, then runs Go with
   `GOOS=js GOARCH=wasm` to produce the WASM artifact.

## Requirements

### Functional

- `--wasm <file>` and `--wasm=<file>` are accepted by `gowdk build`.
- `--wasm` requires `--app <dir>`.
- `BuildTargetConfig` exposes `WASM string`.
- Literal `gowdk.config.go` parsing reads `WASM` on build targets.
- Configured targets with `WASM` require `App`.
- `watch` input analysis treats WASM artifact builds as full builds.

### Non-Functional

- Performance: keep the existing incremental static watch path for builds that
  do not request generated app, binary, or WASM output.
- Reliability: fail early on missing paths or invalid target combinations.
- Accessibility: not applicable.
- Security/privacy: generated WASM artifacts must use the same selected source
  set as static output and binaries.
- Observability: print the generated WASM artifact path like other build
  artifacts.

## Acceptance Criteria

- [x] `gowdk build --out <dir> --app <dir> --wasm <file> <files...>` emits a
  non-empty `.wasm` artifact.
- [x] `Build.Targets[].WASM` emits a non-empty `.wasm` artifact for the selected
  target.
- [x] `--wasm` without `--app` returns a clear error.
- [x] Docs show CLI and static-config usage.

## Edge Cases

- Empty `--wasm=` should fail before build work begins.
- `--target` must remain mutually exclusive with ad hoc `--module`, `--out`,
  `--app`, `--bin`, `--wasm`, and explicit files.
- Multiple configured targets may emit different WASM artifacts from different
  module sets.

## Dependencies

- Internal: `internal/appgen`, `cmd/gowdk`, `internal/project`, public
  `gowdk.BuildTargetConfig`.
- External: local Go toolchain with `js/wasm` support.

## Open Questions

- Which deployment runtimes should receive first-class guides after the generic
  `.wasm` artifact exists?
- When browser WASM islands arrive, should they reuse `WASM` or introduce a
  separate config boundary?
