# Implementation Plan: SEO Structured Data And Dynamic Sitemap

## Context

Spec: `docs/product/seo-structured-data-and-dynamic-sitemap.md`

Issue: #637

## Assumptions

- GOWDK owns deterministic output for build-time public pages.
- Apps own request-time inventory, auth, and crawler visibility for dynamic
  URLs.
- The first structured-data slice supports only `WebPage` and `Article`.

## Proposed Changes

- Parse `jsonld <kind>` as page metadata and store it in IR.
- Validate supported kinds and reject duplicates with stable diagnostics.
- Render JSON-LD through Go JSON serialization and existing script escaping.
- Add runtime SEO sitemap helpers for deterministic URL normalization and XML
  response handling.
- Extend SEO addon options with a dynamic provider import path/function and
  runtime caps/cache settings.
- Generate `/sitemap.xml` in embedded apps when the dynamic provider is
  configured.
- Document and example the new contracts.

## Files Expected To Change

- Parser/analyzer/IR: `internal/parser`, `internal/compiler`,
  `internal/gwdkir`, `internal/lang`.
- Output/app generation: `internal/buildgen`, `internal/appgen`.
- Public API/addon/runtime: `gowdk.go`, `addons/seo`, `runtime/seo`.
- Docs/examples: `docs/reference/seo.md`, `docs/reference/config.md`,
  `docs/language`, `examples/seo`.

## Data And API Impact

- New public config field: `SEOOptions.DynamicSitemap`.
- New public type aliases: `SEOURL` now points at `runtime/seo.URL`;
  `addons/seo.DynamicSitemap` aliases the root dynamic sitemap type.
- Manifest metadata gains `metadata.jsonld`.
- Build report gains `seo/structured_data` events.

## Tests

- Unit: parser metadata, compiler validation, runtime sitemap normalization and
  handler behavior.
- Integration: build output JSON-LD rendering, appgen dynamic sitemap route,
  config loading.
- End-to-end: SEO example build.
- Manual: inspect generated HTML and `/sitemap.xml` output when serving a
  generated app.

## Verification Commands

```sh
go test ./runtime/seo
go test ./internal/parser ./internal/compiler
go test ./internal/buildgen
go test ./internal/appgen ./internal/project
go run ./cmd/gowdk build --config examples/seo/gowdk.config.go --out /tmp/gowdk-seo-build examples/seo/*.gwdk
```

## Rollback Plan

- Remove `DynamicSitemap` from SEO options and appgen route registration.
- Remove `jsonld` metadata parsing/validation and JSON-LD rendering.
- Keep static `sitemap.xml` and `robots.txt` behavior unchanged.

## Risks

- Provider functions can expose private URLs if app code skips its own policy;
  docs must state that the provider owns visibility.
- More schema kinds can create compatibility pressure; keep this slice narrow
  until concrete examples justify expansion.
