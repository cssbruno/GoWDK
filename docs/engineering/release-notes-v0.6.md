# Experimental 0.x release: GOWDK v0.6

GOWDK v0.6 continues the 0.x line.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before 1.0.

## Breaking

- **The lane model.** GOWDK's three execution lanes — build-time, request-time
  on the server, and the browser — are now named consistently, and `g:for`/`g:if`
  infer their lane from the data they touch.
  - `load {}` → `server {}` and `go ssr {}` → `go server {}` (the request-time
    server lane). `go build {}` is the explicit form of the default `go {}`.
  - A `server {}` block implies request-time rendering (no separate render mode).
  - `g:each` → `g:for` and `g:when` → `g:if`. Over a `server {}` field they render
    server-side; over client `state`/`store` they bind a reactive island. A
    top-level server `g:if` accepts a full bool expression
    (`g:if={count > 0 && status == "open"}`) evaluated at request time.
  - The removed keywords (`load`, `go ssr`, `g:each`, `g:when`) parse to a precise
    migration nudge — no silent aliases.
  - Internal SSR naming (the `addons/ssr` package, the `"ssr"` addon, the
    `gowdk.SSR` render mode, `ssr.LoadContext`, and the `Load<PageID>` handler
    convention) is unchanged. SSR remains the rendering technique that powers the
    server lane.

  Migration: rename `load {}`→`server {}`, `go ssr {}`→`go server {}`,
  `g:each={x in xs}`→`g:for={x in xs}`, and `g:when={f}`→`g:if={f}`. See
  `CHANGELOG.md` and `docs/language/ssr.md` for the full contract.

- **`g:html` → `g:unsafe-html`.** The raw-HTML escape hatch is renamed so its XSS
  surface is explicit at every call site; the old name parses to a nudge.

## Changed

- `gowdk version` and the VS Code extension metadata report `0.6.1`.
- Generated apps import request-time helpers from `runtime/actions`,
  `runtime/api`, `runtime/partial`, `runtime/ratelimit`, `runtime/realtime`, and
  `runtime/ssr` instead of the corresponding `addons/*` packages. The addon
  packages remain the config-facing packages and re-export their runtime helpers
  for 0.x compatibility.

## Implemented

- OS-level playground sandbox for `gowdk playground run --allow-hosted-execution`
  (namespaced, `pivot_root`ed, no network, fails closed when unavailable).
- Addon lifecycle contract documentation and a computed addon version handshake.
- A real-world Go interop example delegating behavior to the standard library.

## Partial / Planned

- Compound server `g:if` is supported at the top level; inside a server row a
  conditional is a single field. Function calls are not evaluated server-side
  (compute in Go and expose a field).
- The 0.x hardening checklist in `docs/engineering/release-plan.md` still governs
  what is implemented, partial, and planned. No minor version is a
  production-readiness target.

## Intentionally out of scope

- No production-readiness claim. The compiler, generated runtime, and docs
  continue through the 0.x line.
