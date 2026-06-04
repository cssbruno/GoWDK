# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- `@render ssr` is parsed as a render mode.
- `@render ssr` requires the SSR addon during validation.
- `@render hybrid` also requires the SSR addon in the current validator.
- `load {}` is allowed only with `@render ssr` or `@render hybrid`.
- `@guard` is parsed and emitted as metadata, but does not execute.

## Planned Support

Future SSR work must define request-aware `load {}` execution, guard contracts, request layouts, error handling, route registration, and exactly how hybrid pages avoid becoming implicit full-page SSR.
