<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)
![Build](https://img.shields.io/badge/build-succeeds-2ea44f)
![Quality](https://img.shields.io/badge/quality-gated-2ea44f)

GOWDK ships Go web apps through GOWDK Compiler plus GOWDK Runtime.

- **GOWDK Compiler** turns package-peer `.gwdk` files into page output,
  manifests, assets, diagnostics, and generated Go adapter source.
- **GOWDK Runtime** serves the generated app, routes backend behavior, runs
  actions/APIs/fragments/SSR hooks, and owns the typed contract runtime.
- **`gowdk`** is the CLI that drives both layers.

Full pages default to build-time SPA output. Backend actions, APIs, fragments,
commands, and queries run at request time without forcing full-page SSR. Pages
that need request data opt into request-time page rendering with `@render ssr`.

**Status: pre-release.** The compiler and runtime already cover build-time
pages, components, generated JS islands, component-level WASM islands, CSRF-wired
actions, partial fragments, action/API handlers, contract references, and
concrete or dynamic SSR pages with declared `load {}` fields. The language and
runtime contracts are still evolving. Not ready for production use.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)

## Event-Driven Runtime

GOWDK is not only page compilation. `runtime/contracts` provides the first
event-driven backend slice:

```text
frontend UI event -> command/query -> backend handler -> backend-owned event
frontend <- result or presentation event
```

- `g:command` on forms declares backend command intent.
- `g:query` on elements declares readonly backend query intent.
- Commands have one owner handler.
- Queries read state and should not mutate it.
- Domain and integration events are emitted after backend command success.
- Presentation events can notify UI; they are not trusted input.
- Jobs and worker-style event replay are part of the same contract model.

The local in-process registry works today for single-binary apps. Compiler
scanning, generated web adapters, role filtering, contract graph/trace CLI
commands, event envelope capture/replay, and a dependency-free file outbox are
implemented. Split worker/cron wiring, database outboxes, concrete broker
adapters, retry policy, and realtime fanout adapters remain planned.

See [Contracts](docs/reference/contracts.md) for the runtime API and current
compiler integration details.

## Execution Model

- Routes are declared in `.gwdk` files, not inferred from folders.
- `.gwdk` files are peers of Go files and declare `package <name>`.
- `paths {}` runs at build time and declares dynamic SPA routes.
- `build {}` runs at build time.
- `load {}` runs at request time and requires request-time rendering.
- `act Name POST "/path"` declares POST/action endpoints.
- `api Name METHOD "/path"` declares API endpoints.
- `view {}` renders markup.
- User behavior, storage, auth, validation, and services stay in normal Go.
- Generated Go is adapter glue, not generated application logic.
- Core stays `net/http` compatible; Gin, Echo, Fiber, and similar frameworks
  are optional adapters.

## Getting Started

GOWDK is currently used from source. No release binary yet.

Requirements: Go 1.26+ and git.

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
```

Scaffold and run a new app:

```sh
./gowdk init --tests --template site ~/my-app
cd ~/my-app
/path/to/GoWDK/gowdk build
GOWDK_BIN=/path/to/GoWDK/gowdk go test ./tests
/path/to/GoWDK/gowdk serve --dir dist/site
```

Open `http://127.0.0.1:8080`.

`serve` is static only. It does not run actions, APIs, fragments, commands,
queries, or SSR routes. For backend handlers, use `gowdk build --app --bin` to
produce a runnable binary.

For a live rebuild loop:

```sh
/path/to/GoWDK/gowdk dev
```

Add `--app <dir>` to recompile and restart the generated backend binary on
changes.

Project compiler commands require `gowdk.config.go` in the current directory or
`--config <file>`, even when explicit `.gwdk` files are passed.

See [Getting started](docs/getting-started.md) for the full walkthrough.

## Status Snapshot

Implemented or usable today:

- Page/component compilation, build-time SPA output, layouts, CSS assets,
  generated JS islands, component-level WASM islands, manifests, route reports, LSP,
  generated app source, binaries, and selected-module packaging.
- Action/API handlers, partial fragments, form decoding, CSRF-wired actions,
  guards, endpoint error pages, no-store request-time responses, and concrete
  or dynamic SSR pages with declared `load {}` fields.
- Typed runtime contracts for queries, commands, backend-owned events,
  presentation events, jobs, role filtering, graph/trace/list CLI, event
  capture/replay, and file outbox/event-source support.

Partial or planned: richer fragment data, broader browser reactivity, split
worker/cron contract wiring, database outboxes, broker/realtime adapters,
hybrid behavior, and production operations docs.

## Example

```gwdk
package blog

@page blog.post
@route "/blog/{slug}"
@render spa

paths {
  => { slug: "hello-gowdk" }
  => { slug: "compile-first" }
}

build {
  => {
    title: "GOWDK ships apps",
    description: "Portable pages, build-time data, actions, and fragments."
  }
}

act Refresh POST "/blog/{slug}"

view {
  <main>
    <h1>{title}</h1>
    <p data-slug="{slug}">{description}</p>

    <form g:post={Refresh} g:target="#article-list" g:swap="innerHTML">
      <input name="query" placeholder="Filter articles" />
      <button>Refresh</button>
    </form>

    <section id="article-list">
      <article>
        <h2>{slug}</h2>
        <p>Generated as SPA HTML at build time.</p>
      </article>
    </section>
  </main>
}
```

`Refresh` is a normal exported Go function in package `blog`. Generated code
wires the form POST and writes the returned `runtime/response.Response`.

## CLI

```sh
gowdk version
gowdk init [--force] [--tests] [--template site|minimal] [dir]
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk dev [--addr 127.0.0.1:8080] [--interval 1s] [build flags...]
gowdk preview [--addr 127.0.0.1:8080] [--hot] [build flags...]
gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
gowdk contracts [--json] [dir]
gowdk graph [--json] [dir]
gowdk trace <contract> [--json] [dir]
gowdk list commands|queries|events|jobs [--json] [dir]
gowdk fmt [--write] <files>
gowdk tokens <file.gwdk>
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk lsp [--ssr]
```

See [CLI reference](docs/reference/cli.md) for flags and examples.

## Boundaries

GOWDK does not include an ORM, data layer, login system, admin app, billing
stack, upload service, email service, or background job platform. Those are
application code.

The preferred data-access direction is explicit SQL with tools such as `sqlc`,
`database/sql`, `pgx`, or `sqlx`. ORM-first behavior is not the recommended
path for GOWDK apps.

## Docs

- [Getting started](docs/getting-started.md)
- [Product vision](docs/product/vision.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
- [Architecture](docs/engineering/architecture.md)
- [Contracts](docs/reference/contracts.md)
- [CLI reference](docs/reference/cli.md)
- [Dev loop reference](docs/reference/dev.md)
- [Config reference](docs/reference/config.md)
- [Routing reference](docs/reference/routing.md)
- [Hooks reference](docs/reference/hooks.md)
- [Errors reference](docs/reference/errors.md)
- [Deployment reference](docs/reference/deployment.md)
- [Language notes](docs/language/README.md)
- [Browser compiler](docs/compiler/browser-compiler.md)
- [Examples](examples/README.md)
- [VS Code extension](https://marketplace.visualstudio.com/items?itemName=GoWDK.gowdk-vscode)
