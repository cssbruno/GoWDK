# Implementation Plan: Localization Contract

## Context

Relevant spec: [localization-contract.md](../product/localization-contract.md)

Tracking issues: `#506`, `#636`

## Assumptions

- Locale routing is project-wide config for this slice.
- Page routes are localized; backend endpoint paths remain declared exactly.
- Message catalogs are Go-owned data in this slice; compiler-extracted resource
  files need a later `.gwdk` message-use syntax decision.
- The implementation must preserve existing output when `I18N.Locales` is empty.
- Endpoint route localization remains explicit app-owned behavior for now.

## Proposed Changes

- Add `gowdk.I18NConfig`, `LocaleConfig`, and route-localization helpers.
- Validate `I18N` config in literal and executable config loading paths.
- Add `Locale` to `gowdk.BuildParams` and generated build-data runner calls.
- Expand build-time page artifacts and SSR artifacts per localized route.
- Add `lang` attributes, route manifest locale fields, site-map locale fields,
  and SEO sitemap localized page URLs.
- Attach localized SSR route metadata to `runtime/app` request contexts.
- Add `runtime/i18n` typed catalog and bundle helpers.
- Add deterministic `runtime/i18n` message references, bundle reports, catalog
  templates, and bounded locale formatting helpers.
- Document the config, routing, manifest, and generated-output contract.

## Files Expected To Change

- Root config/runtime API: `gowdk.go`, `runtime/app`, `runtime/i18n`.
- Config loading: `internal/project`.
- Generated output: `internal/buildgen`.
- Generated app source: `internal/appgen`.
- Route and tooling metadata: `internal/compiler`, `internal/lang`.
- Docs and examples: `docs/product`, `docs/reference`, `docs/compiler`,
  `docs/engineering`, `examples/i18n`.

## Data And API Impact

- `gowdk.Config` gains `I18N`.
- `gowdk.BuildParams` gains `Locale`.
- `runtime/app.RouteMetadata` gains `Locale`.
- `gowdk-routes.json` and site-map JSON include optional `locale`.
- Existing non-localized output remains unchanged.
- `runtime/i18n` gains additive helper types and functions; existing catalog
  APIs remain source-compatible.

## Tests

- Unit: `I18NConfig` validation/localization, `runtime/i18n` catalog lookup,
  catalog completeness reports, templates, plural/number/date/time formatting,
  config parser/executable loader, runtime app locale context.
- Integration: localized build output, route manifests, SEO sitemap, compiler
  route metadata, site-map JSON.
- End-to-end: localized generated app source for SSR auto-routes.
- Manual: build the `examples/i18n` project.

## Verification Commands

```sh
go test ./runtime/i18n ./runtime/app ./internal/project ./internal/buildgen ./internal/compiler ./internal/lang ./internal/appgen
go test ./...
go build ./cmd/gowdk
go run ./cmd/gowdk build --config examples/i18n/gowdk.config.go --out /tmp/gowdk-i18n-build examples/i18n/*.gwdk
```

## Rollback Plan

- Remove `Config.I18N` usage from project config and rebuild. With no configured
  locales, generated routes and manifests return to the previous shape.
- Revert the route-expansion helpers and generated locale metadata if the public
  contract changes before release.
- Remove only the additive `runtime/i18n` helpers if the future extraction
  format chooses a different report shape.

## Risks

- Locale-prefixed routes can multiply output size by the number of locales.
- A future locale-negotiation feature may need redirects or alternate links,
  which this slice intentionally does not own.
- Message extraction and ICU formatting may require a different catalog source
  format later; this slice keeps catalogs in normal Go and documents the limit.
