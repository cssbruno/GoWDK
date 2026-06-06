# Semantics

## Current Render Rules

- Default render mode is `spa`.
- Supported render modes are `spa`, `action`, `hybrid`, and `ssr`.
- `@render ssr` requires the SSR addon. `@render hybrid` stays build-time SPA
  output unless it declares explicit request-time behavior such as `load {}`.
- Page IDs must be unique within the manifest.
- Component names must be unique within the manifest.
- Dynamic build-time routes such as `/blog/{slug}` require a `paths {}` block.
  Action endpoints on those pages inherit the generated concrete paths.
- SPA navigation enhancement is optional runtime behavior over literal internal
  links. Route existence, route output, auth, and server behavior remain owned
  by generated files and generated Go.
- `load {}` runs at request time and requires `@render ssr` or `@render hybrid`.
- SPA pages may declare `act` blocks without SSR.

## Current Metadata Semantics

- `@page` and `@route` are required.
- `@title`, `@description`, `@canonical`, and `@image` record document metadata
  used by generated HTML output. If `@title` is omitted, generated output falls
  back to the page ID. `@image` feeds generated Open Graph and Twitter image
  tags when social head output is enabled by page or config metadata.
- `@layout` records ordered page layout references. Bare references resolve to
  same-package layout IDs or legacy package-less layouts. Cross-package layouts
  require `use alias "package"` and qualified refs such as `alias.root`.
- `@guard` records guard IDs. Generated SSR, action, and API handlers run
  declared guards before request-time user logic and fail closed unless the
  generated app registers matching guard functions.
- Generated SSR, action, and API handlers are protected by runtime panic
  boundaries that return no-store HTTP 500 responses before headers are
  written.
- `paths {}` records that dynamic SPA paths are declared and preserves raw
  body text internally. SPA builds can execute literal string declarations
  such as `=> { slug: "hello-gowdk" }` to expand dynamic route output paths.
- `build {}` records block presence and raw body text internally. SPA builds
  can execute one literal string declaration such as
  `=> { title: "Hello" }` and expose those values to `view {}` interpolation.
  SPA builds can also execute one imported no-argument Go function call such
  as `=> interop.FeaturedCopyForBuild()` when the page declares
  `import interop "github.com/..."`.
- `load {}` runs at request time for SSR or request-time hybrid pages. The
  first generated execution slice supports `=> { field }` declarations and
  same-package Go load functions named `Load<PageID>` that receive
  `ssr.LoadContext`.
- `view {}` records block presence and raw body text for the current app-shell HTML
  subset. SPA builds interpolate route params and component props in text and
  attribute values, escaping the result.
- `act Name POST "/path"` records exact exported action handler symbols and
  endpoint paths.
- `api Name METHOD "/path"` records exact exported API handler symbols, methods,
  and endpoint paths.

## Planned Semantics

Future compiler phases must define broader symbol resolution, type checking,
layout composition, full component resolution, route parameter binding into
imported `build {}` calls, real typed action decoding and execution, generated
API/fragment execution, partial updates, cache/revalidation behavior, and guard
execution.
