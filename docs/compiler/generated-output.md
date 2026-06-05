# Generated Output

Generated output currently covers app-shell HTML, selected browser runtime assets,
generated embedded app source, and optional local binary or Go `js/wasm`
artifacts for the implemented compiler slice.

Implemented today:

- `gowdk build [--out <dir>] [files...]` writes route-derived HTML files for simple `spa` and `action` pages.
- `.cmp.gwdk` files can be passed to build or discovered by default and expanded from self-closing component calls.
- `gowdk-routes.json` records the app route, page ID, and relative output path for emitted pages.
- `gowdk-assets.json` records generated app assets such as CSS files emitted
  by CSS processors, generated page CSS files, and the partial-update client
  runtime when needed.
- `gowdk-build-report.json` records build-output generator validation, planning,
  write, manifest, cleanup, and completion events for every successful disk
  build.
- Configured stylesheets and CSS processor stylesheet links are emitted in page
  `<head>` elements.
- CSS processors can emit CSS asset files under the output directory.
- Discovered page CSS inputs selected by implicit `default page` or explicit
  `@css` annotations are concatenated into generated page CSS files.
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
- Generated apps include POST redirect handlers for the first supported
  action subset on concrete SPA/action routes.
- Generated app creation auto-detects supported action routes and first-slice
  SSR routes from the parsed manifest used by `gowdk build --app`, so the CLI
  does not need to manually register those handler hooks.
- Generated apps include first-slice form input decoder functions that
  preserve repeated values and reject unexpected fields inferred from direct
  literal controls in same-page `g:post` forms.
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
- In the default development build mode, generated JavaScript island assets are
  accompanied by `assets/gowdk/islands/<Component>.js.map` source map files
  that reference the component `.gwdk` source, are recorded in
  `gowdk-assets.json`, include first-slice component/client/view source span
  anchors, and are linked from the JS with `sourceMappingURL`.
  `Build.Mode: gowdk.Production` omits those debug source map artifacts and
  comments and compacts generated island JavaScript by trimming
  formatting-only whitespace.
- Generated build output emits `assets/gowdk/islands/<Component>.wasm` plus
  `assets/gowdk/islands/<Component>.wasm.js` only for component calls that
  explicitly set `g:island="wasm"`. When the component declares
  `@wasm <package>`, GOWDK runs `GOOS=js GOARCH=wasm go build` for that package
  and writes the compiled browser WASM module to the component asset path.
  Local packages are checked for browser-safe imports before build; server,
  process, and network packages such as `net/http`, `os/exec`, and
  `database/sql` are rejected. A package that does not produce a WASM module
  fails the build. Components
  without `@wasm` keep the minimal placeholder module for the loader-shape
  slice. The loader discovers matching island roots, builds the ADR-defined
  bootstrap object from state, props, emits, refs, and binding metadata, calls
  component-scoped WASM exports when present, captures host DOM events, and
  applies validated first-slice patch commands such as text, visibility,
  attribute, class, style, and emitted-event updates.
- Generated apps can serve first-slice concrete and dynamic SSR pages
  without `load {}`. Dynamic route params are substituted into generated SSR
  placeholders with request-time HTML escaping.
- Generated apps can return first-slice partial fragment responses from
  action handlers for `X-GOWDK-Partial` requests.
- Generated app action route extraction rejects direct file inputs and
  multipart `g:post` forms until upload security rules are defined.
- `internal/codegen.GenerateRouteRegistration` can emit formatted Go route
  registration source from `internal/codegen.RouteBinding` plans for future
  generated `internal/routes` packages.
- `internal/codegen.GenerateComponentPackage` can emit formatted Go render
  functions for current `.cmp.gwdk` components with string props and direct
  `{prop}` interpolation. Generated component code writes compiler-owned
  chunks through `runtime/render.Builder.Markup` and expression output through
  `runtime/render.Builder.Text`, which escapes by default.
- `internal/gotypes` resolves component prop/state structs through Go module
  import paths using `go list`, `go/parser`, and `go/types`.
- `runtime/response` defines fragment responses with target and swap metadata
  for generated and future partial handlers.
- `/` maps to `index.html`.
- `/patients` maps to `patients/index.html`.
- Current asset names are stable and deterministic rather than content-hashed.
- Generated embedded apps skip local environment files, source maps, source
  files, VCS/dependency directories, and common temporary artifacts when copying
  build output into the embedded app.
- Generated embedded apps load `gowdk-assets.json` from the embedded filesystem
  when present and expose the loaded asset count through `/_gowdk/health`.

Not implemented yet:

- Route params passed into imported Go `build {}` functions.
- CSS hashing, minification, and full page-aware third-party CSS processor
  selection.
- Non-string props in legacy `props {}` blocks.
- General expression interpolation and arbitrary `build {}` execution.
- Real user Go type resolution for typed action decoders, user action logic,
  CSRF, API handlers, general fragment routes, and SSR `load {}` handlers.

## Target Artifacts

The target output can include:

- App-shell HTML for `spa` and `action` pages.
- Route manifest JSON.
- Generated Go route registration.
- Generated Go component render functions.
- Generated action handlers and typed form decoders.
- Generated API handlers.
- Generated server fragment handlers.
- Optional SSR route handlers.
- CSS/plugin artifacts.
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
requests, maps extensionless routes to nested `index.html` files, and does not
list directories. It exposes `/_gowdk/health` and adds
`X-GOWDK-App`, `X-GOWDK-Module`, and `X-GOWDK-Instance-ID` headers to responses.
It loads `gowdk-assets.json` from the embedded build output filesystem when present.
Identity comes from `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and
`GOWDK_INSTANCE_ID`; if no instance ID is provided, the app creates one at
process start from the module name, hostname, and a random token. It can also
serve auto-detected POST redirect handlers for the first supported action
subset and first-slice SSR pages that do not use `load {}`. Those action
handlers decode allowlisted form fields into named first-slice input wrappers,
cap request bodies before parsing, preserve repeated values, return HTTP 413
for oversized submissions, return HTTP 400 for unexpected fields, and return
HTTP 422 for first-slice required-field validation failures. Direct file inputs
and multipart action forms are rejected before generated app output. For
partial requests, generated handlers can return the first parsed action
fragment matching `X-GOWDK-Target` and expose fragment target/swap metadata in
headers. The generated app does not execute user action logic, enforce CSRF,
resolve real user Go input structs, run user-defined validation, handle uploads,
or serve API handlers, general fragment routes, `load {}` SSR, guards, or
hybrid request-time handlers today.

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
    "assets/app.css": "assets/app.css"
  }
}
```

The `files` map resolves logical asset names to slash-separated paths relative
to the selected output directory. The current implementation records CSS files
emitted by CSS processors, generated page CSS files, partial runtime assets,
generated island runtime assets, and generated island source maps. It does not
record configured stylesheet URLs that were not written by the build.

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
