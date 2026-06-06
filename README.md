<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

![Build](https://img.shields.io/badge/build-succeeds-2ea44f)
![Quality](https://img.shields.io/badge/quality-gated-2ea44f)

A compiler and runtime kit for full-stack Go web applications.

GOWDK processes `.gwdk` files: pages, components, layouts, actions, APIs,
fragments, and assets. It generates Go adapter code for route dispatch, form
decoding, response writing, CSRF, embedded assets, partial updates, and SSR
hooks. Your domain logic, storage, auth, validation, and services stay in
normal Go packages.

**Status: pre-release.** The compiler handles core page and component
compilation, embedded app builds, generated JavaScript islands, explicit WASM
islands, CSRF-wired action handlers, partial fragments, and concrete or dynamic
SSR pages with declared `load {}` fields. The runtime model and language
surface are still evolving. Not ready for production use.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)

## How It Works

Write `.gwdk` files for your web-facing contracts. The compiler emits
`net/http`-compatible Go. There is no reflection and no hidden request-time
magic in the generated adapters.

Full pages default to build-time output. Actions, APIs, and fragments run as
backend endpoints without forcing full-page SSR. Pages that need request data,
guards, sessions, or per-request state opt into request-time rendering with
`@render ssr`.

The generated handlers are plain `http.Handler` values. Echo, Gin, Chi, Fiber,
and similar frameworks are integration targets, not core dependencies.

## Data Access

GOWDK does not include an ORM or a data layer. The project is intentionally
against GORM and ORM-first application design as the recommended path.

The preferred approach is explicit SQL: `sqlc` for type-safe generated queries,
with `database/sql`, `pgx`, `sqlx`, or similar focused libraries for everything
else. Explicit SQL is easier to review, test, and reason about than hidden query
generation, model magic, or framework-owned schema state.

Planned work includes helpers for wiring `sqlc`-generated query packages into
actions and APIs, plus admin/form scaffolding from explicit SQL contracts.

## Application Scaffolding

GOWDK core is the compiler and runtime kit, not an app template. Login, admin,
billing, CRUD, uploads, email, and background jobs are application code.

Examples or optional generators may cover those patterns later, but the output
should be editable Go and `.gwdk` files, not hidden framework behavior.

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
./gowdk init ~/my-app
cd ~/my-app
/path/to/GoWDK/gowdk build
/path/to/GoWDK/gowdk serve --dir dist/site
```

Open `http://127.0.0.1:8080`.

`init` writes a starter `gowdk.config.go`, one page, one component, and one CSS
file. `build` outputs HTML and manifests to `dist/site`. `serve` is static
only: it does not run actions, API handlers, partial fragments, or SSR routes.
For backend handlers, use `gowdk build --app --bin` to produce a runnable
binary.

For a live rebuild loop:

```sh
/path/to/GoWDK/gowdk dev
```

`dev` polls source files, rebuilds on change, and reloads the browser. Add
`--app <dir>` to also recompile and restart the generated backend binary on
changes.

Project compiler commands require `gowdk.config.go` in the current directory,
or `--config <file>`, even when explicit `.gwdk` files are passed. This source
repository includes a root `gowdk.config.go` for example commands.

See [docs/getting-started.md](docs/getting-started.md) for the full
walkthrough.

## What Works Now

- Page and component compilation, config-based discovery, and named build
  targets.
- Literal `paths {}` expansion for dynamic SPA routes.
- Literal `build {}` data and imported no-argument Go build data functions.
- Generated embedded app source, local binaries, and Go `js/wasm` artifacts.
- Action/API handlers, action redirects, partial fragments, CSRF-wired action
  handlers, guards, endpoint-local error pages, and SSR pages with
  declared `load {}` identifier or dotted-path execution, safe load redirects,
  and generated error pages in generated binaries.
- CLI commands: `build`, `dev`, `serve`, `preview`, `check`, `fmt`, `lsp`,
  `manifest`, `sitemap`, and `routes`.

Partial or planned: broader generated validation coverage, richer fragment data
contracts, scoped component CSS/asset emission, and broader local browser
reactivity. File uploads are intentionally left to user-owned API/server
handlers.

## Site Example

```gwdk
package blog

@page blog.post
@route "/blog/{slug}"
@layout root, blog
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
  <main class="page">
    <header class="hero">
      <p>Compile-first Go UI</p>
      <h1>{title}</h1>
      <p data-slug="{slug}">{description}</p>
    </header>

    <form g:post={Refresh} g:target="#article-list" g:swap="innerHTML">
      <input name="query" placeholder="Filter articles" />
      <button>Refresh</button>
    </form>

    <section id="article-list">
      <article>
        <h2>{slug}</h2>
        <p>Generated as spa HTML at build time.</p>
      </article>
    </section>
  </main>
}
```

`Refresh` is a normal exported Go function in package `blog`; generated code
wires the form POST and writes the returned `runtime/response.Response`.

## CLI

```sh
gowdk init [--force] [dir]
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk dev [--addr 127.0.0.1:8080] [--interval 1s] [build flags...]
gowdk preview [--addr 127.0.0.1:8080] [--hot] [build flags...]
gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
gowdk fmt [--write] <file.gwdk>
gowdk tokens <file.gwdk>
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk lsp [--ssr]
```

Every successful disk build writes `gowdk-build-report.json`. Pass `--debug` to
mirror the structured report to stderr.

`gowdk dev --app <dir>` generates the app, compiles a dev binary, and restarts
that generated process after successful rebuilds. `gowdk preview` builds once
into a temporary deploy-preview output and serves it; `gowdk preview --hot`
uses the dev loop for local hot preview.

## Build Targets

`gowdk.config.go` can declare named outputs for separate deployables:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{
			Name:    "site",
			Modules: []string{"public"},
			Output:  "dist/site",
			App:     ".gowdk/site",
			Binary:  "bin/site",
		},
		{
			Name:    "admin",
			Modules: []string{"admin"},
			Output:  "dist/admin",
			App:     ".gowdk/admin",
			Binary:  "bin/admin",
		},
		{
			Name:    "api",
			Modules: []string{"api"},
			Output:  "dist/api",
			App:     ".gowdk/api",
			Binary:  "bin/api",
		},
		{
			Name:    "web",
			Modules: []string{"public", "admin"},
			Output:  "dist/web",
			App:     ".gowdk/web",
			Binary:  "bin/web",
		},
	},
}
```

Run all configured targets:

```sh
gowdk build
```

Run one deployable:

```sh
gowdk build --target admin
```

Run multiple deployables together:

```sh
gowdk build --target site --target api
```

The `site`, `admin`, and `api` targets build separate outputs and binaries. The
`web` target packages two frontend modules together when one binary should serve
both the public site and admin UI.

Modules are local source groups today. A future module system should also allow
importing reusable GOWDK modules from GitHub repositories, Go module paths, or
similar sources, so apps can share pages, components, admin screens, and
integration blueprints without copying files by hand.

## Docs

- [Getting started](docs/getting-started.md)
- [Product vision](docs/product/vision.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
- [Gap checklist](docs/product/gap-checklist.md)
- [Architecture](docs/engineering/architecture.md)
- [Release readiness](docs/engineering/release.md)
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
