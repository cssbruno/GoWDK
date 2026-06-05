# Product Vision

## Product Name

GOWDK

WDK does not have a canonical expansion. No one knows what it stands for; GOWDK
just ships apps.

## One-Line Description

GOWDK is a portable Go web compiler.

## Target Users

- Go developers building product applications who want build-time output, typed backend behavior, and one-binary deployment.
- Small teams that want Go-first UI authoring without a large JavaScript application stack.
- Builders who want request-time SSR only for pages that actually need request context.

## Problem

Modern web frameworks often force teams into a rendering ideology: full SSR, full SPA, or static-only. GOWDK should let Go teams compile portable `.gwdk` files into static pages, components, typed actions, partial updates, and a deployable binary while keeping full-page SSR optional.

## Differentiation

- Files are portable: routes and layouts are declared in files, not implied by folder nesting.
- Static render is the default.
- Actions can exist without SSR.
- Partial updates use server fragments instead of full page request-time rendering.
- SSR is an addon for selected pages, not the framework identity.
- Production can ship as one Go binary with embedded frontend assets.

## Success Metrics

- A v0.1 app can compile movable `.gwdk` files into prerendered HTML, CSS, assets, and one serving binary.
- A v0.2 app can handle typed actions, form decoding, validation, redirects, CSRF, and server fragments without enabling SSR.
- A v0.4 app can opt selected pages into request-time SSR with clear compiler diagnostics and addon checks.
- Developers can explain the mental model in one sentence: GOWDK ships apps through a compile-first Go compiler with build-time output, backend actions, and optional SSR.

## Constraints

- Language: Go-first compiler, runtime, and deployment.
- Styling: CSS tooling should be plugin-driven. Tailwind is an optional addon/plugin, not initial core.
- JavaScript: no user-written JavaScript for normal app flows.
- Rendering: build-time full-page rendering by default.
- Deployment: one-binary production deploy must work with and without SSR.
- Extensibility: SSR, actions, partials, API, embed, CSS plugins, and WASM islands should remain modular capabilities.
