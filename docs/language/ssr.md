# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- `@render ssr` is parsed as a render mode.
- `@render ssr` requires the SSR addon during validation.
- `@render hybrid` also requires the SSR addon in the current validator.
- `gowdk build --ssr --app <dir> --bin <file>` can generate a binary that
  serves simple concrete `@render ssr` pages rendered from `view {}` and
  literal or imported `build {}` data.
- `load {}` is allowed only with `@render ssr` or `@render hybrid`; the parser
  preserves its raw body text and SSR codegen emits first-slice load function
  stubs that receive `ssr.LoadContext`, but generated user execution is not
  wired yet.
- The SSR addon exposes a small router registration contract for generated SSR
  page handlers.
- The SSR addon provides a default HTTP 500 error handler contract for
  request-time SSR failures.
- `@guard` uses comma-separated guard IDs such as `@guard auth.required,
  billing.active`. The SSR addon exposes `GuardFunc`, `GuardRegistry`, and
  ordered guard execution contracts; generated handlers do not wire guard
  execution yet.

## Planned Support

Future SSR work must define request-aware `load {}` execution, guard wiring,
dynamic SSR routes, request layouts, error handling, route registration, and
exactly how hybrid pages avoid becoming implicit full-page SSR.
