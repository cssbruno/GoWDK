# ADR 0002: Compile-First Render Model

Date: 2026-06-04

Status: Accepted

## Context

GOWDK must not be full SSR by default. The product goal is a Go-first portable web compiler that can emit SPA pages, typed backend actions, server fragments, APIs, embedded assets, and one-binary deploys. Request-time full-page rendering is useful, but only for pages that need auth/session/request-aware data.

## Decision

Core GOWDK renders at build time by default. SSR is an optional addon and a per-page render mode.

Render modes:

- `spa`: build-time HTML.
- `action`: SPA page plus backend actions/API behavior.
- `hybrid`: SPA by default with selected request-time behavior.
- `ssr`: request-time full-page rendering through the SSR addon.

Block semantics:

- `paths {}` runs at build time and declares dynamic SPA routes.
- `build {}` runs at build time.
- `load {}` runs at request time and requires request-time rendering.
- `act Name POST "/path"` declares POST/action endpoints.
- `api Name METHOD "/path"` declares API endpoints.
- `view {}` renders markup.

Compiler rules:

- Default render mode is `spa`.
- Dynamic SPA routes require `paths {}`; action endpoints inherit generated
  concrete page paths.
- `@render ssr` requires `ssr.Addon()`.
- `load {}` is rejected on SPA/action pages.
- Actions can exist without SSR.
- Partial updates are server fragments, not full-page SSR.

## Consequences

### Positive

- GOWDK has a sharper identity: app-shipping Go compiler with build-time output, backend actions, and optional SSR.
- One-binary deploy works with or without request-time full-page rendering.
- Actions, APIs, and fragments can provide backend behavior without forcing SSR.
- Compiler diagnostics can catch render model mistakes early.

### Negative

- The compiler must distinguish build-time and request-time blocks clearly.
- Hybrid behavior needs careful design to avoid becoming implicit SSR.
- SPA dynamic routes require a `paths {}` concept before route generation is complete.

### Neutral

- Runtime render core is shared by spa, actions, partials, and SSR.
- `addons/ssr` depends on render core; render core does not depend on SSR.

## Alternatives Considered

- Make SSR the default framework identity. Rejected because it conflicts with portable compile-first output and one-binary app serving.
- Make build output an addon. Rejected because build-time rendering is the
  compiler core product behavior.
- Treat partial updates as SSR. Rejected because server fragments are smaller and do not require full-page request-time rendering.

## Follow-Up

- Implement `.gwdk` discovery and manifest generation.
- Implement parser support for `paths`, `build`, `load`, `act`, `api`, and `view`.
- Implement build-output/prerender codegen.
- Implement CSS/plugin extension points. Tailwind should remain an optional
  addon/plugin, not part of the initial compiler core or runtime core.
- Implement one-binary serving before SSR addon internals.
