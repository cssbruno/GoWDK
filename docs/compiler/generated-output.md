# Generated Output

Generated output currently covers app-shell HTML, selected browser runtime assets,
generated embedded app source, and optional local binary or Go `js/wasm`
artifacts for the implemented compiler slice.

Implemented today:

- `gowdk build [--out <dir>] [files...]` writes route-derived HTML files for simple `spa` and `action` pages.
- `.cmp.gwdk` files can be passed to build or discovered by default and expanded from self-closing component calls.
- `gowdk-routes.json` records the app route, page ID, and relative output path for emitted pages.
- `gowdk-assets.json` records generated app assets such as CSS files emitted
  by CSS processors, generated page CSS files, the partial-update client
  runtime when needed, and route HTML cache metadata.
- `gowdk-build-report.json` records build-output generator validation, planning,
  write, manifest, cleanup, and completion events for every successful disk
  build.
- Page `title`, `description`, `canonical`, `image`, app-level
  `Build.Head`, configured stylesheets, and CSS processor stylesheet links are
  emitted in page `<head>` elements. `Build.Head` enables favicon, Open Graph,
  and Twitter card tags without a post-build patcher.
- CSS processors can emit CSS asset files under the output directory.
- Discovered page CSS inputs selected by implicit `default page` or explicit
  `css` metadata declarations are concatenated into generated page CSS files. Page
  `style {}` CSS is appended to the same generated page CSS asset.
- Page `js "./file.js"` declarations are copied under
  `assets/gowdk/pages/<page>/` and linked only from that page as module scripts.
  Page `js "./file.ts"` declarations are transformed to `.js` files in the same
  directory. Inline page `js {}` blocks emit deterministic files such as
  `inline-gowdk.js`.
- Component `css` files are emitted as scoped CSS assets, linked from
  generated pages, content-hashed, recorded in `gowdk-assets.json`, and served
  with immutable cache headers by generated binaries. Component `style {}` CSS
  is emitted through the same scoped CSS path.
- Component `js "./file.js"` declarations are copied under
  `assets/gowdk/components/<package>/<component>/` and linked only from pages
  that use the component. Component `js "./file.ts"` declarations are
  transformed to `.js` files in the same directory. Inline component `js {}`
  blocks emit deterministic files such as `inline-gowdk.js`. This first slice
  does not bundle JavaScript or follow import graphs.
- Layout `style {}` CSS is emitted as generated CSS and linked by pages that
  declare the layout.
- Dynamic app routes with literal `paths {}` declarations are expanded by
  `gowdk build`.
- Literal dynamic route params can render in the current literal `view {}`
  interpolation subset.
- Literal `build {}` data can render in the current literal `view {}`
  interpolation subset.
- SPA page output composes declared layouts by replacing each applied
  layout's single `<slot />` placeholder before rendering the combined view
  source once.
- `gowdk build --app <dir>` writes a generated Go module that embeds the SPA
  output under `<dir>/SPA`.
- `gowdk build --bin <file>` requires `--app` and compiles that generated app
  into one local binary.
- Generated apps include POST endpoint handlers for the first supported
  action subset on concrete SPA page paths.
- `gowdk inspect go-bindings` reports Go binding status for actions, APIs,
  fragments, SSR load functions, build-time Go calls, and web command/query
  references. `gowdk generate stubs` can write missing action/API handler
  stubs as normal Go code beside the owning source package.
- Generated apps pass one backend hook into `runtime/app.Handler`; generated
  action and API dispatch are internal details behind that hook.
- Generated app creation auto-detects supported action endpoints and supported
  SSR routes from the parsed manifest used by `gowdk build --app`, so the CLI
  does not need to manually register those handler hooks.
- Generated apps include form input decoder functions that
  preserve repeated values and reject unexpected fields inferred from direct
  literal controls in same-page `g:post` forms.
- Named submit controls discovered in literal `g:post` forms are included in
  the allowlist as submit-intent fields before unknown-field rejection; local
  `type="button"` and reset controls are not included.
- Generated action handlers cap request bodies before form parsing and return
  HTTP 413 for oversized submissions.
- Generated apps return HTTP 422 for missing or empty direct SPA
  `required` fields when the action declares `valid(input)?`.
- Generated build output emits `assets/gowdk/gowdk.js` only for pages that use
  partial form metadata with fragment-producing actions.
