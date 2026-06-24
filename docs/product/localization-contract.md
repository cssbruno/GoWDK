# Feature Spec: Localization Contract

## Problem

Apps that publish the same page tree in multiple languages need generated page
routes, build data, request-time metadata, and message lookup to agree on the
active locale. Without a compiler-owned route contract, each app has to rebuild
locale prefixes, manifests, SSR context, and typed message keys by hand.

## Goals

- Declare supported locales in `gowdk.Config`.
- Generate build-time and request-time page routes once per locale.
- Pass the active locale into build helpers and SSR route contexts.
- Provide dependency-free typed Go message catalog helpers.
- Provide deterministic Go-owned catalog completeness checks and catalog
  templates for CI.
- Provide bounded plural, number, date, and time formatting helpers without a
  mandatory JavaScript or npm runtime.
- Keep inspection output and SEO sitemap generation locale-aware.

## Non-Goals

- ICU MessageFormat or CLDR completeness.
- Compiler extraction from arbitrary dynamic Go expressions.
- Browser language negotiation or automatic redirects.
- Translating compiler diagnostics.
- Localizing action, API, fragment, command, or query endpoint paths.

## Users And Permissions

- Primary users: GOWDK app authors building localized static or SSR page trees.
- Roles or permissions: no new auth roles.
- Data visibility rules: locales are public route metadata; message contents
  are app-owned source data.

## User Flow

1. Add `Config.I18N.Locales` with locale codes and optional URL prefixes.
2. Use `gowdk.BuildParams.LocaleCode()` in build helpers to select copy.
3. Build the app and inspect localized routes in generated output and reports.
4. For SSR pages, read the active locale from `runtime/app.Locale(ctx)`.

## Requirements

### Functional

- Empty `Config.I18N.Locales` must preserve existing route output.
- Locale codes must be validated and route prefixes must be clean absolute
  path prefixes.
- SPA, dynamic `paths {}` SPA, SSR, and hybrid page routes must expand per
  locale.
- Generated HTML must carry `<html lang="...">` for localized pages.
- `gowdk-routes.json`, site-map JSON, route reports, and SEO sitemap output
  must include localized page routes.
- Build helpers must receive `gowdk.BuildParams.Locale`.
- Generated SSR handlers must attach locale metadata to request contexts.
- `runtime/i18n` must provide typed catalog and bundle lookup with fallback.
- `runtime/i18n` must provide deterministic message-reference reports and
  catalog templates so apps can fail CI on missing or stale translations.
- `runtime/i18n` must provide bounded plural, number, date, and time helpers
  for documented locale families with stable fallback behavior.

### Non-Functional

- Performance: locale expansion should be linear in pages times locales.
- Reliability: invalid locale policy must fail during config loading.
- Accessibility: generated `lang` attributes must match active locale codes.
- Security/privacy: locale prefixes must reject unsafe path traversal shapes.
- Observability: generated manifests and site maps must expose route locales.

## Acceptance Criteria

- [x] Config loading accepts literal and executable `I18N` config.
- [x] Invalid locales and prefixes fail validation.
- [x] Build output writes localized HTML files and route manifests.
- [x] SSR auto-routes include localized paths and runtime locale context.
- [x] Site-map JSON and SEO sitemap output include localized routes.
- [x] Typed message catalog helpers are covered by runtime tests.
- [x] Typed catalog reports/templates can detect missing and unused keys.
- [x] Bounded plural, number, date, and time formatting helpers are covered by
      runtime tests.
- [x] Docs and examples cover the implemented contract.

## Edge Cases

- `OmitDefaultPrefix` keeps the default locale on the original route.
- Custom `PathPrefix` values can shorten locale URLs, such as `/br`.
- Dynamic route params are expanded after locale prefixes are chosen.
- Endpoint routes keep their declared path and are not locale-prefixed. Apps
  that need endpoint-local locale policy should pass locale explicitly in
  user-owned request data, headers, sessions, or normal Go handler context.

## Dependencies

- Internal: config loader, compiler route metadata, buildgen, appgen, SEO
  output, `runtime/app`.
- External: none.

## Open Questions

- Which `.gwdk` message-use syntax should feed compiler-owned extraction first?
- Should a future runtime addon negotiate browser `Accept-Language`?
- Should endpoint-local locale policies be declared in source or config?
