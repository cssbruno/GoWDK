# Generated Output

`gowdk build` emits inspectable files only for declared and supported source
surfaces. Generated directories are outputs, not authoring locations.

## Main Outputs

| Output | Purpose |
| --- | --- |
| `<out>/` | Static/SPA HTML, assets, runtime-private manifests, build-private reports, and optional SEO files |
| `<app>/` | Generated Go application source for embedded assets and request-time handlers |
| `<bin>` | Optional compiled generated application binary |
| `<wasm>` | Optional Go `js/wasm` deploy artifact |
| role output dirs | Optional worker or cron role apps from configured contract targets |

The default scaffold writes `.gowdk/output/<target>`, `.gowdk/<target>`, and
`bin/<target>` through `Build.Targets`.

## Static Output

Build-time pages can emit route-derived `index.html` files, content-hashed CSS
and asset files, generated browser runtime assets, runtime-private manifests
such as `gowdk-routes.json` and `gowdk-assets.json`, and build-private reports
such as `gowdk-build-report.json`.

Only browser-facing artifacts are served by `gowdk serve` and generated app
static handlers. HTML, generated assets under `assets/`, `sitemap.xml`,
`robots.txt`, `openapi.json`, and `asyncapi.json` are public by default.
`gowdk-routes.json` and `gowdk-assets.json` are embedded for generated runtime
use but are not public URLs. Build reports, timings, security/audit data,
source maps, source files, and unknown files under the output directory are not
served by default.

Dynamic SPA routes emit concrete files only for supported `paths {}` entries.
Request-time SSR or hybrid pages are listed in metadata but are served by the
generated app instead of static HTML.

## Generated App Source

`--app <dir>` writes normal Go source that registers routes, embeds output,
connects generated endpoint adapters, starts lifecycle services, and exposes
supported hooks for guards, rate limits, contracts, tracing, and addon runtime
configuration.

Generated source is adapter glue. User behavior stays in normal Go packages or
in supported extracted `go {}` / `go server {}` slices materialized under the
generated `gowdk_go/` package.

## Generated Binary

`--bin <file>` compiles the generated app. The binary can serve embedded build
output plus generated action, API, fragment, guard, SSR, hybrid, contract, and
realtime surfaces that were enabled and validated at build time.

Generated binaries speak HTTP. TLS, public host routing, secrets, durable
storage, process supervision, and backups remain deployment responsibilities.

## Reports And Manifests

| File | Source |
| --- | --- |
| `gowdk-build-report.json` | Build-private planning, writes, cache policy, security posture, contracts, OpenAPI/AsyncAPI metadata, and diagnostics |
| `gowdk-routes.json` | Runtime-private emitted page routes and generated output paths |
| `gowdk-assets.json` | Runtime-private generated assets, cache policy, CSS, JS, WASM, and component assets |
| `gowdk-security.json` | Non-served posture report for generated-app security checks |
| `sitemap.xml` / `robots.txt` | Optional public SEO addon output |
| `openapi.json` / `asyncapi.json` | Public API documentation artifacts when generated |

The public `gowdk manifest` command is documented in
[Reference Manifest](../reference/manifest.md). Build-report details live in
[Build Report](build-report.md).

## Ownership Rules

- Do not edit generated directories by hand.
- Do not commit generated app output unless a fixture explicitly requires it.
- Keep generated Go deterministic and formatted with `go/format`.
- Keep generated browser code compiler-owned; user JavaScript remains optional
  assets or explicit page code.
- Document new generated files in this page, the build report page, or the
  reference page that owns the public contract.
- New generated artifact kinds must declare whether they are public,
  runtime-private, or build-private before they are embedded, served, or
  packaged.
- Build destination topology is validated before filesystem writes. Selected
  targets must not share output/app/binary/WASM/report/recipe destinations, put
  binaries or WASM under served static output, put generated apps under static
  output, or nest one selected target output inside another.
- Generated output ownership and license policy are documented in
  [LICENSE](../../LICENSE) and
  [Generated Code Policy](../engineering/generated-code-policy.md).

## Generated App Contracts

Generated app packages expose `App()`, `Handler()`, `ServeMux()`, and
`RegisterMiddleware` for generated-binary startup and `net/http` integration.
The generated server uses `GOWDK_ADDR`, applies bounded HTTP server timeouts,
serves `/_gowdk/health`, loads optional generated error pages, and emits
configured security and identity headers.

When the generated frontend or backend app directory is under `.gowdk/` inside
the application module, GOWDK emits it as normal package source in that module
and does not write a nested `go.mod`. Its imports use the application module
path, so app-owned `internal/` packages, `replace` directives, vendoring, and
workspace settings resolve the same way as the rest of the app. Explicit legacy
app directories outside `.gowdk/`, plus generated worker and cron role apps,
keep a nested generated module.

Generated backend routes are registered through `runtime/app.BackendRouter`.
Action, API, fragment, command, query, SSR, hybrid, realtime, guard, rate-limit,
CSRF, CORS, and tracing behavior is included only when declared, enabled, and
validated for the selected build.

Worker and cron role outputs follow the same generated-app rule: they are
normal Go modules downstream of contract metadata, and `--worker-bin` /
`--cron-bin` compile their generated commands with `go build`.
