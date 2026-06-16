# Feature Spec: M4 Go Interop

## Problem

Users can declare actions, APIs, SSR load functions, build-time Go helpers, and
web contract references from `.gwdk`, but they need a direct way to understand
which Go symbol was expected, which package was inspected, why binding failed,
and what to do next.

## Goals

- Make Go interop behavior visible through a dedicated docs page.
- Expose a `gowdk inspect go-bindings` JSON report.
- Generate conservative missing action/API handler stubs without overwriting
  user code.
- Support build-time helpers returning `T` or `(T, error)`.
- Keep build-helper stderr separate from JSON payload parsing.
- Document the current typed route-param and middleware contracts.

## Non-Goals

- Generate fragments, load functions, commands, or queries as stubs.
- Generate per-route param structs.
- Pass route params into Go build functions.
- Add route rewriting, response transform, or fetch interception hooks.
- Treat generated stubs as production implementation.

## Users And Permissions

- Primary users: Go developers connecting `.gwdk` declarations to normal Go
  packages.
- Roles or permissions: no new runtime permissions.
- Data visibility rules: reports include source/package/symbol metadata and
  diagnostic messages, not request bodies, cookies, tokens, or submitted form
  values.

## User Flow

1. User declares a `.gwdk` action, API, `server {}`, build helper, or contract
   reference.
2. User runs `gowdk inspect go-bindings`.
3. Report shows status, package path, expected symbol, signature/input metadata,
   reason, and suggestion.
4. For missing action/API handlers, user can run `gowdk generate stubs`.
5. User edits or moves the generated normal Go functions into app-owned code.

## Requirements

### Functional

- `inspect go-bindings` reports actions, APIs, fragments, load functions,
  build-time Go function calls, and web command/query references where present.
- Binding records include source, source span when known, package, page ID,
  symbol, package path/name when known, method/path for request-time handlers,
  status, signature, input type/fields where available, message, and suggestion.
- `generate stubs` writes only missing action/API handlers to `gowdk_stubs.go`
  beside the owning source package.
- Stub generation refuses to overwrite an existing stub file.
- Generated action stubs use `func(context.Context) (response.Response, error)`.
- Generated API stubs use
  `func(context.Context, *http.Request) (response.Response, error)`.
- Build-time Go helpers can return `T` or `(T, error)`.
- Successful stderr output from build helpers does not corrupt JSON parsing.

### Non-Functional

- Performance: reports reuse the validated compiler IR and existing package
  binding pass.
- Reliability: generated stubs are gofmt-formatted normal Go files.
- Accessibility: no UI surface.
- Security/privacy: report and generated code avoid runtime request data.
- Observability: report schema is versioned.

## Acceptance Criteria

- [x] `gowdk inspect go-bindings` emits versioned JSON with backend, load,
      build, and contract binding records.
- [x] `gowdk generate stubs` writes missing action/API stubs and refuses to
      overwrite existing stub files.
- [x] Build helpers returning `T` still work.
- [x] Build helpers returning `(T, error)` work.
- [x] Successful stderr from build helpers does not corrupt JSON payloads.
- [x] Docs describe Go interop, typed params, middleware wrapping, and deferred
      accessor/stub surfaces.

## Edge Cases

- Missing build imports are reported as missing build bindings.
- Unsupported existing action/API signatures are reported but not duplicated by
  stub generation.
- Package load errors remain binding metadata; generated stubs may not fix a
  broken Go package until the package itself compiles.

## Dependencies

- Internal: `cmd/gowdk`, `internal/compiler`, `internal/buildgen`,
  `internal/gwdkir`, `runtime/app`, `runtime/route`.
- External: local Go toolchain for package inspection and build-helper
  execution.

## Open Questions

- Should route params be passed into build functions, and if so as what stable
  Go type? Deferred to #327.
- Should per-route generated param structs be added after `app.TypedParams(ctx)`
  has settled? Deferred to #23.
