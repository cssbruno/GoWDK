<p align="center">
  <img src="wdk_logo.png" alt="GoWDK logo" width="220">
</p>

# GoWDK

[![CI](https://github.com/cssbruno/GoWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GoWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GoWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GoWDK/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cssbruno/gowdk)](https://goreportcard.com/report/github.com/cssbruno/gowdk)
[![Go Reference](https://pkg.go.dev/badge/github.com/cssbruno/gowdk.svg)](https://pkg.go.dev/github.com/cssbruno/gowdk)

GoWDK is a Go-first full web app platform: write pages and components in
`.gwdk` files that live next to your Go packages, and ship the whole app —
static pages, backend endpoints, SSR, typed contracts — as one Go binary.

No JavaScript application stack required. No reflection or template engine at
request time. Your logic stays in Go.

<!-- TODO: short GIF of `gowdk dev` rebuilding + live-reloading goes here. -->

**Status:** experimental 0.x pre-release. Public contracts can still change.
Not production-ready.

**Website warning:** the public project page is not updated yet and is
currently only a placeholder. Treat this README and the docs in this repository
as the current source of truth.

## Install

```sh
go install github.com/cssbruno/gowdk/cmd/gowdk@latest
```

Or use the install script:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
```

Pin the current CLI release:

```sh
GOWDK_VERSION=v0.3.0 GOWDK_INSTALL_DIR="$HOME/.local/bin" \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh)"
```

Build from source:

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
./gowdk version
```

## Quickstart: Single-Page Server

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

route "/"
guard public

build {
  => { title: "GoWDK ships apps" }
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

component Counter

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

## CLI at a Glance

"Inspectable" is not just a slogan — the CLI exposes every stage of the
pipeline. Run `gowdk` with no arguments for full flags.

### Build and run

| Command | What it does |
| --- | --- |
| `gowdk init` | Scaffold a starter project (`--template site\|minimal`, `--tests`) |
| `gowdk build` | Compile `.gwdk` into static output, a generated Go app, or a single binary (`--out`, `--app`, `--bin`, `--wasm`, backend variants) |
| `gowdk dev` | Build, serve, rebuild on change, live-reload browsers, show an error overlay |
| `gowdk preview` | Build and serve a local deploy preview |
| `gowdk serve` | Serve already-generated build output |

### Inspect and debug

| Command | What it does |
| --- | --- |
| `gowdk check` | Parse and validate, with `--json` and `--warnings-as-errors` |
| `gowdk fix` | Apply registered safe fixes for diagnostics (`--dry-run`, `--code`) |
| `gowdk explain <code>` | Explain a diagnostic code and its next steps |
| `gowdk doctor` | Check local environment and project health |
| `gowdk inspect ir` / `tree` / `endpoint-graph` / `go-bindings` | Print validated compiler IR, source-linked node tree, endpoint dispatch graph, or Go binding report JSON |
| `gowdk manifest` / `routes` / `sitemap` | Print validated manifest, route/endpoint metadata, or editor site-map JSON |
| `gowdk tokens` | Print raw language tokens for a file |
| `gowdk fmt` | Format `.gwdk` sources (`--write`) |

### Contracts and tooling

| Command | What it does |
| --- | --- |
| `gowdk generate stubs` | Write conservative missing action/API Go handler stubs next to their owning source package |
| `gowdk contracts` / `graph` / `trace` / `list` | Print contract registration metadata, the command/event graph, a single contract trace, or filtered lists of commands/queries/events/jobs |
| `gowdk add <addon>` | Wire an optional addon into `gowdk.config.go` (`add --list` to see all) |
| `gowdk lsp` | Start the language server over stdio |

## Design

`.gwdk` sources move through an explicit pipeline: parse → AST → validation →
a typed intermediate representation → code generation. Every stage is
observable from the CLI (`tokens`, `check`, `inspect ir`, `inspect tree`,
`inspect endpoint-graph`, `manifest`), and the
generators emit three kinds of output from the same IR: static HTML/CSS/asset
bundles, generated Go adapters (`net/http` routes, form decoders, guards), and
browser islands (JS and Go `js/wasm`, with source maps). Diagnostics come from
a central registry with stable codes — `gowdk explain <code>` documents each
one, and error output redacts secrets quoted from source.

How responsibility is split, and the opinions behind it:

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

## What Works Today

This table describes the current demoable 0.x slice. Status levels:

- **Works** — the listed path works end-to-end today.
- **Works, contract unstable** — the listed path works end-to-end, but the
  public contract is still pre-1.0 and the remaining limits are tracked in the
  hardening backlog.
- **Early** — a real but narrower slice works; expect missing cases.

| Surface | Status | Works Today | Current Limit | Docs | Example |
| --- | --- | --- | --- | --- | --- |
| Static build output | Works | `gowdk build --out` emits HTML, route metadata, asset metadata, build reports, and OpenAPI/AsyncAPI inspection reports for simple build-time pages. | Generated output is still pre-1.0. | [CLI](docs/reference/cli.md) | [Pages](examples/pages/home.page.gwdk) |
| Dynamic SPA paths | Early | Dynamic SPA routes can be expanded from the first supported literal `paths {}` subset. | Dynamic SPA routes need `paths {}` unless the page uses request-time rendering. | [Routing](docs/reference/routing.md) | [Blog](examples/pages/blog-post.page.gwdk) |
| Build-time Go data | Early | Literal build records and supported no-argument Go build functions can feed SPA rendering. | Arbitrary build-time Go statements and broader data lifecycles are not stable. | [Data](docs/language/data.md) | [Go interop](examples/go-interop/README.md) |
| Actions | Works, contract unstable | Generated apps can serve typed POST action handlers, decode supported form inputs, validate request shape, return redirects/fragments, and opt into CSRF. | File uploads, multipart generated forms, and domain validation stay in user-owned Go handlers. | [Actions](docs/language/actions.md) | [Login](examples/login/README.md) |
| APIs | Works, contract unstable | Feature-bound API handlers can be generated for supported signatures. | Typed body/query helpers and broader error/status contracts remain planned. | [API](docs/language/api.md) | [API](examples/api/status.page.gwdk) |
| Fragments | Works, contract unstable | Partial form submissions and standalone fragment routes can return server fragments and remount local islands. | Richer fragment rendering and broader local client behavior are still hardening work. | [Partials](docs/language/partials.md) | [Fragments](examples/partials/patients-fragment.page.gwdk) |
| SSR | Works, contract unstable | Pages with `load {}` or `go ssr {}` can build request-time handlers when the SSR addon is enabled. | Typed route-param accessors, lifecycle docs, and error/cache contracts need more hardening. | [SSR](docs/language/ssr.md) | [SSR](examples/ssr/simple-ssr.page.gwdk) |
| Hybrid | Early | Hybrid request-time route metadata and generated request-time pages exist for the supported slice. | The public hybrid source contract, streaming, and data refresh policy are not stable. | [Hybrid](docs/language/hybrid.md) | [Hybrid](examples/ssr/hybrid-static.page.gwdk) |
| Components | Works, contract unstable | Components support imported contracts, slots, scoped CSS/assets, first local client behavior, and generated island assets. | Non-string props, richer slots/events, real `g:if`/`g:for`, lifecycle cleanup, and dependency diagnostics are planned. | [Components](docs/language/components.md) | [Components](examples/components/base/base-components.page.gwdk) |
| WASM islands | Early | Component-level `wasm` and page-level `go client {}` can emit Go `js/wasm` browser assets for supported fixtures. | ABI docs, size reporting, runtime validation, and browser behavior coverage need hardening. | [Components](docs/language/components.md) | [Test fixture](testfixture/islands/islands.go) |
| CSS/assets | Works, contract unstable | CSS processors, page CSS, scoped component CSS, component assets, asset manifests, content-hashed filenames, and optional Tailwind wrapper exist. | CSS processor contracts and optional dependency boundaries need hardening. | [CSS](docs/reference/css.md) | [CSS](examples/css/styled.page.gwdk) |
| One-binary output | Works, contract unstable | `gowdk build --app --bin` can generate and compile an embedded Go server for supported SPA/backend/SSR slices. | Runtime operations, split/backend-only deploys, and artifact smoke coverage are still expanding. | [Deployment](docs/reference/deployment.md) | [Embed](examples/embed/site.page.gwdk) |
| Contracts | Works, contract unstable | Runtime contracts support typed queries, commands, events, jobs, role filtering, local dispatch, file outbox, broker/fanout adapters, contract graph/trace/list commands, and generated `g:command`/`g:query` web adapters. | Split worker/cron generation, retry policy, managed deployment recipes, and editor-first contract visualization remain planned. | [Contracts](docs/reference/contracts.md) | [Runtime contracts](runtime/contracts) |
| Dev server | Works | `gowdk dev` polls inputs, skips no-op rebuilds, serves or runs generated output, live-reloads browsers, shows a browser overlay for rebuild failures, and keeps serving the last successful output. | Overlay diagnostics need codes, source spans, changed-file context, and better generated-app runtime attribution. Component HMR is intentionally deferred. | [Dev](docs/reference/dev.md) | [Getting started](docs/getting-started.md) |
| Editor/LSP | Works | The VS Code extension and dependency-free LSP provide diagnostics, formatting, completions, hover, outline, semantic tokens, definitions, references, site-map visualization, and project-aware navigation for supported paths. | Exact source ranges, richer quick fixes, route/endpoint/contract maps, and `g:command`/`g:query` status in the editor are planned. | [Language server](docs/product/language-server.md) | [VS Code](editors/vscode) |

Security note: request-time features are still 0.x, but the core request-time
hardening is now in place. Generated handlers apply body-size limits (actions
and APIs), server read/header/write/idle timeouts, and a per-request handler
deadline; recovered panics return a generic no-store 500 and are logged
server-side with secret redaction; ordinary returned 5xx errors use generic
client messages unless the app sets an explicit `response.HandlerError`
message; CSRF is opt-in with a 403 invalid-token contract; redirects are
validated against a safe allowlist (local paths only, no protocol-relative or
CRLF, same-origin referer); and diagnostics redact secrets quoted from source.
Still hardening: full auth/session, multipart uploads, and the broader
operations policy.

Known gaps and release hardening work live in
[the 0.x improvement checklist](docs/engineering/release-plan.md), with public
tracking in the
[0.x Hardening project](https://github.com/users/cssbruno/projects/2).

## Addons

The runtime core stays dependency-light; everything optional is an addon you
wire in with `gowdk add <addon>` (see `gowdk add --list`):

| Addon | What it adds |
| --- | --- |
| `actions` | Backend form/action handlers |
| `api` | Request-time API endpoints |
| `auth` | Batteries-included auth: PBKDF2, signed sessions, RBAC guards (no external deps) |
| `contracts` | Contract-driven command/event metadata |
| `css` | Build-time CSS processing |
| `db` | sqlc + `database/sql` plumbing helper (no domain, no driver dep) |
| `embed` | Embed build output into the binary |
| `partial` | Fragment/partial responses |
| `ratelimit` | Request-time rate limiting |
| `ssr` | Server-side rendering |

Heavier integrations (Tailwind, Redis, NATS, WebSocket fanout) live in nested
optional modules so the root module's dependency graph stays small.

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

Generated output is golden-tested: the compiler's HTML, generated Go adapters,
island JS/WASM assets, IR, and CLI behavior are all snapshot-checked against
committed fixtures, so any change to generated artifacts shows up as a
reviewable test diff rather than a silent behavior change.

## Docs

- [Getting started](docs/getting-started.md)
- [Changelog](CHANGELOG.md)
- [0.x improvement checklist](docs/engineering/release-plan.md)
- [v0.2 release checklist](docs/engineering/v0.2-release-checklist.md)
- [Public 0.x hardening backlog](https://github.com/cssbruno/GoWDK/issues)
- [Examples](examples/README.md)
