# Feature Spec: SEO Structured Data And Dynamic Sitemap

## Problem

GOWDK can emit static SEO files, but content-heavy apps need structured data in
generated pages and a way to include request-time or database-owned URLs in a
sitemap without making the compiler guess app inventory policy.

## Goals

- Let pages declare supported JSON-LD schema kinds with `.gwdk` metadata.
- Validate unsupported or duplicate structured-data declarations at compile
  time.
- Emit deterministic, safely escaped JSON-LD for generated pages.
- Let generated apps serve `/sitemap.xml` from static public URLs plus an
  optional app-owned dynamic provider.
- Preserve guard, `noindex`, and request-time visibility rules for
  compiler-owned sitemap entries.
- Expose the behavior through docs, examples, manifest metadata, and build
  report events.

## Non-Goals

- Owning Search Console setup, CDN policy, or crawler operations.
- Discovering database-backed URLs automatically.
- Modeling every schema.org type in the first slice.
- Overriding app authorization policy for request-time-only URLs.

## Users And Permissions

- Primary users: GOWDK app authors and docs/tooling authors.
- Roles or permissions: public pages can contribute static sitemap URLs; app Go
  code owns dynamic URL visibility.
- Data visibility rules: guardless, protected, and `noindex` pages are excluded
  from GOWDK-owned static sitemap entries.

## User Flow

1. The author adds `jsonld WebPage` or `jsonld Article` to a page.
2. `gowdk check` rejects unsupported or duplicate schema declarations.
3. `gowdk build` emits JSON-LD in generated HTML and records report metadata.
4. The author optionally configures `seo.DynamicSitemap` with an app-owned Go
   provider.
5. The generated app serves `/sitemap.xml` by merging public static URLs and
   provider URLs.

## Requirements

### Functional

- Support `jsonld WebPage` and `jsonld Article`.
- Emit one `application/ld+json` script per declaration.
- Include structured-data kinds in `gowdk manifest`.
- Record `seo/structured_data` build report events.
- Generate a runtime `/sitemap.xml` route only when `DynamicSitemap` is fully
  configured.
- Normalize, sort, deduplicate, and cap dynamic sitemap URLs.

### Non-Functional

- Performance: sitemap normalization is bounded by `MaxURLs`; default runtime
  cap applies when no explicit cap is configured.
- Reliability: provider errors return `503` with `Cache-Control: no-store`.
- Accessibility: no visual UI impact.
- Security/privacy: generated JSON uses structured serialization and script
  escaping; dynamic providers own app-specific visibility.
- Observability: diagnostics, manifest metadata, and build report events expose
  the feature.

## Acceptance Criteria

- [x] Unsupported structured-data kinds fail validation.
- [x] Duplicate structured-data kinds fail validation with related source span.
- [x] SPA HTML includes deterministic JSON-LD before stylesheets.
- [x] Generated app code registers dynamic `/sitemap.xml` before the page
  fallback route.
- [x] Runtime sitemap handler handles GET/HEAD, URL normalization, caps,
  provider errors, and cache headers.
- [x] SEO docs and examples show structured data and the dynamic provider.

## Edge Cases

- Request-time pages are excluded from build-time sitemap output unless the app
  provider returns their public URLs.
- Root-relative dynamic URLs are resolved against `BaseURL`.
- Absolute dynamic URLs must be `http` or `https`.
- Provider overflow and provider errors fail the request without exposing error
  details to crawlers.

## Dependencies

- Internal: parser metadata, compiler validation, buildgen head rendering,
  appgen route registration, runtime SEO helper package.
- External: none.

## Open Questions

- Broader schema kinds are planned follow-up work once concrete app examples
  need them.
