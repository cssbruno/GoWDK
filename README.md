<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26.4-00ADD8)

GOWDK ships Go web apps as generated Go servers. Write portable `.gwdk`
pages, compile frontend output, and package it into one binary.

**Status:** experimental 0.x pre-release. Public contracts can still change.
Not production-ready.

Project laws:

- `.gwdk` declares web structure.
- Normal Go owns app behavior.
- Generated Go is adapter glue, not application logic.
- Build-time/static pages are the default.
- Request-time behavior, SSR, and hybrid pages are explicit.
- Generated JavaScript is enhancement only; it does not own auth, routing truth,
  validation truth, business logic, server state, or cache policy.
- `net/http` is the runtime boundary; Gin, Echo, Fiber, Redis, NATS, WebSocket,
  Tailwind, and npm stay optional.
- Unsupported behavior should produce clear diagnostics.

Install:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
```

Build from source:

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
./gowdk version
```

## What Works Today

This matrix is a compact status view for the current 0.x line. "Demo" means
the slice is stable enough to try in examples, not production-ready.

| Surface | Status | Demo | Not production security | Docs | Example | Tests |
| --- | --- | --- | --- | --- | --- | --- |
| Static build output | Implemented | Yes | Yes | [CLI](docs/reference/cli.md) | [Pages](examples/pages/home.page.gwdk) | Yes |
| Dynamic SPA paths | Partial | Yes | Yes | [Routing](docs/reference/routing.md) | [Blog](examples/pages/blog-post.page.gwdk) | Yes |
| Build-time Go data | Partial | Yes | Yes | [Data](docs/language/data.md) | [Go interop](examples/go-interop/README.md) | Yes |
| Actions | Partial | Yes | Yes | [Actions](docs/language/actions.md) | [Login](examples/login/README.md) | Yes |
| APIs | Partial | Yes | Yes | [API](docs/language/api.md) | [API](examples/api/status.page.gwdk) | Yes |
| Fragments | Partial | Yes | Yes | [Partials](docs/language/partials.md) | [Fragments](examples/partials/patients-fragment.page.gwdk) | Yes |
| SSR | Partial | Yes | Yes | [SSR](docs/language/ssr.md) | [SSR](examples/ssr/simple-ssr.page.gwdk) | Yes |
| Hybrid | Partial | Yes | Yes | [Hybrid](docs/language/hybrid.md) | [Hybrid](examples/ssr/hybrid-static.page.gwdk) | Yes |
| Components | Partial | Yes | Yes | [Components](docs/language/components.md) | [Components](examples/components/base/base-components.page.gwdk) | Yes |
| WASM islands | Partial | Yes | Yes | [Components](docs/language/components.md) | [Test fixture](testfixture/islands/islands.go) | Yes |
| CSS/assets | Partial | Yes | Yes | [CSS](docs/reference/css.md) | [CSS](examples/css/styled.page.gwdk) | Yes |
| One-binary output | Partial | Yes | Yes | [Deployment](docs/reference/deployment.md) | [Embed](examples/embed/site.page.gwdk) | Yes |
| Contracts | Partial | Yes | Yes | [Contracts](docs/reference/contracts.md) | [Runtime contracts](runtime/contracts) | Yes |
| Dev server | Partial | Yes | Yes | [Dev](docs/reference/dev.md) | [Getting started](docs/getting-started.md) | Yes |
| LSP | Implemented | Yes | Yes | [Language server](docs/product/language-server.md) | [VS Code](editors/vscode) | Yes |

Known gaps and release hardening work live in
[the 0.x improvement checklist](docs/engineering/release-plan.md).

## Single-Page Server

Create a one-page app, build its generated server, and run the binary:

```sh
gowdk init --tests --template site ./hello-gowdk
cd ./hello-gowdk
gowdk build
./bin/site
```

Open `http://127.0.0.1:8080/`. A minimal page:

```gwdk
package pages

use widgets "components"

@page home
@route "/"

build {
  => { title: "GOWDK ships apps" }
}

view {
  <main>
    <h1>{title}</h1>
    <p>A .gwdk page compiled into an embedded Go server.</p>
    <widgets.Counter />
  </main>
}
```

Add local reactivity:

```go
// ui/counter.go
package ui

type CounterState struct {
	Count int `json:"count"`
}

func NewCounterState() CounterState {
	return CounterState{Count: 0}
}
```

```gwdk
// components/counter.cmp.gwdk
package components

import ui "github.com/acme/hello-gowdk/ui"

@component Counter

state ui.CounterState = ui.NewCounterState()

client {
  computed Label string {
    if Count == 0 {
      return "Start"
    }
    return string(Count)
  }

  func Increment() {
    Count++
  }

  func Reset() {
    Count = 0
  }
}

view {
  <section>
    <p class:active={Count > 0}>{Label}</p>
    <button g:on:click={Increment()}>Add</button>
    <button g:if={Count > 0} g:on:click={Reset()}>Reset</button>
  </section>
}
```

Replace `github.com/acme/hello-gowdk/ui` with your app module path.

## Docs

- [Getting started](docs/getting-started.md)
- [0.x improvement checklist](docs/engineering/release-plan.md)
- [v0.2 release checklist](docs/engineering/v0.2-release-checklist.md)
- [Examples](examples/README.md)
