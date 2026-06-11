# Feature Spec: go/packages Endpoint Binding Inspection

## Problem

Endpoint binding used to parse sibling `.go` files directly and match AST
shapes. That could drift from ordinary Go semantics for build tags, renamed
imports, type aliases, package load failures, and exact exported package-scope
symbols.

## Goals

- Inspect real same-directory Go packages through `golang.org/x/tools/go/packages`.
- Resolve endpoint handlers from package type information instead of import-name
  AST matching.
- Preserve the current supported action, API, fragment, and SSR load
  signatures.
- Keep missing and unsupported bindings explicit for generated HTTP 501 stubs
  and strict production diagnostics.

## Non-Goals

- Add new endpoint signatures.
- Change `.gwdk` endpoint syntax.
- Change inline `go {}` extraction. Inline generated Go blocks still use the
  existing synthetic AST path because they are not real packages on disk yet.

## Requirements

### Functional

- Owning source directories are loaded as Go packages with `packages.Load`.
- Exported package-scope handler symbols are inspected through `types.Info`,
  `types.Signature`, and package scope/type metadata.
- Build-tag-excluded files do not bind handlers unless their tags are active in
  the build environment.
- Renamed imports and response type aliases resolve according to Go type
  identity.
- Package load errors become clear missing-binding metadata.
- Typed action input fields are derived from compiled `types.Struct` metadata
  and preserve existing form tag and supported-field-type behavior.

### Non-Functional

- Performance: only owning endpoint package directories are loaded, and each
  directory is cached for the current binding pass.
- Reliability: unsupported signatures continue to be reported without importing
  or referencing invalid handlers in generated output.
- Security/privacy: diagnostics and binding messages do not include submitted
  form values or runtime secrets.
- Observability: public route/build metadata keeps the existing binding status,
  message, import path, package, function, signature, and input field fields.

## Acceptance Criteria

- [x] Handler binding uses type-checked Go package information for real Go
      packages.
- [x] Build-tag-excluded handlers remain missing by default.
- [x] Renamed imports and response type aliases resolve correctly.
- [x] Non-exported handlers remain missing.
- [x] Package load errors produce clear binding metadata.
- [x] Unsupported signatures still fail strict production builds unless missing
      backend stubs are explicitly allowed.
- [x] Existing generated app behavior remains compatible for supported
      signatures.

## Dependencies

- Internal: `internal/compiler`, `internal/gwdkir`, generated app consumers of
  backend binding metadata.
- External: `golang.org/x/tools/go/packages`, used only by the compiler to load
  owning Go packages with standard Go semantics.
