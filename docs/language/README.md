# GOWDK Language

This directory documents the current `.gwdk` language contract.

The current implementation supports line-oriented page metadata, page-level Go
imports for build-time data, GOWDK `use` declarations for page-level
cross-package component calls, explicit component metadata, top-level block
detection, the first typed action input/redirect/fragment-metadata subset, the
first API method/route metadata subset, minimal literal `view {}` markup parsing,
first-slice `g:post`, `g:target`, and `g:swap` lowering, literal dynamic SPA
route expansion, literal build data, imported no-argument Go build data
functions, route/build-data interpolation in views, Go-typed component
props/state contracts, first-slice generated JavaScript islands for stateful
components, explicit `g:island="wasm"` asset emission, formatting, diagnostics,
manifest output, build output for simple SPA pages/components, generated
partial fragment responses for embedded apps, and LSP/editor integration. It
does not yet parse non-string inline props, full typed action semantics, API
request/response bodies, broad local client-side reactivity, or full
semantic/type analysis outside the component contract slice.

## Current Files

- `syntax.md`: lexical tokens and accepted top-level forms.
- `grammar.md`: current parser grammar and future grammar boundaries.
- `semantics.md`: current render-mode and validation rules.
- `blocks.md`: block meanings and current parser support.
- `markup.md`: current `view {}` status and planned markup behavior.
- `components.md`: component status and portability rules.
- `layouts.md`: layout metadata and planned layout resolution.
- `docs/reference/routing.md`: route validation, route plans, and generated
  route output.
- `actions.md`: action status and planned typed action behavior.
- `api.md`: API block status and planned handler behavior.
- `partials.md`: partial update status and planned fragment behavior.
- `ssr.md`: SSR render-mode, `load`, and guard behavior.
- `diagnostics.md`: current diagnostic shape and known codes.
- `formatting.md`: current formatter behavior.

## File Kinds

The compiler currently treats every parsed file as a page file and requires:

```gwdk
@page home
@route "/"
```

Component files are supported as explicit or discovered `gowdk build` inputs
with `@component`. Layout files are also supported. Separate island and
plugin-adjacent file kinds are planned.
