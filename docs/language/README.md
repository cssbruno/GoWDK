# GOWDK Language

This directory documents the current `.gwdk` language contract.

The current implementation supports line-oriented page metadata, page-level Go
imports for build-time data, GOWDK `use` declarations for page-level
cross-package component calls, explicit component metadata, top-level block
detection, the first typed action input/redirect/fragment-metadata subset, the
first API method/route metadata subset, minimal literal `view {}` markup parsing,
metadata capture for `go {}` inline Go authoring blocks,
first-slice `g:post`, `g:target`, and `g:swap` lowering, literal dynamic SPA
route expansion, literal build data, imported no-argument Go build data
functions, default `go {}` build-data functions, package
type-checking for saved default `go {}` blocks, generated
`gowdk_go/` packages for default `go {}` and `go ssr {}` blocks,
same-page action/API/fragment handlers from default `go {}` blocks,
page-level `go client {}` WASM mounts, `go ssr {}` load
handlers, configured-addon
`go addon.<name> {}` validation and generated app Go file emission through
`gowdk.GoBlockConsumer`, route/build-data
interpolation in views, Go-typed component props/state contracts, first-slice
generated JavaScript islands for stateful components, component-level `wasm`
island asset emission, formatting, diagnostics, manifest output, build output
for simple SPA pages/components, generated partial fragment responses for
embedded apps, and LSP/editor integration. It does not yet parse non-string
inline props, full typed action semantics, API request/response bodies, broad
local client-side reactivity, or full semantic/type analysis outside the
component contract and inline package-go-block slices.

## Current Files

- `spec.md`: compact current `.gwdk` language contract for M2 compiler work.
- `syntax.md`: lexical tokens and accepted top-level forms.
- `grammar.md`: current parser grammar and future grammar boundaries.
- `semantics.md`: current render-mode and validation rules.
- `guards.md`: `guard` and the default-deny page access contract.
- `blocks.md`: block meanings and current parser support.
- `data.md`: build-time data, request-time load data, endpoint data, and
  invalidation boundaries.
- `markup.md`: current `view {}` status and planned markup behavior.
- `components.md`: component status and portability rules.
- `layouts.md`: layout metadata and planned layout resolution.
- `docs/reference/routing.md`: route validation, route plans, and generated
  route output.
- `actions.md`: action status and planned typed action behavior.
- `api.md`: API block status and planned handler behavior.
- `partials.md`: partial update status and planned fragment behavior.
- `forms.md`: form submission, progressive enhancement, validation, and
  invalidation boundaries.
- `ssr.md`: SSR render-mode, `load`, and guard behavior.
- `hybrid.md`: hybrid request-time behavior and deferred hybrid capabilities.
- `diagnostics.md`: current diagnostic shape and known codes.
- `formatting.md`: current formatter behavior.

## File Kinds

The compiler currently treats every parsed file as a page file. A minimal page
uses:

```gwdk
route "/"
guard public
```

The page ID derives from the filename unless `page` is present.

Component files are supported as explicit or discovered `gowdk build` inputs
with `component`. Layout files are also supported. Separate island and
plugin-adjacent file kinds are planned.
