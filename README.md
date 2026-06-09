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

**Website warning:** the public project page is not updated yet and is
currently only a placeholder. Treat this README and the docs in this repository
as the current source of truth.

Project shape:

- `.gwdk` files declare pages, components, layouts, routes, views, build data,
  browser behavior hooks, and endpoint metadata.
- Normal Go packages own handlers, domain logic, auth, persistence, contracts,
  jobs, integration events, and production validation.
- The compiler parses `.gwdk`, validates contracts, lowers to inspectable IR,
  and emits HTML, assets, manifests, build reports, and generated Go adapters.
- Generated Go is runtime wiring: `net/http` routes, form decoders, response
  envelopes, CSRF checks, fragments, SSR/load calls, guards, rate limits, and
  contract web adapters.
- Build-time SPA/static output is the default page lane. `load {}` and
  `go ssr {}` opt pages into request-time rendering. Actions, APIs, and
  fragments are request-time endpoints without forcing full-page SSR.
- Generated browser code is compiler-owned enhancement for SPA navigation,
  partial form posts, fragments, local islands, and WASM mounts. User JS,
  TypeScript, npm assets, and framework interop stay explicit and page-scoped.
- Runtime core stays dependency-light. Framework adapters, Tailwind, Redis,
  NATS, WebSocket fanout, and similar integrations live in optional addons or
  nested modules.
- Unsupported source should fail with diagnostics before generated output gets
  clever or surprising.

Install:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
```

Pin the current CLI release:

```sh
GOWDK_VERSION=v0.2.7 GOWDK_INSTALL_DIR="$HOME/.local/bin" \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh)"
```

Build from source:

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
./gowdk version
```

## What Works Today

This table describes the current demoable 0.x slice. "Partial" means the listed
path works, but the public contract is not stable and the remaining limit is
still tracked in the hardening backlog.

