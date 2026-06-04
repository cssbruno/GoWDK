# Product Requirements

## Current Status

The initial product direction is compile-first and static/action-first. Full-page request-time SSR is optional and lives behind the SSR addon.

## Requirements

| ID | Requirement | Priority | Status | Notes |
| --- | --- | --- | --- | --- |
| PRD-001 | Compile portable `.gwdk` files that declare `@page`, `@route`, `@layout`, and optional `@render`. | High | Partial | Discovery, metadata parsing, default build discovery, and explicit component-file build input are implemented; full compile/codegen remains planned. |
| PRD-002 | Default render mode must be `static`. | High | Implemented | Root `RenderConfig.DefaultMode()` defaults to `gowdk.Static`. |
| PRD-003 | Support render modes `static`, `action`, `hybrid`, and `ssr`. | High | Implemented | Root `RenderMode` constants exist. |
| PRD-004 | Reject `@render ssr` unless the SSR addon is enabled. | High | Implemented | `internal/compiler.ValidatePage` emits `missing_ssr_addon`. |
| PRD-005 | Require `paths {}` for dynamic static/action routes. | High | Implemented | Static/action dynamic routes without paths are rejected; the first literal string `paths {}` subset can prerender dynamic static routes. |
| PRD-006 | Keep typed actions available without SSR. | High | Partial | Static pages with `act` blocks validate without SSR; the first action body subset parses `input := form Type`, `valid(input)?`, and local redirects, and generated static apps can serve POST redirect handlers with first-slice input decoders, unexpected-field rejection, and required-field validation. Real user Go type resolution, CSRF, user action logic, and fragment responses remain planned. |
| PRD-007 | Treat `load {}` as request-time behavior requiring SSR or hybrid rendering. | High | Implemented | Static pages with `load` are rejected. |
| PRD-008 | Keep runtime render core separate from optional SSR addon. | High | Implemented | `runtime/render` exists independently from `addons/ssr`. |
| PRD-009 | Generate static/prerender output for v0.1. | High | Partial | `gowdk build --out` emits static HTML, `gowdk-routes.json`, and `gowdk-assets.json` for simple build-time pages, the first literal dynamic path and build-data subsets, and explicit or discovered components; generated handlers, arbitrary build-time execution, and full component semantics remain planned. |
| PRD-010 | Provide CSS/plugin extension points without adding Tailwind to initial core. | High | Partial | `FeatureCSS`, `addons/css`, configured stylesheet links, compile-time CSS processors, CSS asset output, and CSS asset manifest entries are implemented; Tailwind, class extraction, hashing, and full plugin docs remain planned. |
| PRD-011 | Support embedded assets and one-binary serving. | High | Partial | `addons/embed` and `runtime/asset` boundaries exist; `gowdk serve` can serve generated static output locally; `gowdk build --app` can generate an embedded static app and `--bin` can compile it into one binary for static pages and first-slice action redirects. API/fragment/SSR handlers remain planned. |
| PRD-012 | Support server fragments for partial updates without full-page SSR. | Medium | Planned | `addons/partial` and `runtime/response.FragmentFor` exist. |
| PRD-013 | Add SSR addon with request-aware `load {}`, guards, layouts, and error handling. | Medium | Planned | `addons/ssr` boundary exists; guard annotations are parsed as metadata only today. |
| PRD-014 | Add optional WASM islands after the core compiler and action flow are stable. | Low | Future | Roadmap v0.6. |
| PRD-015 | Provide language tools for `.gwdk` token inspection, formatting, validation, manifest output, and LSP editor integration. | High | Implemented | `internal/lang`, `internal/lsp`, and CLI commands exist. |

## Non-Functional Requirements

- Performance: static pages should be generated at build time and served directly from disk or embedded assets.
- Reliability: compiler diagnostics must fail fast for invalid render modes, missing SSR addon, and dynamic static routes without paths.
- Security: actions need CSRF, typed form decoding, validation, and safe redirects before production use.
- Privacy: generated logs and diagnostics must not expose secrets or sensitive form data.
- Accessibility: generated components should preserve semantic HTML and support focus restoration for partial updates.
- Localization: route and content generation should not assume one locale.
- Supportability: manifest output should include route, render mode, layouts, paths presence, and guards for debugging.

## Out Of Scope

- Full SPA runtime as the default experience.
- Mandatory full-page SSR.
- User-written JavaScript for normal forms, actions, and partial update flows.
- WASM islands before the static compiler, typed actions, and partial update flow are stable.

## Open Questions

- What exact `.gwdk` grammar should the first parser support?
- Should `hybrid` be a page render mode, a route policy, or a higher-level application mode?
- How should GOWDK express cache revalidation for static and hybrid routes?
- What plugin interface should CSS integrations use?
