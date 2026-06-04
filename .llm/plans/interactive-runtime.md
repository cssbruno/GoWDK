# Implementation Plan: Interactive Runtime

## Context

Relevant spec: `.llm/features/interactive-runtime.md`

GOWDK should stop feeling like a fancy HTML displayer by shipping real
interaction paths while preserving its compile-first identity. Static HTML stays
the base output. Interactivity is layered in this order:

1. Server fragments for action-driven updates.
2. Stronger form/action behavior.
3. Declarative local client islands.
4. Optional richer WASM islands after the lower-level contracts stabilize.

## Assumptions

- Static/action-first remains the core model.
- SSR remains optional and should not be required for partial updates.
- No npm dependency is required for the first interactive runtime.
- Normal forms must still work without JavaScript.
- The first local-state slice can be deliberately smaller than React/Svelte.

## Proposed Changes

- Phase 1: Complete action partials.
  - Emit `gowdk.js` only for pages with partial form metadata.
  - Add a script tag to those pages.
  - Generate action handlers that return fragment HTML for `X-GOWDK-Partial` requests.
  - Keep normal POST redirect behavior for non-JavaScript fallback.
  - Record the runtime asset in `gowdk-assets.json`.

- Phase 2: Harden action forms.
  - Wire CSRF into generated handlers.
  - Preserve validation failures as fragment responses when a partial request includes a target.
  - Add explicit diagnostics for actions that cannot produce a partial response.
  - Document request and response headers.

- Phase 3: Improve partial ergonomics.
  - Add loading and error target conventions.
  - Support multiple fragments per action selected by target.
  - Allow rendered fragment bodies to use the supported component subset.
  - Add examples for table refresh, inline validation, and modal body replacement.

- Phase 4: Add declarative client islands.
  - Define a small page-local syntax for local state and events.
  - Start with a counter/disclosure/select-filter slice.
  - Emit static initial HTML plus generated island runtime only for opted-in islands.
  - Avoid full-page hydration; islands attach to compiler-owned markers.

- Phase 5: Decide richer runtime strategy.
  - Compare generated JavaScript islands against Go WASM islands.
  - Keep the chosen path compatible with one-binary deploys.
  - Add an ADR before committing to a broad island architecture.

## Files Expected To Change

- `.llm/features/interactive-runtime.md`
- `.llm/plans/interactive-runtime.md`
- `internal/clientrt/runtime.go`
- `internal/clientrt/*_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `internal/appgen/appgen.go`
- `internal/appgen/appgen_test.go`
- `docs/language/partials.md`
- `docs/compiler/generated-output.md`
- `docs/product/requirements.md`
- Future island phase:
  - `internal/parser`
  - `internal/view`
  - `internal/codegen`
  - `runtime/*`
  - `docs/language/*`

## Data And API Impact

- Static output may include `assets/gowdk/gowdk.js`.
- `gowdk-assets.json` should list the emitted runtime asset.
- Partial requests use:
  - `X-GOWDK-Partial: 1`
  - `X-GOWDK-Target: #id`
  - `X-GOWDK-Swap: innerHTML|outerHTML`
- Fragment responses use:
  - `Content-Type: text/html; charset=utf-8`
  - `Cache-Control: no-store`
  - `X-GOWDK-Fragment-Target: #id`
  - optional `X-GOWDK-Fragment-Swap`
- Future islands will need stable generated DOM markers. That should get an ADR before expansion.

## Tests

- Unit:
  - Client runtime source includes partial headers, swap handling, loading state, and focus restoration.
  - Static build emits `gowdk.js` only when a page uses partial metadata.
  - Action route extraction renders fragment bodies and preserves redirect fallback.

- Integration:
  - Generated binary serves partial fragment responses.
  - Generated binary keeps normal POST redirects for non-partial requests.
  - Validation and unexpected fields return explicit action errors.

- End-to-end:
  - Browser/DOM harness submits a partial form and verifies target swap plus focus restoration.
  - Example app build command compiles a binary and exercises `/patients`.

- Manual:
  - Run an example page in the generated binary.
  - Submit with JavaScript enabled and disabled.

## Verification Commands

```sh
go test ./internal/clientrt ./internal/staticgen ./internal/appgen
go test ./...
go build ./cmd/gowdk
go run ./cmd/gowdk build --out /tmp/gowdk-partials-build --app /tmp/gowdk-partials-app --bin /tmp/gowdk-partials-site examples/basic/patients-fragment.page.gwdk
```

## Rollback Plan

- Remove runtime asset emission and script injection from `internal/staticgen`.
- Remove fragment response handling from `internal/appgen`.
- Keep parser and manifest fragment metadata, since they already existed.
- Revert docs to describing partials as metadata-only.

## Risks

- Scope creep into a full SPA runtime before actions and partials are production-grade.
- Fragment rendering could bypass escaping if it does not go through `internal/view` or `runtime/render`.
- CSRF is still required before claiming production-ready action behavior.
- Local islands may create a second programming model if syntax is not kept narrow.
- Generated JavaScript and Go WASM could compete unless the runtime strategy is decided with an ADR.
