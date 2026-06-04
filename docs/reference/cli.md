# CLI Reference

The current CLI includes language tooling, an initial static HTML build command,
generated embedded static app output, and a local static serving command for
development.

## Commands

```sh
gowdk version
gowdk tokens <file.gwdk>
gowdk fmt [--write] <files>
gowdk check [--json] [--ssr] <files>
gowdk manifest [--ssr] <files>
gowdk sitemap [--ssr] <files>
gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]
gowdk serve --dir <dir> [--addr <addr>]
gowdk lsp [--ssr]
```

## Flags

- `--ssr`: enables SSR validation by adding the SSR addon to the in-memory config.
- `--json`: supported by `check`; prints editor-friendly diagnostic JSON.
- `--write`: supported by `fmt`; overwrites formatted files.
- `--config`: supported by `build`; loads a static `gowdk.config.go` subset from the given path.
- `--module`: supported by `build`; may be repeated or comma-separated, and limits discovery to selected configured modules when no explicit file list is passed.
- `--out`: supported by `build`; selects the output directory and overrides `Build.Output`.
- `--app`: supported by `build`; writes generated Go app source that embeds the selected output directory.
- `--bin`: supported by `build`; requires `--app` and compiles the generated app with `go build -o <file>`.
- `--dir`: supported by `serve`; selects the generated static output directory.
- `--addr`: supported by `serve`; selects the listen address and defaults to
  `127.0.0.1:8080`.

## Examples

```sh
go run ./cmd/gowdk check examples/basic/home.page.gwdk
go run ./cmd/gowdk check --ssr examples/basic/dashboard.page.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk build --module frontend --module backend --out /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

Current `build` accepts explicit file paths. If no files are passed, it loads
`gowdk.config.go` when present and discovers configured root `Source.Include`
globs plus configured module sources, or `**/*.gwdk` by default. `--module`
limits discovery to selected configured modules and skips root `Source.Include`;
explicit file paths still bypass discovery. A module with a name and no explicit
include uses `<module-name>/**/*.gwdk`. Build discovery excludes `.git`,
`vendor`, `node_modules`, root/module `Source.Exclude` globs, and the selected
output directory. `--out` overrides `Build.Output`; one of them is required.

Generated apps created with `--app` read `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose `/_gowdk/health`, and include identity in
`X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the generated
app creates one at process start; set it explicitly when deployment code needs a
stable ID.

Current `build` limitations: it emits only simple static HTML files,
`gowdk-routes.json`, `gowdk-assets.json`, generated embedded static app source,
and an optional static-serving binary for build-time pages with non-dynamic
routes or literal `paths {}` dynamic routes, literal `build {}` data, lowercase
HTML markup, component files with string props, and first-slice action redirect
handlers with form decoder wrappers and required-field validation.

Current generated binary limitations: it serves embedded static files and local
POST redirects for the first supported action subset, including first-slice
form input decoder wrappers and required-field validation. It does not run real
user Go type-bound action decoders, user action logic, CSRF, APIs, partial
fragments, SSR routes, or hybrid request-time behavior.

Current `serve` limitations: it serves generated static files only. It does not
run generated actions, APIs, partial fragments, or SSR routes.