- Generated build output emits `assets/gowdk/islands/<Component>.js` for
  stateful component instances that use the default generated JavaScript island
  runtime. Island roots carry compiler-owned `data-gowdk-island` markers, and
  generated island assets register idempotent browser mount hooks for initial
  load and partial-swap remounts plus destroy hooks for islands removed by
  partial swaps. The generated JavaScript is scoped to matching
  `<gowdk-island>` roots rather than hydrating the full page. Generated island
  assets carry a compact binding descriptor table and update collected bindings
  through per-binding functions for text, form values, checked state, classes,
  styles, attributes, conditionals, and lists; keyed list updates reuse
  existing DOM nodes by `g:key` and remove stale keyed nodes.
- Page store seed JSON embedded in compiler-owned
  `<script type="application/json">` tags escapes literal `<` as `\u003c`, so
  store data cannot terminate the script element or enter HTML escaped-script
  parser states.
- In the default development build mode, generated JavaScript island assets are
  accompanied by `assets/gowdk/islands/<Component>.js.map` source map files
  that reference the component `.gwdk` source, are recorded in
  `gowdk-assets.json`, include first-slice component/client/view source span
  anchors, and are linked from the JS with `sourceMappingURL`.
  `Build.Mode: gowdk.Production` omits those debug source map artifacts and
  comments and compacts generated island JavaScript by trimming
  formatting-only whitespace.
- Generated build output emits `assets/gowdk/islands/<Component>.wasm` plus
  `assets/gowdk/islands/<Component>.wasm.js` for normal calls to components
  that declare `wasm <package>`, and for explicit call-site overrides that set
  `g:island="wasm"`. When the component declares `wasm <package>`, GOWDK runs
  `GOOS=js GOARCH=wasm go build` for that package and writes the compiled
  browser WASM module to the component asset path plus
  `assets/gowdk/islands/wasm_exec.js` for Go's browser runtime imports. Local
  packages are checked for browser-safe imports before build; server, process,
  and network packages such as `net/http`, `os/exec`, and `database/sql` are
  rejected. A package that does not produce a WASM module or omits required ABI
  exports fails the build. Components without `wasm` keep the minimal
  placeholder module for the loader-shape slice and do not ship `wasm_exec.js`.
  The loader discovers matching island roots, builds the ADR-defined bootstrap
  object from state, props, emits, refs, and binding metadata, calls
  component-scoped WASM exports when present, captures host DOM events, and
  applies the supported validated patch commands for text, visibility,
  attribute, class, style, and emitted-event updates.
- Generated apps can serve concrete and dynamic SSR pages in the supported
  generated request-time slice. Dynamic route params are substituted into
  generated SSR placeholders with request-time HTML escaping. Declared
  `load { => { field } }` pages call same-package Go load functions named
  `Load<PageID>` with `ssr.LoadContext` and replace declared load placeholders
  with escaped returned values. Load errors that wrap `ssr.RedirectError`
  become no-store local redirects; other load failures use generated error-page
  output.
- Generated embedded apps load optional `404.html` and `500.html` from the
  embedded build output and use those pages for not-found responses and
  generated SSR load failures. SSR routes can also declare
  `error "/errors/page.html"` to use a generated route-local HTML error page
  for load failures, generated render failures, and route panics before
  falling back to `500.html`.
- Generated SSR, action, and API request-time lanes recover panics before
  response headers are written as no-store HTTP 500 responses without exposing
  panic values. SSR route panics use a declared route-local `error` page when
  one is available. Action and API declarations can also use endpoint-local
  `error "/errors/name.html"` pages for generated panic boundaries.
- Generated action, API, fragment, contract, SSR load, and addon 5xx error
  responses hide ordinary returned error details from clients. Apps can expose
  an intentional client message by returning `response.HandlerError` with an
  explicit `Message`; generated 4xx handler errors keep their application
  message contract.
- Generated apps can return partial fragment responses from
  action handlers for `X-GOWDK-Partial` requests and standalone
  `fragment Name GET "/path" "#target" { ... }` routes. Standalone fragment
  routes can be concrete or dynamic; dynamic fragment route params are matched
  with `runtime/route` and exposed to hooks through `runtime/app.Params(ctx)`
  and `runtime/app.TypedParams(ctx)`. Standalone fragment bodies can expand
  known components at app generation time. If the source package exports
  `func Name(context.Context) (response.Response, error)`, the generated
  fragment handler calls that request-time hook instead of the static fallback.
- Generated app action endpoint extraction rejects direct file inputs and
  multipart `g:post` forms. Uploads belong in user-owned API/server handlers.
- `internal/compiler` resolves same-package action, API, fragment, and SSR load
  handlers through `go/packages` and `go/types`, so build tags, renamed imports,
  type aliases, and package load errors follow ordinary Go package semantics.
