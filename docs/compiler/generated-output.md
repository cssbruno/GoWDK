# Generated Output

Generated output has started with static HTML emission for simple build-time pages.

Implemented today:

- `gowdk build [--out <dir>] [files...]` writes route-derived HTML files for simple `static` and `action` pages.
- `.cmp.gwdk` files can be passed to build or discovered by default and expanded from self-closing component calls.
- `gowdk-routes.json` records the static route, page ID, and relative output path for emitted pages.
- `gowdk-assets.json` records generated static assets such as CSS files emitted by CSS processors.
- Configured stylesheets and CSS processor stylesheet links are emitted in page
  `<head>` elements.
- CSS processors can emit CSS asset files under the output directory.
- Dynamic static routes with literal `paths {}` declarations are expanded by
  `gowdk build`.
- Literal dynamic route params can render in the current static `view {}`
  interpolation subset.
- Literal `build {}` data can render in the current static `view {}`
  interpolation subset.
- `gowdk build --app <dir>` writes a generated Go module that embeds the static
  output under `<dir>/static`.
- `gowdk build --bin <file>` requires `--app` and compiles that generated app
  into one local static-serving binary.
- Generated static apps include POST redirect handlers for the first supported
  action subset on concrete static/action routes.
- Generated static apps include first-slice form input decoder functions that
  preserve repeated values and reject unexpected fields inferred from direct
  static controls in same-page `g:post` forms.
- Generated static apps return HTTP 422 for missing or empty direct static
  `required` fields when the action declares `valid(input)?`.
- Generated static app action route extraction rejects direct file inputs and
  multipart `g:post` forms until upload security rules are defined.
- `/` maps to `index.html`.
- `/patients` maps to `patients/index.html`.

Not implemented yet:

- Route params bound into `build {}` expressions.
- CSS class extraction, hashing, minification, and third-party CSS tool
  integrations.
- Component children, slots, and non-string props.
- General expression interpolation and arbitrary `build {}` execution.
- Real user Go type resolution for typed action decoders, user action logic, CSRF, API handlers,
  fragment handlers, and SSR handlers.

## Target Artifacts

The target output can include:

- Static HTML for `static` and `action` pages.
- Route manifest JSON.
- Generated Go component render functions.
- Generated action handlers and typed form decoders.
- Generated API handlers.
- Generated server fragment handlers.
- Optional SSR route handlers.
- CSS/plugin artifacts.
- Embedded asset manifest.
- A generated Go command for one-binary static serving.

## Current Generated Static App

`gowdk build --out dist --app .gowdk/app` writes:

```text
.gowdk/app/
  go.mod
  main.go
  static/
    index.html
    gowdk-routes.json
    gowdk-assets.json
```

`gowdk build --out dist --app .gowdk/app --bin dist/site` then runs `go build`
inside `.gowdk/app` and writes `dist/site`.

The generated app reads `GOWDK_ADDR`, defaults to `127.0.0.1:8080`, serves GET
and HEAD requests, maps extensionless routes to nested `index.html` files, and
does not list directories. It exposes `/_gowdk/health` and adds
`X-GOWDK-App`, `X-GOWDK-Module`, and `X-GOWDK-Instance-ID` headers to responses.
Identity comes from `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and
`GOWDK_INSTANCE_ID`; if no instance ID is provided, the app creates one at
process start from the module name, hostname, and a random token. It can also
serve POST redirect handlers for the first supported action subset. Those
handlers decode allowlisted form fields into named first-slice input wrappers,
preserve repeated values, return HTTP 400 for unexpected fields, and return
HTTP 422 for first-slice required-field validation failures. Direct file inputs
and multipart action forms are rejected before generated app output. The
generated app does not execute user action logic, enforce CSRF, resolve real
user Go input structs, run user-defined validation, handle uploads, or serve
API, fragment, SSR, or hybrid request-time handlers today.

## Current Static Route Manifest

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
directory. Dynamic static routes are recorded once for each generated concrete
route, for example `/blog/{slug}` with `=> { slug: "hello-gowdk" }` is recorded
as `/blog/hello-gowdk`.

## Current Static Asset Manifest

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
emitted by CSS processors. It does not record configured stylesheet URLs that
were not written by the build.

## Planned Server Defaults

Generated servers must include HTTP timeouts, header/body limits, method handling, safe static asset serving, and logs that do not expose secrets or sensitive form values.

## Ownership And Licensing

Generated output ownership and license policy are documented in `LICENSE.md` and `docs/engineering/generated-code-policy.md`.
