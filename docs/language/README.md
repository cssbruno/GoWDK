# GOWDK Language

This directory documents the current `.gwdk` language contract.

`.gwdk` files declare pages, components, layouts, metadata, routes, build-time
data, request-time endpoints, and bounded client behavior. Normal Go packages
own domain logic, persistence, auth, and production policy.

## Minimal Page

```gwdk
package pages

route "/"
guard public

build {
  => { title: "GOWDK ships apps" }
}

view {
  <main>
    <h1>{title}</h1>
  </main>
}
```

## Implemented

- Line-oriented page metadata, including `route`, `guard`, `title`,
  `description`, `canonical`, `noindex`, and supported `jsonld` declarations.
- Top-level page, component, layout, action, API, fragment, build, paths,
  server, view, client, CSS, import, and use declarations for the supported
  slices.
- Build-time SPA output for simple pages and components.
- Literal dynamic SPA route expansion through `paths {}`.
- Literal build data and imported Go build-data functions, including optional
  `gowdk.BuildParams`.
- Default `go {}` and `go server {}` package emission under generated
  `gowdk_go/` packages.
- Same-page action, API, and fragment handlers from default `go {}` blocks.
- Request-time page loading through `server {}` or `go server {}` when SSR is
  enabled.
- Route/build-data interpolation in views.
- Go-typed component props and state contracts.
- First-slice generated JavaScript islands, page-level `go client {}` WASM
  mounts, and component-level `wasm` island assets.
- Formatting, diagnostics, manifest output, build reports, and LSP/editor
  integration.

## Partial

- Typed actions cover the first input, redirect, and fragment-metadata subset.
- APIs cover the first method/route metadata and supported handler signatures.
- View parsing accepts the current literal markup subset and selected
  directives such as `g:post`, `g:target`, and `g:swap`.
- Configured `go addon.<name> {}` blocks validate known addon consumers and can
  emit generated app Go files through `gowdk.GoBlockConsumer`.

## Not Yet Implemented

- Full typed action semantics.
- Full API request/response body modeling.
- Broad local client-side reactivity.
- Full semantic/type analysis outside component contracts and inline package Go
  block slices.

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
- `audit.md`: `*.audit.gwdk` policy and generated audit test syntax.
- `partials.md`: partial update status and planned fragment behavior.
- `forms.md`: form submission, progressive enhancement, validation, and
  invalidation boundaries.
- `ssr.md`: SSR render-mode, `server {}`, and guard behavior.
- `hybrid.md`: hybrid request-time behavior and deferred hybrid capabilities.
- `diagnostics.md`: current diagnostic shape and known codes.
- `formatting.md`: current formatter behavior.
- `stability.md`: per-construct stability and deprecation tiers.
- `conformance.md`: machine-checked accept/reject corpus that pins the contract.

## File Kinds

The compiler currently treats every parsed file as a page file. A minimal page
uses:

```gwdk
route "/"
guard public
```

The page ID derives from the filename unless `page` is present.

Component files are supported as explicit or discovered `gowdk build` inputs
with `component`. Layout files are also supported. `*.audit.gwdk` files are a
separate audit policy/test kind consumed by `gowdk audit`; they do not generate
pages. Separate island file kinds are planned.
