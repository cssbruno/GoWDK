# Feature Spec: Build Report

## Problem

`internal/buildgen` has grown into a monolithic package file and build failures
currently surface mostly as plain joined error strings. That makes SPA builds
harder to debug as pages, CSS, components, partials, and islands interact.

Users need every SPA build to produce structured build context so failures
can explain which stage failed and successful builds can show what was planned
and emitted.

## Goals

- Split SPA generation code into focused package files without changing the
  public build behavior.
- Make every successful SPA build result carry a structured report.
- Write that report as `gowdk-build-report.json` for successful disk builds.
- Make SPA build errors wrap the same structured report.
- Add a CLI debug path that can print report events for troubleshooting.
- Document the report contract and when users should inspect it.

## Non-Goals

- Replacing compiler diagnostics or language-server diagnostics.
- Adding a logging dependency.
- Adding a global logger or printing debug output by default.
- Redesigning all SPA generation internals in one change.

## Users And Permissions

- Primary users: GOWDK app developers and tooling that call `buildgen`.
- Roles or permissions: no new roles.
- Data visibility rules: reports must describe source/build stages and artifact
  paths, not form values, secrets, or rendered private data.

## User Flow

1. A developer runs `gowdk build`.
2. SPA generation records validation, planning, writing, and manifest events.
3. On success, `buildgen.Result.Report` is available to tools and disk builds
   write `gowdk-build-report.json`.
4. On failure, callers can unwrap `*buildgen.BuildError` and inspect the report.
5. With `gowdk build --debug`, the CLI prints report events to stderr.

## Requirements

### Functional

- `Build`, `BuildMemory`, and `BuildIncremental` must populate `Result.Report`
  on success.
- `Build` and `BuildIncremental` must write `gowdk-build-report.json` on
  success, and `BuildMemory` must include the same payload in memory output.
- SPA generation errors from those entrypoints must be wrapped in
  `BuildError` with the accumulated report.
- `BuildReport` must include version, mode, output directory, and ordered
  events.
- Events must include severity, stage, kind, message, and optional page, route,
  and path metadata.
- `gowdk build --debug` must print report events without changing artifact path
  stdout.
- `gowdk dev` must accept forwarded `--debug` build flags.

### Non-Functional

- Performance: report recording must be simple append-only in memory.
- Reliability: error wrapping must preserve existing error strings and support
  `errors.Unwrap`.
- Accessibility: CLI debug output must be plain text and readable.
- Security/privacy: report messages must avoid sensitive user data.
- Observability: reports must cover validation, planning, writes, manifests, and
  completion.

## Acceptance Criteria

- [x] Existing buildgen behavior remains compatible with current callers.
- [x] Tests assert successful reports and failed report wrapping.
- [x] `go test ./internal/buildgen ./cmd/gowdk` passes.
- [x] `go test ./...` and `go build ./cmd/gowdk` pass.

## Edge Cases

- Validation fails before planning.
- Planning fails after partial stage events.
- File write fails.
- Incremental builds skip unchanged pages but refresh manifests.
- Debug flag is forwarded through dev build args.

## Dependencies

- Internal:
  - `internal/buildgen`
  - `cmd/gowdk`
  - `docs/compiler`
- External:
  - None.

## Open Questions

- Should future reports be emitted as JSON through a dedicated CLI flag?