| Surface | Status | Works Today | Current Limit | Docs | Example |
| --- | --- | --- | --- | --- | --- |
| Static build output | Implemented | `gowdk build --out` emits HTML, route metadata, and asset metadata for simple build-time pages. | Generated output is still pre-1.0. | [CLI](docs/reference/cli.md) | [Pages](examples/pages/home.page.gwdk) |
| Dynamic SPA paths | Partial | Dynamic SPA routes can be expanded from the first supported literal `paths {}` subset. | Dynamic SPA routes need `paths {}` unless the page uses request-time rendering. | [Routing](docs/reference/routing.md) | [Blog](examples/pages/blog-post.page.gwdk) |
| Build-time Go data | Partial | Literal build records and supported no-argument Go build functions can feed SPA rendering. | Arbitrary build-time Go statements and broader data lifecycles are not stable. | [Data](docs/language/data.md) | [Go interop](examples/go-interop/README.md) |
| Actions | Partial | Generated apps can serve typed POST action handlers, decode supported form inputs, validate request shape, return redirects/fragments, and opt into CSRF. | File uploads, multipart generated forms, and domain validation stay in user-owned Go handlers. | [Actions](docs/language/actions.md) | [Login](examples/login/README.md) |
| APIs | Partial | Feature-bound API handlers can be generated for supported signatures. | Typed body/query helpers and broader error/status contracts remain planned. | [API](docs/language/api.md) | [API](examples/api/status.page.gwdk) |
| Fragments | Partial | Partial form submissions and standalone fragment routes can return server fragments and remount local islands. | Richer fragment rendering and broader local client behavior are still hardening work. | [Partials](docs/language/partials.md) | [Fragments](examples/partials/patients-fragment.page.gwdk) |
| SSR | Partial | Pages with `load {}` or `go ssr {}` can build request-time handlers when the SSR addon is enabled. | Typed route-param accessors, lifecycle docs, and error/cache contracts need more hardening. | [SSR](docs/language/ssr.md) | [SSR](examples/ssr/simple-ssr.page.gwdk) |
| Hybrid | Partial | Hybrid request-time route metadata and generated request-time pages exist for the supported slice. | The public hybrid source contract, streaming, and data refresh policy are not stable. | [Hybrid](docs/language/hybrid.md) | [Hybrid](examples/ssr/hybrid-static.page.gwdk) |
| Components | Partial | Components support imported contracts, slots, scoped CSS/assets, first local client behavior, and generated island assets. | Non-string props, richer slots/events, real `g:if`/`g:for`, lifecycle cleanup, and dependency diagnostics are planned. | [Components](docs/language/components.md) | [Components](examples/components/base/base-components.page.gwdk) |
| WASM islands | Partial | Component-level `@wasm` and page-level `go client {}` can emit Go `js/wasm` browser assets for supported fixtures. | ABI docs, size reporting, runtime validation, and browser behavior coverage need hardening. | [Components](docs/language/components.md) | [Test fixture](testfixture/islands/islands.go) |
| CSS/assets | Partial | CSS processors, page CSS, scoped component CSS, component assets, asset manifests, content-hashed filenames, and optional Tailwind wrapper exist. | CSS processor contracts and optional dependency boundaries need hardening. | [CSS](docs/reference/css.md) | [CSS](examples/css/styled.page.gwdk) |
| One-binary output | Partial | `gowdk build --app --bin` can generate and compile an embedded Go server for supported SPA/backend/SSR slices. | Runtime operations, split/backend-only deploys, and artifact smoke coverage are still expanding. | [Deployment](docs/reference/deployment.md) | [Embed](examples/embed/site.page.gwdk) |
| Contracts | Partial | Runtime contracts support typed queries, commands, events, jobs, role filtering, local dispatch, file outbox, broker/fanout adapters, contract graph/trace/list commands, and generated `g:command`/`g:query` web adapters. | Split worker/cron generation, retry policy, managed deployment recipes, and editor-first contract visualization remain planned. | [Contracts](docs/reference/contracts.md) | [Runtime contracts](runtime/contracts) |
| Dev server | Partial | `gowdk dev` polls inputs, skips no-op rebuilds, serves or runs generated output, live-reloads browsers, shows a browser overlay for rebuild failures, and keeps serving the last successful output. | Overlay diagnostics need codes, source spans, changed-file context, and better generated-app runtime attribution. Component HMR is intentionally deferred. | [Dev](docs/reference/dev.md) | [Getting started](docs/getting-started.md) |
| Editor/LSP | Implemented | The VS Code extension and dependency-free LSP provide diagnostics, formatting, completions, hover, outline, semantic tokens, definitions, references, site-map visualization, and project-aware navigation for supported paths. | Exact source ranges, richer quick fixes, route/endpoint/contract maps, and `g:command`/`g:query` status in the editor are planned. | [Language server](docs/product/language-server.md) | [VS Code](editors/vscode) |

Security note: request-time features are still 0.x, but the core request-time
hardening is now in place. Generated handlers apply body-size limits (actions
and APIs), server read/header/write/idle timeouts, and a per-request handler
deadline; recovered panics return a generic no-store 500 and are logged
server-side with secret redaction; CSRF is opt-in with a 403 invalid-token
contract; redirects are validated against a safe allowlist (local paths only,
no protocol-relative or CRLF, same-origin referer); and diagnostics redact
secrets quoted from source. Still hardening: full auth/session, multipart
uploads, and the broader operations policy.

Known gaps and release hardening work live in
[the 0.x improvement checklist](docs/engineering/release-plan.md), with public
tracking in the
[0.x Hardening project](https://github.com/users/cssbruno/projects/2).

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

@route "/"
@guard public

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

## Development

Run the full Go test gate, including nested optional adapter modules:

```sh
scripts/test-go-modules.sh
```

Run the root module only:

```sh
go test ./...
go build ./cmd/gowdk
```

Run the all-module vulnerability gate before release-style dependency changes:

```sh
scripts/vulncheck-go-modules.sh
```

## Docs

- [Getting started](docs/getting-started.md)
- [Changelog](CHANGELOG.md)
- [0.x improvement checklist](docs/engineering/release-plan.md)
- [v0.2 release checklist](docs/engineering/v0.2-release-checklist.md)
- [Public 0.x hardening backlog](https://github.com/cssbruno/GoWDK/issues)
- [Examples](examples/README.md)