- `internal/gotypes` resolves component prop/state structs through Go module
  import paths using `go list`, `go/parser`, and `go/types`.
- `addons/partial` exposes generated fragment and swap helpers. The underlying
  `runtime/response` envelope carries target and swap metadata for generated
  and future partial handlers.
- `/` maps to `index.html`.
- `/patients` maps to `patients/index.html`.
- Current asset names are stable and deterministic. `gowdk-assets.json`
  records content hashes and cache policy for generated CSS/runtime assets and
  component-level `asset` files. Generated CSS is minified and emitted with
  content-hashed filenames. Component `asset` files are emitted with
  content-hashed filenames under `assets/gowdk/components/`.
- Generated embedded apps skip local environment files, source maps, source
  files, VCS/dependency directories, and common temporary artifacts when copying
  build output into the embedded app.
- Generated embedded apps load `gowdk-assets.json` from the embedded filesystem
  when present and expose the loaded asset count through `/_gowdk/health`.

Not implemented yet:

- Route params passed into imported Go `build {}` functions.
- Full page-aware third-party CSS processor selection beyond the current
  processor stylesheet and generated CSS asset support.
- Non-string props in inline `props {}` blocks.
- Arbitrary build-time statements beyond literal expression records and
  imported/same-package no-argument build data functions returning `T` or
  `(T, error)`.
- Broader user Go type resolution beyond typed action decoders, user action
  logic, API handlers, and general fragment routes.
- Generated handlers beyond the supported action, API, fragment, and SSR load
  signatures. Strict production builds diagnose missing or unsupported
  action/API bindings with `backend_binding_required`; migration builds must
  opt into `--allow-missing-backend` or `Build.AllowMissingBackend` to emit
  explicit HTTP 501 stubs.

## Compatibility Notes

Generated output is still pre-1.0. Public releases must document
generated-output changes that can affect checked-in deploy recipes, generated
app imports, route manifests, asset manifests, cache headers, or generated
binary behavior.

The stable compatibility surface for v0.1 is intentionally narrow:

- `gowdk-routes.json`, `gowdk-assets.json`, and `gowdk-build-report.json`
  include explicit `version` fields.
- Generated app entrypoints expose standard `net/http` handlers through
  `gowdkapp.Handler()` and `gowdkapp.ServeMux()`.
- Generated binaries serve selected embedded output and request-time routes from
  the same build metadata used by `gowdk build --out`.
- Generated asset logical paths resolve through `gowdk-assets.json`; consumers
  should not hard-code content-hashed filenames.

Generated source layout, helper function names, private package internals, and
intermediate files under selected app/output directories may change between
pre-1.0 releases unless a reference doc explicitly names them as public.

## Target Artifacts

The target output can include:

- App-shell HTML for `spa` and `action` pages.
- Route manifest JSON.
- Generated Go app/runtime adapter code.
- Generated action handlers and typed form decoders.
- Generated API handlers.
- Generated server fragment handlers.
- Optional SSR route handlers.
- CSS processor addon artifacts.
- Embedded asset manifest.
- A generated Go command for one-binary app serving.

## Current Generated App

`gowdk build --out dist --app .gowdk/app` writes:

```text
.gowdk/app/
  go.mod
  gowdkapp/
    app.go
    SPA/
      index.html
      gowdk-routes.json
      gowdk-assets.json
      gowdk-build-report.json
  cmd/
    server/
      main.go
```

`gowdk build --out dist --app .gowdk/app --bin dist/site` then runs `go build`
for `.gowdk/app/cmd/server` and writes `dist/site`.

