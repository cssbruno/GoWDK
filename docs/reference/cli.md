# CLI Reference

The current CLI includes language tooling, an initial build-output command,
generated embedded app output, and a local output-serving command for
development.

## Commands

```sh
gowdk version
gowdk init [--force] [dir]
gowdk tokens <file.gwdk>
gowdk fmt [--write] <files>
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]
gowdk dev [--addr <addr>] [--interval <duration>] [build flags...]
gowdk serve --dir <dir> [--addr <addr>]
gowdk lsp [--ssr]
```

## Flags

- `--ssr`: enables SSR validation by adding the SSR addon to the in-memory config.
- `--force`: supported by `init`; overwrites starter files that already exist.
- `--json`: supported by `check`; prints editor-friendly diagnostic JSON.
- `--write`: supported by `fmt`; overwrites formatted files.
- `--config`: supported by `check`, `manifest`, `sitemap`, `routes`, and `build`; loads a literal config subset from the given path instead of the required default `gowdk.config.go`.
- `--debug`: supported by `build` and forwarded by `dev`; prints the structured SPA build report to stderr while generated paths remain on stdout.
- `--allow-missing-backend`: supported by `build` and forwarded by `dev`; in production mode, allows missing or unsupported action/API handlers to generate HTTP 501 stubs instead of failing the build.
- `--target`: supported by `build`; may be repeated or comma-separated, and runs selected `Build.Targets` entries.
- `--module`: supported by `check`, `manifest`, `sitemap`, `routes`, and `build`; may be repeated or comma-separated, and limits discovery to selected configured modules when no explicit file list is passed.
- `--out`: supported by `build`; selects the output directory and overrides `Build.Output`.
- `--app`: supported by `build`; writes generated Go app source that embeds the selected output directory.
- `--bin`: supported by `build`; requires `--app` and compiles the generated app with `go build -o <file>`.
- `--wasm`: supported by `build`; requires `--app` and compiles the generated app with `GOOS=js GOARCH=wasm go build -o <file>`.
- `--backend-app`: supported by `build`; writes generated backend-only Go app source for feature-bound action/API endpoints.
- `--backend-bin`: supported by `build`; requires `--backend-app` and compiles the generated backend app with `go build -o <file>`.
- `--addr`: supported by `dev` and `serve`; selects the listen address and defaults to `127.0.0.1:8080`.
- `--interval`: supported by `dev`; sets the polling interval, such as `500ms`, `1s`, or `2s`.
- `--dir`: supported by `serve`; selects the generated build output directory.

## Examples

