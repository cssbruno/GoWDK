# Feature Spec: API CORS Policy

## Problem

Generated API and web contract endpoints are same-origin only. Apps that expose
those endpoints to trusted browser clients on another origin need a bounded CORS
policy, preflight handling, and explicit credentials rules.

## Goals

- Add config-level CORS policy for generated API, command, and query routes.
- Add endpoint-level `.gwdk` CORS policy syntax for generated API routes.
- Keep the default closed: no CORS headers are emitted unless enabled.
- Handle browser `OPTIONS` preflight before guards, rate limiting, CSRF, or user
  handlers.
- Reject wildcard origins combined with credentials.

## Non-Goals

- Treating CORS as authentication or authorization.
- CORS for generated action, fragment, SSR, or static page routes.

## Requirements

### Functional

- `gowdk.BuildConfig.CORS` declares allowed origins, methods, headers, exposed
  headers, credential support, and preflight max-age.
- `.gwdk` API declarations can add a trailing `cors` clause with `origins`,
  `methods`, `headers`, `expose`, `credentials`, and `maxAge` options.
- Generated embedded and backend-only apps install the policy when API or web
  contract routes exist.
- Matching preflight requests return `204` with CORS headers.
- Existing API and web contract preflights fail closed with `403` when no policy
  allows them.

### Non-Functional

- Security/privacy: default same-origin, explicit origin allowlist, no `*` with
  credentials, and no CORS bypass of guards or handler authorization.
- Reliability: invalid policy fails during config load or generated router setup.
- Observability: generated route handling stays in the existing backend router.

## Acceptance Criteria

- [x] Config-level CORS policy is parsed from `gowdk.config.go`.
- [x] Generated API/contract routers answer preflight without calling handlers.
- [x] Actual generated API/contract responses include CORS headers for allowed
  origins.
- [x] Wildcard origin plus credentials is rejected.
- [x] Endpoint-level API CORS clauses are parsed, lowered into IR, generated as
  route-local runtime policies, and validated with the same credentialed
  wildcard safety rule.
- [x] Runtime, appgen, and config tests cover the slice.

## Edge Cases

- Literal origins are normalized to `scheme://host`; URL paths, queries, and
  fragments are rejected.
- Requested preflight headers must be listed in `AllowedHeaders`.
- `AllowedMethods` is optional; when omitted, the matched route method is used.

## Dependencies

- Internal: `gowdk.Config`, `internal/project`, `internal/appgen`,
  `runtime/app.BackendRouter`.
- External: none.

## Open Questions

- Whether split frontend proxy routes should optionally terminate CORS instead
  of leaving policy to the backend app.
