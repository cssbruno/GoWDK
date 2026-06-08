# Draft v0.1 Release Notes

GOWDK v0.1 is an experimental pre-1.0 compiler/runtime release. It should not
be described as production-ready.

Release builds must use Go `1.26.4` or newer in the Go 1.26 line. Earlier Go
1.26 patch versions have reachable standard-library vulnerabilities reported by
`govulncheck`.

## Implemented

- Package-peer `.gwdk` parsing for pages, components, layouts, routes, render
  modes, guards, actions, APIs, fragments, CSS metadata, stores, and source
  spans.
- Typed GOWDK AST, analyzer metadata, and versioned compiler IR for the current
  compiler slice.
- Project config loading from `gowdk.config.go` or `--config <file>` for
  project-level compiler commands.
- `gowdk build --out` for build-time SPA output, route manifests, asset
  manifests, and build reports.
- Literal dynamic `paths {}` expansion for the first supported dynamic SPA
  route subset.
- Literal, imported, and same-package no-argument `build {}` data for the
  supported expression subset.
- Generated build output for declared layouts, discovered components, page CSS,
  component CSS, layout CSS, configured stylesheets, and compile-time CSS
  processors.
- Optional Tailwind v4 standalone CLI wrapper through `addons/tailwind`, using
  `tailwindcss` on `PATH` or an explicit `tailwind.Options.Command`. GOWDK does
  not download Tailwind.
- Generated app output with embedded SPA files, selected modules, generated
  backend routes, local binary builds, and Go `js/wasm` deploy artifacts.
- Typed generated actions for supported same-package handler signatures,
  direct literal request-shape validation, no-store responses, local redirects,
  partial fragments, and opt-in generated CSRF.
- Feature-bound generated API handlers for the supported API signature.
- Standalone fragments and partial form enhancement for the current fragment
  slice.
- Concrete and dynamic request-time SSR generated pages for the supported SSR
  slice, including declared load paths, guards, error pages, and no-store panic
  boundaries.
- Component JavaScript islands and explicit component-level browser WASM island
  assets for the supported ABI slice.
- Component-level `@asset` file emission with content-hashed asset manifest
  entries and immutable generated binary cache headers.
- CLI commands for version, tokens, fmt, check, manifest, sitemap, routes,
  build, dev, preview, serve, lsp, contracts, graph, and trace.
- VS Code extension syntax, diagnostics, manifest/token/sitemap commands,
  route-map tree, and file-move helper behavior covered by Node tests.

## Partial

- Build-time data is limited to the documented expression and no-argument Go
  function subset.
- Component semantics are still narrower than the full target component model.
- CSS plugin support exists, but richer plugin capabilities remain planned.
- Actions cover generated request-shape validation, CSRF, redirects, fragments,
  selected handler signatures, and explicit reload outcomes for enhanced forms.
- APIs require user-owned Go for auth, validation, request parsing, and response
  policy beyond the supported generated dispatch signature.
- SSR supports the current generated request-time slice with explicit action
  outcomes for rerunning request-time data.
- Hybrid pages use the generated request-time lane; streaming, data refresh, and
  non-HTTP revalidation remain planned.
- Generated output compatibility is pre-1.0 and limited to the explicitly
  documented compatibility surface.

## Planned

- Generated handlers needed for the full v0.1 build-output target that are not
  already covered by current action/API/fragment/SSR generation.
- Arbitrary build-time statements beyond expression records.
- Full component semantics, including richer props, slots, bindings, and
  lifecycle behavior.
- Full downstream migration from compatibility structs to `internal/gwdkir`.
- Complete diagnostic spans for every v0.1-supported parser, route, view,
  component, client, package, build, and endpoint path.
- User-defined domain validation helpers beyond generated request-shape checks.
- Production operations hardening beyond the current deployment guidance.
- Automated dependency and license checks in CI.

## Intentionally Out Of Scope

- Production-readiness claims.
- Mandatory full-page SSR.
- Full-page hydration as the default browser model.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Gin, Echo, Fiber, Redis, NATS, or any other optional
  framework/tool dependency.
- GOWDK downloading Tailwind or other optional styling tools during builds.

## Required Release Verification

Run the full checklist in `docs/engineering/v0.1-release-checklist.md` before
publishing. The release must stay draft or experimental if any required item in
that checklist remains unchecked.
