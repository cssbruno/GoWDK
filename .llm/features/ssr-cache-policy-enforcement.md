# Feature Spec: SSR Cache Policy Enforcement

## Problem

`@cache` is parsed and carried through route metadata, but generated SSR
binaries still wrote successful HTML responses with `Cache-Control: no-store`.
That made the syntax informational rather than enforceable for request-time
pages.

## Goals

- Apply explicit page `@cache` values to successful generated SSR HTML
  responses.
- Preserve no-store defaults for request-time pages without `@cache`.
- Preserve no-store behavior for errors, load redirects, actions, APIs,
  fragments, and CSRF-mutated HTML.

## Non-Goals

- Full SPA route cache policy.
- Revalidation directives beyond literal HTTP `Cache-Control` values.
- Cache policy for user-owned API/action responses.
- Automatic cache safety analysis for personalized `load {}` data.

## Users And Permissions

- Primary users: GOWDK app authors deploying generated SSR binaries.
- Roles or permissions: no new roles.
- Data visibility rules: cache policy is explicit author intent; GOWDK does not
  infer whether loaded SSR data is personalized.

## User Flow

1. A page declares `@cache "public, max-age=60"`.
2. The parser/analyzer carries the cache value through route metadata.
3. Appgen emits SSR response code with the explicit cache policy.
4. The generated binary returns that `Cache-Control` on successful SSR HTML.

## Requirements

### Functional

- Successful generated SSR HTML uses `@cache` when present.
- Successful generated SSR HTML without `@cache` remains no-store.
- HEAD requests still suppress the body.
- Generated SSR route metadata still records the cache value.

### Non-Functional

- Performance: no additional request-time route lookup.
- Reliability: generated source must compile and tests must prove header output.
- Accessibility: no UI impact.
- Security/privacy: generated errors and CSRF-mutated responses stay no-store.
- Observability: route metadata includes cache policy.

## Acceptance Criteria

- [x] Runtime HTML writer supports explicit cache policy with no-store fallback.
- [x] Appgen emits explicit-cache SSR response calls when `SSRRoute.Cache` is
      set.
- [x] Generated source tests assert the cache writer is emitted.
- [x] Generated binary tests assert `Cache-Control` is applied.
- [x] Deployment/routing docs describe the enforced behavior and remaining
      no-store safety boundaries.

## Edge Cases

- Empty cache policy should fall back to no-store.
- HEAD responses should preserve headers and suppress body.
- SSR errors and load redirects must not inherit the page cache policy.

## Dependencies

- Internal: `runtime/response`, `internal/appgen`.
- External: none.

## Open Questions

- What syntax should cover SPA route revalidation and hybrid branch cache
  policy?
