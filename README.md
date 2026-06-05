<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

Welcome to GOWDK, a portable Go web compiler for people who would rather ship a
Go binary than babysit a JavaScript dependency graph.

GOWDK exists because I am fed up with npm being the default answer to every web
UI problem. The ecosystem is powerful, but the constant churn, huge dependency
trees, and recurring supply-chain attacks are not a price I want to keep paying
for ordinary product apps.

The idea is simple: write movable `.gwdk` files, compile them first, and ship
static output or a single Go binary. No React required. No Svelte required. No
npm dependency pile unless you deliberately choose one.

Live demo: [gowdk.com](https://gowdk.com/) is the public project site and a
real GOWDK-built app. It shows the pitch in practice: portable `.gwdk` files,
compile-first output, and a small Go-centered path from source to shipped web
UI. Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page).

## Why

Modern frontend stacks often make small apps feel heavier than they need to be:
install half the internet, run a dev server, wire a bundler, accept a lockfile
the size of a novella, then hope the next transitive package is not compromised.

That may be normal now. It does not have to be the only path.

GOWDK is a Go-first path for building web apps with less moving machinery:

- Go-first web apps.
- Static output by default.
- Backend actions without full-page SSR.
- Optional SSR only when a page actually needs request-time rendering.
- A path toward one-binary deploys with embedded frontend assets.
- Small dependency surface.
- Less npm by default.

WDK does not have a canonical expansion. No one knows what it stands for.
GOWDK just ships apps.

## What It Is

GOWDK compiles `.gwdk` source files into web output. A page declares its route
inside the file, so files stay portable instead of being coupled to framework
folder magic.

Current compiler slices include:

- `@page`, `@route`, `@layout`, and `@render` metadata.
- Static page generation.
- Components with default slots.
- Literal `paths {}` dynamic static routes.
- Literal `build {}` data and first-slice imported Go build functions for
  static rendering.
- First-slice typed action parsing, generated action redirects, and generated
  partial fragment responses.
- API route metadata.
- First-slice generated SSR routes for simple concrete `@render ssr` pages.
- CSS discovery, page CSS output, and CSS processor hooks.
- Local static serving.
- Embedded static app generation and optional binary compilation.
- Generated Go WASM artifacts from embedded apps.
- Module-selected generated apps and binaries, so one binary can embed one
  module, multiple modules, or all discovered modules.
- CLI, formatter, diagnostics, manifest output, sitemap output, routes output,
  and a VS Code extension.

This project is still early. It is useful as a compiler/runtime foundation and
for experiments, but not yet a production framework.

## Quick Start

```sh
go run ./cmd/gowdk init my-app
cd my-app
go run ../cmd/gowdk build
go run ../cmd/gowdk serve --dir dist/site
```

You can also build an example directly:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

For the local edit loop, use `dev`. It builds, serves, watches source hashes,
and live-reloads the browser after successful rebuilds:

```sh
go run ./cmd/gowdk dev --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## CLI

```sh
gowdk init [--force] [dir]
gowdk tokens <file.gwdk>
gowdk fmt [--write] <file.gwdk>
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk dev [--addr 127.0.0.1:8080] [--interval 1s] [build flags...]
gowdk watch [--once] [--restart] [--interval 1s] [build flags...]
gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
gowdk lsp [--ssr]
```

Static build targets can be declared in `gowdk.config.go`. With `Build.Targets`
present, `gowdk build` runs all targets and `gowdk build --target <name>` runs
one selected target:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{Name: "admin", Modules: []string{"admin"}, Output: "dist/admin", App: ".gowdk/admin", Binary: "bin/admin", WASM: "bin/admin.wasm"},
		{Name: "public-admin", Modules: []string{"public", "admin"}, Output: "dist/app", App: ".gowdk/app", Binary: "bin/app"},
	},
}
```

Ad hoc `--module`, `--out`, `--app`, `--bin`, and `--wasm` flags still work
for one-off builds and may be repeated or comma-separated where applicable.
Every successful disk build writes `gowdk-build-report.json`; pass `--debug`
to mirror that structured report to stderr during build, dev, or watch runs.

`--wasm <file>` requires `--app <dir>` and compiles the generated app with
`GOOS=js GOARCH=wasm`:

```sh
gowdk build --out dist/app --app .gowdk/app --wasm bin/app.wasm
```

During source development, use `go run ./cmd/gowdk ...` instead of `gowdk ...`.

For a local redeploy loop, compile a binary and let `watch` restart it after
successful rebuilds:

```sh
gowdk watch --restart --target admin
gowdk watch --restart --out dist/app --app .gowdk/app --bin bin/app
```

`watch` compares input content hashes, can incrementally render changed static
page sources for plain `--out` builds, and generated static/app files are not
rewritten when their bytes are unchanged. Failed rebuilds leave the current
process running.

## Tiny Example

```gwdk
@page home
@route "/"

build {
  => { title: "Hello from GOWDK" }
}

view {
  <main>
    <h1>{title}</h1>
    <p>Compiled by Go. Served as plain HTML.</p>
  </main>
}
```

## Project Direction

GOWDK is compile-first and static/action-first:

```text
Core GOWDK:
  Build static pages and app output first.

Actions:
  Run backend mutations without making the whole page SSR.

Partials:
  Swap server fragments instead of turning everything into a SPA.

SSR Addon:
  Render selected pages at request time when they need request context.
```

Default render mode is `static`.

The first generated SSR slice supports concrete `@render ssr` pages that render
from `view {}` plus literal or imported `build {}` data. `load {}`, guards,
dynamic SSR routes, and user request-time logic are still being built.

## Code Quality Tools

Use Go's standard tools for repository changes:

```sh
gofmt -w <changed-go-files>
go test ./...
go build ./cmd/gowdk
```

For the VS Code extension, use Node's built-in checks:

```sh
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
```

GOWDK also ships source-quality tools for `.gwdk` files:

```sh
gowdk fmt --write <file.gwdk>
gowdk check [files...]
gowdk tokens <file.gwdk>
```

## Verification

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
```

## Docs

- `docs/product/vision.md`: product intent.
- `docs/product/requirements.md`: current requirements.
- `docs/product/roadmap.md`: planned phases.
- `docs/reference/cli.md`: command reference.
- `docs/language/`: `.gwdk` language notes.
- `examples/README.md`: runnable examples.

## License

Apache-2.0. See `LICENSE` and `LICENSE.md`.
