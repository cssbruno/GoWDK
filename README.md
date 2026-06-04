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
- Literal `build {}` data for static rendering.
- First-slice typed action parsing and generated action redirects.
- API route metadata.
- First-slice generated SSR routes for simple concrete `@render ssr` pages.
- CSS discovery, page CSS output, and CSS processor hooks.
- Local static serving.
- Embedded static app generation and optional binary compilation.
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

## CLI

```sh
gowdk init [--force] [dir]
gowdk tokens <file.gwdk>
gowdk fmt [--write] <file.gwdk>
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]
gowdk watch [--once] [--interval 1s] [build flags...]
gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
gowdk lsp [--ssr]
```

During source development, use `go run ./cmd/gowdk ...` instead of `gowdk ...`.

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
from `view {}` plus literal `build {}` data. `load {}`, guards, dynamic SSR
routes, and user request-time logic are still being built.

## Verification

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
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
