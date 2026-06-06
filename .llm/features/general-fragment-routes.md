# Feature Spec: General Fragment Routes

## Problem

Partial updates currently depend on action POSTs returning first-slice fragment
metadata. GOWDK also needs server fragment endpoints that can be requested
independently of form submission, while keeping fragments in the backend
endpoint lane instead of turning them into page route kinds.

## Goals

- Add a concrete `.gwdk` declaration for generated fragment endpoints.
- Keep route metadata limited to static, SPA, SSR, and hybrid page routes.
- Generate normal Go backend adapter code for fragment endpoints.
- Return fragment responses through `runtime/response` with no-store headers.

## Non-Goals

- User-owned Go fragment handler binding.
- Dynamic fragment route params.
- Component expansion or request-time data inside fragment bodies.
- Client-side route ownership or SPA state ownership.

## Users And Permissions

- Primary users: Go developers adding partial updates to pages.
- Roles or permissions: fragment routes share the declaring page's generated
  guard and rate-limit wiring.
- Data visibility rules: generated fragment bodies are static markup in this
  first slice; request-specific or sensitive behavior stays in Go handlers.

## User Flow

1. A page declares `fragment Patients GET "/patients/list" "#patients" { ... }`.
2. `gowdk build --app` emits a generated backend route with kind `fragment`.
3. Requests to the route receive a no-store fragment response for the target.

## Requirements

### Functional

- Parse top-level fragment endpoint declarations on page files.
- Lower fragment declarations through manifest compatibility structs and IR.
- Generate embedded and backend-only app handlers for fragment endpoints.
- Include fragment endpoints in split-backend proxy route detection.

### Non-Functional

- Performance: fragment body rendering happens at build/codegen time.
- Reliability: unsupported methods and malformed paths/targets fail early.
- Accessibility: partial client focus-restoration behavior remains unchanged.
- Security/privacy: generated fragments are static; user data belongs in Go.
- Observability: endpoint metadata marks generated handlers as `fragment`.

## Acceptance Criteria

- [ ] `go test ./internal/parser ./internal/gwdkanalysis ./internal/appgen` covers fragment parsing, IR lowering, generated source, and generated binary behavior.
- [ ] `go test ./...` passes.
- [ ] `go build ./cmd/gowdk` passes.

## Edge Cases

- Fragment routes must be concrete absolute paths with no route params.
- Fragment targets must be literal id selectors.
- Only `GET` is supported in this first slice.

## Dependencies

- Internal: existing backend router, AST-generated appgen, `runtime/response.FragmentFor`.
- External: none.

## Open Questions

- Whether future user-owned fragment handlers should bind to exported Go
  functions or always return `response.Response` from API/action handlers.
