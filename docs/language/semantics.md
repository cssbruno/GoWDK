# Semantics

## Current Render Rules

- Default render mode is `spa`.
- Source pages do not declare render modes.
- `server {}` or `go server {}` selects request-time SSR and requires the SSR addon.
- Page IDs must be unique within the manifest.
- Component names must be unique within the manifest.
- Dynamic build-time routes such as `/blog/{slug}` require a `paths {}` block.
  Action endpoints on those pages inherit the generated concrete paths.
- SPA navigation enhancement is optional runtime behavior over literal internal
  links. Route existence, route output, auth, and server behavior remain owned
  by generated files and generated Go. The runtime can prefetch same-origin
  internal page HTML on hover, focus, or touch, fetch it with
  `X-GOWDK-Navigate` during navigation, mark `<html data-gowdk-navigating>`,
  dispatch `gowdk:navigate-start` / `gowdk:navigate-end`, and fall back to a
  normal browser navigation on unsupported or failed responses.
- `server {}` runs at request time.
- SPA pages may declare `act` blocks without SSR.

## Current Metadata Semantics

- `route` is required for page sources. `guard` is optional but a page is not
  public by default: a page that declares no `guard` builds with a
  `missing_page_guard` warning and its route is denied (403) at request time
  until access is stated. Declare `guard public` to serve a page on purpose;
  declare custom guard IDs or native RBAC IDs such as `role:admin` and
  `permission:posts.write` for protected pages. `guard public` must stand
  alone. Access is never granted by omission.
- `page` is optional for file-backed page sources. When omitted, page ID
  derives from the source filename by removing `.page.gwdk` or `.gwdk`.
  Explicit `page` keeps page identity stable across file renames.
- `title`, `description`, `canonical`, and `image` record document metadata
  used by generated HTML output. If `title` is omitted, generated output falls
  back to the page ID. `image` feeds generated Open Graph and Twitter image
  tags when social head output is enabled by page or config metadata.
- `layout` records ordered page layout references. Bare references resolve to
  same-package layout IDs or legacy package-less layouts. Cross-package layouts
  require `use alias "package"` and qualified refs such as `alias.root`.
- `guard` records guard IDs. Generated SSR, action, API, and fragment handlers
  run declared non-public guards before request-time user logic. Guarded
  generated apps require matching backing functions and fail Go compilation
  when those functions are missing. Non-public page guards require request-time
  page rendering because build-time SPA output emits plain static HTML.
- Generated SSR, action, and API handlers are protected by runtime panic
  boundaries that return no-store HTTP 500 responses before headers are
  written.
- `paths {}` records that dynamic SPA paths are declared and preserves raw
  body text internally. SPA builds can execute literal string declarations
  such as `=> { slug: "hello-gowdk" }` to expand dynamic route output paths.
- `build {}` records block presence and raw body text internally. SPA builds
  can execute one literal string declaration such as
  `=> { title: "Hello" }` and expose those values to `view {}` interpolation.
  SPA builds can also execute one imported Go function call such as
  `=> interop.FeaturedCopyForBuild()` when the page declares
  `import interop "github.com/..."`; dynamic `paths {}` builds pass route
  params to helpers that declare one `gowdk.BuildParams` argument.
- `server {}` runs at request time for SSR or request-time hybrid pages.
  Generated SSR supports `=> { field, user.name }` declarations and
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
API/fragment execution, partial updates, cache/revalidation behavior, and
richer guard response behavior.