The generated `gowdkapp` package exposes `Handler() (http.Handler, error)` and
`ServeMux() (*http.ServeMux, error)` for `net/http`, Chi, Echo, Gin, and other
router integrations. The generated `cmd/server` entrypoint uses that same
handler, reads `GOWDK_ADDR`, defaults to `127.0.0.1:8080`, serves GET and HEAD
requests, applies `http.Server` defaults of `ReadHeaderTimeout: 5s`,
`ReadTimeout: 10s`, `WriteTimeout: 30s`, `IdleTimeout: 60s`, and
`MaxHeaderBytes: 1 MiB`, maps extensionless routes to nested `index.html`
files, and does not list directories. It exposes `/_gowdk/health` and adds
`X-GOWDK-App`, `X-GOWDK-Module`, and `X-GOWDK-Instance-ID` headers to responses.
Request-time action/API/fragment dispatch registers generated backend routes with
`runtime/app.BackendRouter` and passes the router hook into `runtime/app`.
Generated action/API body caps default to 1 MiB and use `Build.BodyLimits`
overrides when configured. Older separate action/API hook fields remain a
compatibility path for existing
generated apps.
It loads `gowdk-assets.json`, `404.html`, `500.html`, and route-local SSR
`error` pages from the embedded build output filesystem when present.
Identity comes from `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and
`GOWDK_INSTANCE_ID`; if no instance ID is provided, the app creates one at
process start from the module name, hostname, and a random token. It can also
serve auto-detected POST redirect handlers for the first supported action
subset and supported SSR/hybrid pages with or without declared `load {}`
identifier or dotted paths. SSR load functions can return safe local redirects with
`ssr.RedirectTo`/`ssr.Redirect`, and generated SSR load failures render the
route-local `error` page when declared or the optional `500.html` when
present.
Action handlers decode allowlisted form fields into named input wrappers,
cap request bodies before parsing, preserve repeated values, return HTTP 413
for oversized submissions, return HTTP 400 for unexpected fields, and return
HTTP 422 for generated required-field validation failures. Direct file inputs
and multipart action forms are rejected before generated app output because
uploads are user-owned API/server behavior. For
partial requests, generated handlers can return the first parsed action
fragment matching `X-GOWDK-Target` and expose fragment target/swap metadata in
headers. Feature-bound generated action handlers can call no-input,
`form.Values`, typed value, and typed pointer same-package Go handlers; typed
handlers decode through generated normal Go functions built from Go AST input
struct metadata. When `Build.CSRF.Enabled` is true, generated apps read a
signing secret from `Build.CSRF.SecretEnv` or `GOWDK_CSRF_SECRET`, inject hidden
CSRF token fields into served HTML POST forms, validate action POSTs before
generated decoding or user handlers run, and return HTTP 403
`invalid csrf token` with `Cache-Control: no-store` for invalid tokens. The
generated app does not run user-defined validation beyond handler logic,
handle uploads, stream hybrid responses, refresh hybrid server data in place, or
perform non-HTTP revalidation today.

Generated app source is an output artifact and sits downstream of feature
packages. Feature packages may import stable public GOWDK runtime/addon
packages, but must not import generated `gowdkapp` packages, generated
`cmd/server` packages, build output directories, or any path under the selected
generated app directory. This keeps the dependency direction one-way: generated
adapters import user feature packages and call exported handlers.

## Current SPA Route Manifest

`gowdk build` writes `gowdk-routes.json` at the output root:

```json
{
  "version": 1,
  "routes": [
    {
      "page": "home",
      "route": "/",
      "path": "index.html"
    }
  ]
}
```

The `path` field is slash-separated and relative to the selected output
directory. Dynamic app routes are recorded once for each generated concrete
route, for example `/blog/{slug}` with `=> { slug: "hello-gowdk" }` is recorded
as `/blog/hello-gowdk`.

## Current App Asset Manifest

`gowdk build` writes `gowdk-assets.json` at the output root:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.7ada5a1234b1.css"
  },
  "hashes": {
    "assets/app.css": "sha256:..."
  },
  "cache": {
    "assets/app.css": "public, max-age=31536000, immutable",
    "index.html": "public, max-age=120"
  }
}
```

The `files` map resolves logical asset names to slash-separated paths relative
to the selected output directory. `hashes` records SHA-256 content hashes for
generated assets, and `cache` records the HTTP cache policy generated binaries
should apply when serving generated assets or route HTML files. The current
implementation records CSS files emitted by CSS processors, generated page CSS
files, partial runtime assets, generated island runtime assets, generated island
source maps, and page-level `cache` policies for generated SPA HTML. It does
not record configured stylesheet URLs that were not written by the build.

## Current Build Report

`gowdk build` writes `gowdk-build-report.json` at the output root:

```json
{
  "version": 1,
  "mode": "build",
  "outputDir": "dist/site",
  "events": [
    {
      "level": "info",
      "stage": "complete",
      "kind": "build_complete",
      "message": "SPA build completed"
    }
  ]
}
```

The report records build-output generator stages even when the CLI is not run with
`--debug`. Debug mode only mirrors the structured events to stderr for humans.

## Planned Server Defaults

Generated servers must include HTTP timeouts, header/body limits, method handling, safe app asset serving, and logs that do not expose secrets or sensitive form values.

## Ownership And Licensing

Generated output ownership and license policy are documented in `../../LICENSE`
and `../engineering/generated-code-policy.md`.
