# Feature Spec: GWDK Go Build Imports

## Problem

GOWDK examples should show how a `.gwdk` file imports user-owned Go code using a
normal Go import path. The current examples only show literal `build {}` data,
which makes the intended Go-first authoring model unclear.

## Goals

- Allow a page `.gwdk` file to declare a top-level Go import with a GitHub-style
  import path.
- Allow `build {}` to call one no-argument function from an imported package.
- Feed the returned JSON object into the existing static `view {}` interpolation
  data.
- Keep literal `build {}` behavior working.

## Non-Goals

- No generated action, API, partial, or SSR `load {}` user handler wiring.
- No arbitrary statement execution inside `build {}`.
- No fallback literal data path in the Go import example.

## Users And Permissions

- Primary users: Go developers writing `.gwdk` pages.
- Roles or permissions: local build users only.
- Data visibility rules: build-time Go functions run with the same local
  permissions as `gowdk build`.

## User Flow

1. User writes `import alias "github.com/user/project/pkg"` in a page.
2. User writes `build { => alias.Func() }`.
3. `gowdk build` runs the function at build time and renders `view {}` with the
   returned data.

## Requirements

### Functional

- Parse aliased and unaliased top-level `import` declarations in page files.
- Resolve `=> alias.Func()` against parsed imports.
- Execute the function using the Go toolchain at build time.
- Accept JSON object results and convert scalar fields to string interpolation
  values.

### Non-Functional

- Performance: one Go invocation per page output using Go build cache.
- Reliability: errors must name the unsupported build syntax or missing import.
- Accessibility: no UI impact.
- Security/privacy: build-time functions execute local Go code intentionally.
- Observability: manifest should expose parsed page imports.

## Acceptance Criteria

- [x] The Go interop example builds without literal fallback data.
- [x] `go run ./cmd/gowdk check` accepts the new import syntax.
- [x] `go test ./...` passes.

## Edge Cases

- Missing import alias for the build function.
- Build function returns a non-object JSON value.
- Build function returns nested data that cannot be interpolated by the current
  string-only renderer.

## Dependencies

- Internal: parser, manifest, staticgen.
- External: Go toolchain available during `gowdk build`.

## Open Questions

- Should future phases support `(T, error)` return signatures?
- Should imports be shared across page, component, and layout files?
