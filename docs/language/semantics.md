# Semantics

## Current Render Rules

- Default render mode is `static`.
- Supported render modes are `static`, `action`, `hybrid`, and `ssr`.
- `@render ssr` and `@render hybrid` require the SSR addon in the current validator.
- Page IDs must be unique within the manifest.
- Component names must be unique within the manifest.
- Dynamic static/action routes such as `/blog/{slug}` require a `paths {}` block.
- `load {}` runs at request time and requires `@render ssr` or `@render hybrid`.
- Static pages may declare `act` blocks without SSR.

## Current Metadata Semantics

- `@page` and `@route` are required.
- `@layout` records ordered page layout IDs. Layout files can declare layout
  identity with `@layout <id>`; when layout files are present, validation
  resolves page layout refs by ID.
- `@guard` records guard IDs as metadata only.
- `paths {}` records that dynamic static paths are declared and preserves raw
  body text internally. Static builds can execute literal string declarations
  such as `=> { slug: "hello-gowdk" }` to expand dynamic route output paths.
- `build {}` records block presence and raw body text internally. Static builds
  can execute one literal string declaration such as
  `=> { title: "Hello" }` and expose those values to `view {}` interpolation.
- `load {}` records block presence and raw body text internally. Request-time
  execution is planned.
- `view {}` records block presence and raw body text for the current static HTML
  subset. Static builds interpolate route params and component props in text and
  attribute values, escaping the result.
- `act <name> {}` records action names plus the first supported form-input,
  validation-intent, and local redirect subset.
- `api <name> {}` records names plus the first method/route metadata line, such
  as `GET "/api/health"`.

## Planned Semantics

Future compiler phases must define symbol resolution, type checking, layout composition, full component resolution, route parameter binding into `build {}` expressions, real typed action decoding and execution, generated API/fragment execution, partial updates, cache/revalidation behavior, and guard execution.
