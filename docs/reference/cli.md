# CLI Reference

The current CLI includes language tooling, an initial static HTML build command,
generated embedded static app output, and a local static serving command for
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
gowdk build [--config <file>] [--ssr] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk watch [--once] [--restart] [--interval <duration>] [build flags...]
gowdk serve --dir <dir> [--addr <addr>]
gowdk lsp [--ssr]
```

## Flags

- `--ssr`: enables SSR validation by adding the SSR addon to the in-memory config.
- `--force`: supported by `init`; overwrites starter files that already exist.
- `--json`: supported by `check`; prints editor-friendly diagnostic JSON.
- `--write`: supported by `fmt`; overwrites formatted files.
- `--config`: supported by `check`, `manifest`, `sitemap`, `routes`, and `build`; loads a static `gowdk.config.go` subset from the given path.
- `--target`: supported by `build`; may be repeated or comma-separated, and runs selected `Build.Targets` entries.
- `--module`: supported by `check`, `manifest`, `sitemap`, `routes`, and `build`; may be repeated or comma-separated, and limits discovery to selected configured modules when no explicit file list is passed.
- `--out`: supported by `build`; selects the output directory and overrides `Build.Output`.
- `--app`: supported by `build`; writes generated Go app source that embeds the selected output directory.
- `--bin`: supported by `build`; requires `--app` and compiles the generated app with `go build -o <file>`.
- `--wasm`: supported by `build`; requires `--app` and compiles the generated app with `GOOS=js GOARCH=wasm go build -o <file>`.
- `--once`: supported by `watch`; runs one build using the forwarded build flags and exits.
- `--restart`: supported by `watch`; restarts one generated binary after each successful rebuild.
- `--interval`: supported by `watch`; sets the polling interval, such as `500ms`, `1s`, or `2s`.
- `--dir`: supported by `serve`; selects the generated static output directory.
- `--addr`: supported by `serve`; selects the listen address and defaults to
  `127.0.0.1:8080`.

## Examples

```sh
go run ./cmd/gowdk init my-site
go run ./cmd/gowdk check examples/basic/home.page.gwdk
go run ./cmd/gowdk check --config gowdk.config.go
go run ./cmd/gowdk check --ssr examples/basic/dashboard.page.gwdk
go run ./cmd/gowdk manifest --module frontend --ssr
go run ./cmd/gowdk sitemap --module frontend --ssr
go run ./cmd/gowdk routes --module frontend --ssr
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/basic/simple-ssr.page.gwdk
go run ./cmd/gowdk build --module frontend --module backend --out /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --wasm /tmp/gowdk-site.wasm examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --bin bin/admin
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --wasm bin/admin.wasm
go run ./cmd/gowdk build --module public,admin --out dist/app --app .gowdk/app --bin bin/app
go run ./cmd/gowdk build --target admin
go run ./cmd/gowdk watch --interval 1s --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk watch --restart --target admin
go run ./cmd/gowdk watch --restart --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
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
`dist/site`, and discovers CSS under `styles/**/*.css`. Existing starter files
are not overwritten unless `--force` is passed.

`check`, `manifest`, `sitemap`, `routes`, and `build` accept explicit file
paths. If no files are passed, they load `gowdk.config.go` when present and discover
configured root `Source.Include` globs plus configured module sources, or
`**/*.gwdk` by default. `--module` limits discovery to selected configured
modules and skips root `Source.Include`; explicit file paths still bypass
discovery. A module with a name and no explicit include uses
`<module-name>/**/*.gwdk`. Discovery excludes `.git`, `vendor`, `node_modules`,
root/module `Source.Exclude` globs, and the configured build output directory
when one exists. `build --out` overrides `Build.Output`; one of them is required
for `build`.

For generated apps and binaries, the selected modules are the packaging set:
`--app` copies the selected build output; `--bin` and `--wasm` embed it. Prefer
`Build.Targets` in `gowdk.config.go` for repeatable packaging. With targets
configured, `gowdk build` runs all targets when no ad hoc output, module, app,
binary, or explicit file arguments are passed; `gowdk build --target <name>`
runs selected targets. `--target` cannot be combined with `--module`, `--out`,
`--app`, `--bin`, `--wasm`, or explicit files. The ad hoc flags remain useful
for one-off builds.

`--wasm` produces a Go `js/wasm` compile artifact from the generated app. This
is a deploy artifact for hosts that can run Go WebAssembly; it is not the future
browser WASM-islands feature.

`watch` forwards all non-watch flags and file paths to `build`. It runs an
initial build, then polls explicit or discovered build inputs plus
`gowdk.config.go` when present and rebuilds when the file set or content hashes
change. It intentionally uses polling instead of a platform-specific file
watcher dependency. Static output, route/asset manifests, generated `main.go`,
generated `go.mod`, and embedded static files are only rewritten when their
bytes change, which keeps watch loops from retriggering on no-op generation.
For plain static `--out` builds, page-only source edits use an incremental
static renderer that validates the full manifest, refreshes manifests, writes
only changed page output, and removes stale route output for changed pages.
Component, layout, CSS, config, source-set, target, app, binary, WASM, and
restart changes use the full build path.

`watch --restart` creates an Air-like local redeploy loop without depending on
Air. It infers the binary from ad hoc `--bin <file>` or from exactly one
configured `Build.Targets` entry with `Binary`. After each successful build, it
interrupts the previous process, kills it if it does not stop quickly, and
starts the new binary. Failed rebuilds leave the current process running.
`--restart` cannot be combined with `--once`.

Generated apps created with `--app` read `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose `/_gowdk/health`, and include identity in
`X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the generated
app creates one at process start; set it explicitly when deployment code needs a
stable ID. `GOWDK_MODULE_NAME` is runtime identity metadata; it does not change
which modules were embedded. Embedded module composition is fixed at build time
by `Build.Targets` or the selected `--module` flags.

`gowdk routes` prints the validated route-binding plan as JSON. The current
schema is version `1` and includes route kind, method, route pattern, page ID,
and planned handler symbol or static embedded asset handler expression.

Current `build` limitations: it emits only simple static HTML files,
`gowdk-routes.json`, `gowdk-assets.json`, generated embedded static app source,
and an optional static-serving binary for build-time pages with non-dynamic
routes or literal `paths {}` dynamic routes, literal `build {}` data, lowercase
HTML markup, first-slice imported Go build data functions, component files with
string props, and first-slice action redirect handlers with form decoder
wrappers and required-field validation.

Current generated binary limitations: it serves embedded static files for the
selected build output and local POST redirects for the first supported action
subset, including first-slice form input decoder wrappers and required-field
validation. It also serves first-slice concrete SSR pages rendered from
`view {}` and literal or imported `build {}` data. It does not run real user Go
type-bound action decoders, user action logic, CSRF, APIs, partial fragments,
`load {}` execution, dynamic SSR routes, guards, or hybrid request-time
behavior.

Current `serve` limitations: it serves generated static files only. It does not
run generated actions, APIs, partial fragments, or SSR routes.
