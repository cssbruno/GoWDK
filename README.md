<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

GOWDK is a compiler and runtime for shipping Go web apps. Write `.gwdk` files,
get build-time pages, request-time backend behavior, opt-in SSR, and one-binary
deploys while keeping application logic in Go.

GOWDK Compiler owns `.gwdk` parsing, analysis, route metadata, endpoint
metadata, assets, generated adapter source, diagnostics, and LSP support.
GOWDK Runtime owns serving, request context, actions, APIs, fragments, CSRF,
contracts, SSR hooks, embedded assets, and one-binary wiring.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)

**Status:** pre-release. Public contracts can still change. Not production
ready.

## What It Looks Like

```gwdk
// pages/home.page.gwdk
package pages

import copy "github.com/acme/app/content"

use ui "components"

@page home
@route "/"

build {
  => copy.HomePageForBuild()
}

view {
  <main>
    <h1>{title}</h1>
    <p>{canonicalPath}</p>
    <ui.Counter />
  </main>
}

style {
  h1 { color: #0f766e; }
}
```

```go
// content/home.go
package content

import (
	"strings"
)

type PageCopy struct {
	Title         string `json:"title"`
	Slug          string `json:"slug"`
	CanonicalPath string `json:"canonicalPath"`
}

func HomePageForBuild() PageCopy {
	title := "GOWDK ships apps"
	slug := strings.ToLower(strings.ReplaceAll(title, " ", "-"))

	return PageCopy{
		Title:         title,
		Slug:          slug,
		CanonicalPath: "/posts/" + slug,
	}
}
```

```gwdk
// components/counter.cmp.gwdk
package components

import ui "github.com/acme/app/ui"

@component Counter

state ui.CounterState = ui.NewCounterState()

client {
  computed Label string {
    return if Count == 0 { "Start" } else { string(Count) }
  }

  fn Increment() {
    Count++
  }

  fn Reset() {
    Count = 0
  }
}

view {
  <section .counter>
    <p class:active={Count > 0}>{Label}</p>
    <button g:on:click={Increment()}>Add</button>
    <button g:if={Count > 0} g:on:click={Reset()}>Reset</button>
  </section>
}

style {
  .counter { display: grid; gap: 0.75rem; }
  .active { color: #0f766e; }
}
```

Programming logic stays in Go. Today that means normal `.go` files imported or
referenced from `.gwdk`, plus default and `go spa {}` blocks for colocated
static helpers. Saved default and `go spa {}` blocks are type-checked with
sibling Go files during validation. `build {}` can call an imported or inline
no-argument Go function at build time, JSON-encode its returned object, and
expose scalar fields to `view {}`. Request-time and addon-targeted go blocks are
parsed and validated; generated apps can execute `go ssr {}` load handlers,
page-level `go spa {}` can opt into browser Go by exporting
`//go:wasmexport GOWDKMount<PageID>` for a generated WASM page loader, and
configured addons that implement `gowdk.GoBlockConsumer` can validate
`go addon.<name> {}` blocks and emit generated app Go files. Generated app source
materializes default, `go spa {}`, and `go ssr {}` blocks as normal Go
packages under `gowdk_go/`.

Pages can stay build-time while components own local reactive state; the
compiler generates the island runtime for `client {}` handlers, computed
values, bindings, class toggles, and conditional DOM.

## How It Works

- Build-time by default: full pages compile to SPA output with route and asset
  manifests.
- Request-time only where needed: `@render ssr` opts a page into supported
  `load {}`, guards, route params, redirects, and generated error pages.
- Backend behavior without full SSR: actions, APIs, fragments, commands, and
  queries run per request without making every page request-rendered.
- Direct CSS: `style {}` emits generated CSS assets. Component CSS, scoped CSS,
  processors, and Tailwind CLI integration are available in bounded slices.
- Generated Go is adapter glue: generated code can decode supported typed
  inputs, validate supported form fields, wire CSRF, run guards, and apply
  optional rate limiting.
- One-binary deploys: generated apps can embed frontend output and request-time
  handlers into a single Go binary.

## Event-Driven Contracts

GOWDK can enable a typed contract runtime through the contracts addon:

```text
UI event -> command/query -> backend handler -> backend event
UI <- result or presentation event
```

- `g:command` binds forms to backend commands.
- `g:query` binds elements to readonly backend queries.
- Commands have one owner.
- Domain and integration events are emitted after command success.
- Presentation events notify UI but are not trusted input.
- Jobs, event capture/replay, role filtering, graph/trace CLI, file outbox,
  local in-memory broker, Redis Streams, NATS, SSE, and WebSocket fanout
  support exist today.

See [Contracts](docs/reference/contracts.md).

## Install

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
```

## Quickstart

### Static Site

```sh
./gowdk init --tests --template site ~/my-app
cd ~/my-app
/path/to/GoWDK/gowdk build
/path/to/GoWDK/gowdk serve --dir dist/site
```

Open `http://127.0.0.1:8080`.

`serve` is static only. It is for generated frontend output without backend
handlers.

### Full App

```sh
/path/to/GoWDK/gowdk build --app dist/app --bin dist/my-app
./dist/my-app
```

This builds a Go binary that serves embedded frontend output alongside
supported backend handlers, commands, queries, fragments, and SSR routes.

## CLI Reference

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

## Current Status

| Area | Status |
| --- | --- |
| Pages, routes, layouts, and render modes | Available |
| Build-time SPA output | Available |
| Actions, APIs, and fragments | Available |
| Commands, queries, and contracts addon | Available |
| Direct `style {}` CSS assets | Available |
| One-binary deploys | Available |
| LSP server | Available |
| SSR with `load {}`, guards, params, and errors | Partial |
| Component behavior, client behavior, and scoped CSS | Partial |
| WASM islands | Partial |
| `go {}` metadata for inline Go authoring | Available |
| Build-time default/SPA go block functions for `build {}` | Partial |
| Browser `go spa {}` page mounts through Go WASM | Partial |
| Request-time `go ssr {}` load execution | Partial |
| Addon inline Go adapter file generation | Partial |
| Hybrid rendering beyond explicit request-time branches | Planned |
| Split worker/cron contract wiring | Planned |
| Durable outbox, broker, and realtime adapters | Partial |
| Production operations guidance | Planned |

See [Requirements](docs/product/requirements.md) for the full status table and
[Roadmap](docs/product/roadmap.md) for planned work.

## Docs

- [Getting started](docs/getting-started.md)
- [Language](docs/language/README.md)
- [Contracts](docs/reference/contracts.md)
- [CLI reference](docs/reference/cli.md)
- [Architecture](docs/engineering/architecture.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
