# Feature Spec: Typed Result Accessors

## Problem

Request-time pages can declare `server {}` data, but map-based load data leaves
field drift until runtime. A page can declare `server { => { user.name } }` while
the Go load function no longer exposes that field, and generated SSR still builds.

## Goals

- Allow SSR load handlers to return exported typed result structs.
- Check declared `server {}` field paths against exported struct fields.
- Keep generated Go as adapter glue and avoid runtime reflection in the typed
  load boundary.

## Non-Goals

- Do not replace existing `map[string]any` load handlers.
- Do not add typed action-result accessors until action result contracts are
  explicit.
- Do not make request-time rendering the default.

## Users And Permissions

- Primary users: GOWDK app developers using `server {}` and generated SSR.
- Roles or permissions: no new permission model.
- Data visibility rules: only exported Go fields are visible; `json:"-"` hides
  a field from typed load declarations.

## User Flow

1. Define an exported same-package result struct.
2. Return that struct from `Load<PageID>(ssr.LoadContext)`.
3. Declare `server { => { field, nested.field } }` paths that match exported Go
   field names or `json` tag names.
4. Run `gowdk check` or `gowdk build`; invalid declared paths fail before the
   generated app runs.

## Requirements

### Functional

- Load handlers may return `Data` or `(Data, error)` where `Data` is an
  exported same-package struct.
- Exported fields are addressable by Go field name or `json` tag name.
- Nested exported struct fields are available for declared path validation.
- Generated SSR converts top-level typed result fields into `map[string]any` for
  the existing SSR runtime path.

### Non-Functional

- Performance: generated adapters use direct field selectors, not reflection.
- Reliability: map load handlers keep existing dynamic behavior.
- Accessibility: no user-facing UI changes.
- Security/privacy: unexported fields and `json:"-"` fields are not exposed.
- Observability: existing SSR load tracing remains unchanged.

## Acceptance Criteria

- [x] Same-package and inline load binding can classify typed struct results.
- [x] Unknown declared `server {}` fields on typed results produce diagnostics.
- [x] Generated SSR apps compile and serve typed load result data.
- [x] Manifest/go-binding metadata includes typed result fields.

## Edge Cases

- Pointer typed results are accepted; nil pointers expose an empty load map.
- Embedded fields are rejected to avoid ambiguous field paths.
- Map-returning load handlers do not get field-path validation.

## Dependencies

- Internal: backend binding metadata, server-list validation, appgen SSR routes.
- External: none.

## Open Questions

- What action-result contract should typed action accessors use?
- Should future client bindings consume typed load metadata beyond declared
  `server {}` fields?
