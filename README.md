<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

GOWDK ships Go web apps through GOWDK Compiler plus GOWDK Runtime.

- Build-time pages by default.
- Backend actions, APIs, fragments, commands, and queries at request time.
- SSR only when a page opts in with `@render ssr`.
- Direct `style {}` blocks emit generated CSS assets.
- Generated Go stays adapter glue; app logic stays in normal Go packages.
- One-binary deploys are supported.

**Status:** pre-release, not production ready.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)

## Event-Driven

GOWDK includes a typed contract runtime:

```text
UI event -> command/query -> backend handler -> backend event
UI <- result or presentation event
```

- `g:command` binds forms to backend commands.
- `g:query` binds elements to readonly backend queries.
- Commands have one owner.
- Domain and integration events are emitted after command success.
- Presentation events notify UI but are not trusted input.
- Jobs, event capture/replay, role filtering, graph/trace CLI, and file outbox
  support exist today.

See [Contracts](docs/reference/contracts.md).

## Install From Source

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
```

## Start An App

```sh
./gowdk init --tests --template site ~/my-app
cd ~/my-app
/path/to/GoWDK/gowdk build
/path/to/GoWDK/gowdk serve --dir dist/site
```

Open `http://127.0.0.1:8080`.

`serve` is static only. Use `gowdk build --app --bin` for backend handlers,
commands, queries, fragments, and SSR routes.

## Core Syntax

```gwdk
@page blog.post
@route "/blog/{slug}"
@render spa

paths { => { slug: "hello-gowdk" } }
build { => { title: "GOWDK ships apps" } }

style {
  h1 { color: #0f766e; }
}

act Refresh POST "/blog/{slug}"

view {
  <main>
    <h1>{title}</h1>
    <form g:post={Refresh} g:target="#article-list" g:swap="innerHTML">
      <button>Refresh</button>
    </form>
  </main>
}
```

## Commands

```sh
gowdk init [dir]
gowdk check
gowdk build [--app <dir>] [--bin <file>]
gowdk dev
gowdk serve --dir <dir>
gowdk contracts | graph | trace | list
gowdk fmt | manifest | sitemap | routes | lsp
```

See [CLI reference](docs/reference/cli.md).

## Docs

- [Getting started](docs/getting-started.md)
- [Contracts](docs/reference/contracts.md)
- [Language](docs/language/README.md)
- [Architecture](docs/engineering/architecture.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
