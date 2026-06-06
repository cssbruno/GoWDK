# Feature Spec: SSR Load Execution

## Problem

`load {}` is already parsed and validated as request-time page behavior, but
generated SSR handlers currently reject pages that declare it. That blocks the
request-time page lane from using user-owned Go logic for request data.

## Goals

- Allow `@render ssr` and request-time hybrid pages with `load {}` to generate
  SSR handlers.
- Keep load behavior in normal Go packages.
- Keep generated code as adapter glue: route metadata, guard execution, load
  call, error writing, escaping, and response writing.

## Non-Goals

- Do not add a JavaScript-owned page loading policy.
- Do not execute arbitrary statements from `load {}` inside the compiler.
- Do not add session/auth/storage implementations.

## Requirements

### Functional

- `load { => { field } }` declares request-time interpolation keys.
- Generated SSR handlers call an exported same-package Go function named
  `Load<PageID>` with `ssr.LoadContext`.
- Supported function signatures are:
  - `func LoadPage(ssr.LoadContext) (map[string]any, error)`
  - `func LoadPage(ssr.LoadContext) map[string]any`
- Returned scalar values replace generated placeholders in SSR HTML after HTML
  escaping.
- Load failures return a no-store HTTP 500 response.

### Non-Functional

- Security/privacy: generated handlers must escape load values before writing
  HTML.
- Reliability: missing or unsupported load functions produce explicit 501
  handler output rather than silently rendering placeholders.

## Acceptance Criteria

- [ ] Buildgen no longer rejects SSR pages with first-slice `load {}` keys.
- [ ] Appgen imports and calls the same-package load function for auto routes.
- [ ] Generated SSR handlers replace load placeholders with escaped values.
- [ ] Tests cover bound, missing, and unsupported load handlers.

## Dependencies

- `addons/ssr.LoadContext`
- Existing backend binding metadata and appgen import alias policy