```sh
go run ./cmd/gowdk init my-site
go run ./cmd/gowdk check examples/pages/home.page.gwdk
go run ./cmd/gowdk check --config gowdk.config.go
go run ./cmd/gowdk check --ssr examples/ssr/dashboard.page.gwdk
go run ./cmd/gowdk manifest --module frontend --ssr
go run ./cmd/gowdk sitemap --module frontend --ssr
go run ./cmd/gowdk routes --module frontend --ssr
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --debug --out /tmp/gowdk-build examples/pages/home.page.gwdk
go run ./cmd/gowdk build --allow-missing-backend --out /tmp/gowdk-build examples/actions/signup.page.gwdk
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/ssr/simple-ssr.page.gwdk
go run ./cmd/gowdk build --module frontend --module backend --out /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --wasm /tmp/gowdk-site.wasm examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --bin bin/admin
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --wasm bin/admin.wasm
go run ./cmd/gowdk build --module public,admin --out dist/app --app .gowdk/app --bin bin/app
go run ./cmd/gowdk build --target admin
go run ./cmd/gowdk dev --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk dev --target admin --addr 127.0.0.1:8090
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

`init` creates a buildable starter project in the selected directory, or the
current directory when omitted:

```text
gowdk.config.go
src/pages/home.page.gwdk
src/components/hero.cmp.gwdk
styles/global.css
```

The generated config discovers `src/**/*.gwdk`, writes build output to
`dist/site`, discovers CSS under `styles/**/*.css`, and writes `.gitignore`
with `gowdk_cache/` for the default dev output. Existing starter files
are not overwritten unless `--force` is passed.

`check`, `manifest`, `sitemap`, `routes`, `build`, and `dev` require a config
file before they compile or validate `.gwdk` code. By default they load
`gowdk.config.go` from the current directory; `--config <file>` can point at a
different config for project examples or one-off checks.

These commands accept explicit file paths, but explicit paths do not remove the
config requirement. If no files are passed, commands discover configured root
`Source.Include` globs plus configured module sources, or `**/*.gwdk` when the
loaded config does not declare source includes. `--module` limits discovery to
selected configured modules and skips root `Source.Include`; explicit file
paths still bypass discovery. A module with a name and no explicit include uses
`<module-name>/**/*.gwdk`. Discovery excludes `.git`, `vendor`, `node_modules`,
`testdata`, root/module `Source.Exclude` globs, and the configured build output
directory when one exists. `build --out` overrides `Build.Output`; one of them
is required for `build`. Every successful disk build writes
`gowdk-build-report.json` to the output root. Passing `--debug` prints the same
build report to stderr for validation, planning, write, manifest, cleanup, and
completion events without changing stdout artifact-path output. `gowdk dev`
uses `gowdk_cache` as its default output directory unless `--out <dir>` or a
selected build target supplies an explicit output.

For generated apps and binaries, the selected modules are the packaging set:
`--app` copies the selected build output; `--bin` and `--wasm` embed it. Prefer
`Build.Targets` in `gowdk.config.go` for repeatable packaging. With targets
configured, `gowdk build` runs all targets when no ad hoc output, module, app,
binary, or explicit file arguments are passed; `gowdk build --target <name>`
runs selected targets. `--target` cannot be combined with `--module`, `--out`,
`--app`, `--bin`, `--wasm`, or explicit files. The ad hoc flags remain useful
for one-off builds.

`--wasm` produces a Go `js/wasm` compile artifact from the generated app. This
is a deploy artifact for hosts that can run Go WebAssembly; it is separate from
explicit browser island assets emitted by `g:island="wasm"`.

`dev` is the one-command SPA development loop. It forwards non-dev flags to
`build`, resolves the output directory from `--out`, exactly one selected
`Build.Targets` entry, or the default `gowdk_cache` dev output, serves that
directory, polls explicit or discovered build inputs plus the loaded config
file, rebuilds after content changes, and injects a tiny server-sent-events
live-reload script into served HTML pages. Rebuild failures are printed and the
last successful output keeps serving.

Build output, route/asset manifests, generated `go.mod`, generated
`gowdkapp/app.go`, generated `cmd/server/main.go`, and embedded build output
files are only rewritten when their bytes change, which keeps local dev loops
from retriggering on no-op generation. For plain SPA `--out` builds, page-only
source edits use an incremental SPA renderer that validates the full manifest,
refreshes manifests, writes only changed page output, and removes stale route
output for changed pages. Component, layout, CSS, config, source-set, target,
app, binary, and WASM changes use the full build path.

Generated apps created with `--app` read `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose `/_gowdk/health`, and include identity in
`X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the generated
app creates one at process start; set it explicitly when deployment code needs a
stable ID. `GOWDK_MODULE_NAME` is runtime identity metadata; it does not change
which modules were embedded. Embedded module composition is fixed at build time
by `Build.Targets` or the selected `--module` flags.

`gowdk routes` prints validated route and endpoint metadata as JSON. The current
schema is version `1`. The `routes` list is limited to page/file route kinds
such as `static`, `spa`, `ssr`, and `hybrid`; backend actions and APIs appear
in the separate `endpoints` list with source path, `.gwdk` package, method,
path, page ID, planned adapter handler, and backend binding metadata. Backend
binding metadata includes the Go package name, import path when known, handler
symbol, signature/input metadata when bound, status, and binding message.
Non-fatal route-mode notes, such as SSR disabled on a SPA route or static SPA
output disabled on an SSR route, appear in `info` and are also mirrored to
stderr as `info:` console lines.

Current `build` limitations: it emits only simple app-shell HTML files,
`gowdk-routes.json`, `gowdk-assets.json`, generated embedded app source,
and an optional generated binary for build-time pages with non-dynamic
routes or literal `paths {}` dynamic routes, literal `build {}` data, lowercase
HTML markup, first-slice imported Go build data functions, component files with
string props, first-slice action redirect handlers with form decoder wrappers
and required-field validation, and first-slice action fragment responses for
partial requests.

Current generated binary limitations: it serves embedded build output files for
the selected build output and local POST endpoints for the first supported
action subset, including first-slice form input decoder wrappers and
required-field validation. It can also serve first-slice action fragment
responses for `X-GOWDK-Partial` requests, CSRF validation when
`Build.CSRF.Enabled` is set, and first-slice concrete or dynamic SSR pages
rendered from `view {}` and literal or imported `build {}` data. It does not
run general fragment endpoints, `load {}` execution, guards, or hybrid
request-time behavior.

Current `serve` limitations: it serves generated build output files only. It does not
run generated actions, APIs, partial fragments, or SSR routes.
