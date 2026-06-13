# Feature Spec: Milestone 8 Generated Adapter IR

## Problem

Generated app output supports one-binary, split frontend proxy, and backend-only
artifacts, but some backend decisions still read raw action/API/fragment option
slices instead of the typed backend adapter IR. That makes route registration,
proxy route matching, guards, rate limits, CSRF, imports, and fallback behavior
harder to reason about as endpoint kinds grow.

## Goals

- Make backend adapter generation use one typed IR for endpoint registrations,
  request decoding metadata, handler calls, response metadata, and fallback
  metadata.
- Keep generated app, split frontend proxy, and backend-only generation on the
  same backend route metadata.
- Preserve current generated app behavior and public runtime contracts.
- Keep generated Go emitted through AST/printer/format.

## Non-Goals

- Do not add new public `.gwdk` syntax.
- Do not change route manifest or asset manifest JSON shapes.
- Do not remove compatibility fallback hooks in `runtime/app`.
- Do not migrate SSR route generation into this backend adapter IR slice.

## Users And Permissions

- Primary users: GOWDK maintainers and app authors who inspect generated Go.
- Roles or permissions: generated route guards and rate limiting keep their
  existing semantics.
- Data visibility rules: generated error responses continue to hide ordinary
  5xx handler details and avoid exposing secrets.

## User Flow

1. An app author declares actions, APIs, fragments, or web contract references.
2. The compiler validates bindings and builds endpoint metadata.
3. App generation lowers backend metadata into `BackendAdapterIR`.
4. One-binary, split proxy, or backend-only output uses that IR to register,
   match, guard, limit, and dispatch request-time backend routes.

## Requirements

### Functional

- Backend route registration must be derived from `BackendAdapterIR`.
- Split frontend proxy route matching must use the same registration metadata
  as backend route registration.
- Guard, rate-limit, CSRF, and backend import decisions must be derived from
  adapter IR where they depend on backend endpoint metadata.
- Bound action/API/fragment handler calls and missing/unsupported fallbacks must
  remain represented in adapter IR and covered by tests.

### Non-Functional

- Performance: route matching remains a simple generated switch plus existing
  dynamic fragment matcher.
- Reliability: generated app source remains gofmt-formatted and golden-tested.
- Accessibility: no user-visible markup changes.
- Security/privacy: body limits, CSRF ordering, guard checks, rate limits, and
  no-store error behavior stay unchanged.
- Observability: generated route metadata remains inspectable in the generated
  source and existing CLI reports.

## Acceptance Criteria

- [ ] `internal/appgen` can prove adapter IR contains registrations, decoders,
  handler calls, responses, fallbacks, guards, imports, and dynamic-route flags.
- [ ] Generated app and backend-only source use adapter IR for backend router
  construction.
- [ ] Split frontend proxy source uses adapter IR for backend route matching.
- [ ] Existing generated app goldens and appgen tests pass.
- [ ] The roadmap step 8 row is accurate after implementation.

## Edge Cases

- Proxy frontend builds must not import user backend handler packages.
- Dynamic fragment routes still need runtime route matching in split proxy
  output.
- Public guards are omitted from runtime guard execution.
- Guardless request-time pages remain handled by the existing SSR/page route
  logic, not the backend adapter IR.

## Dependencies

- Internal: `internal/appgen`, `internal/gwdkir`, `runtime/app`.
- External: none.

## Open Questions

- None for this slice. SSR route unification remains a later generated-output
  cleanup, not part of milestone 8.
