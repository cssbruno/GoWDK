# Implementation Plan: Bounded Markup Transitions

## Context

Relevant spec, issue, ADR, or discussion:

- Spec: `docs/product/markup-transitions-spec.md`
- Issue: https://github.com/cssbruno/GoWDK/issues/503
- ADR: `docs/engineering/decisions/0008-bounded-client-language.md`

## Assumptions

- Transition and animation names are literal CSS hook names.
- This slice targets JS islands for client `g:if` and keyed client `g:for`.
- Server-rendered `server {}` regions remain static request-time HTML and do
  not get transition hooks.

## Proposed Changes

- Add `g:transition` and `g:animate` to the closed directive set.
- Validate motion directive values and placement in `viewrender`.
- Emit `data-gowdk-transition` and `data-gowdk-animate` attributes.
- Extend `island.js` conditional and keyed-list lifecycles with class toggles,
  transition/animation end handling, and fallback cleanup.
- Update docs, stability metadata, and conformance coverage.

## Files Expected To Change

- `internal/viewparse/directives.go`
- `internal/viewrender/directives.go`
- `internal/viewrender/element.go`
- `internal/viewanalysis/contract_references.go`
- `internal/clientrt/assets/island.js`
- `internal/lang/stability.go`
- `internal/lang/conformance_coverage_test.go`
- `internal/viewrender/view_test.go`
- `internal/buildgen/islands_test.go`
- `docs/compiler/generated-output.md`
- `docs/language/components.md`
- `docs/language/diagnostics.md`
- `docs/language/markup.md`
- `docs/language/stability.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`

## Data And API Impact

- Public `.gwdk` markup gains two accepted directives.
- Generated HTML gains compiler-owned `data-gowdk-transition` and
  `data-gowdk-animate` attributes.
- No manifest shape changes.

## Tests

- Unit: `internal/viewrender` directive validation and rendering tests.
- Integration: `internal/buildgen` generated HTML/runtime source tests.
- End-to-end: browser island smoke for enter, leave interruption, and reorder.
- Manual: no separate example is required for this first slice; docs include
  the minimal CSS contract.

## Verification Commands

```sh
go test ./internal/viewparse ./internal/viewrender ./internal/viewanalysis ./internal/clientrt ./internal/buildgen ./internal/lang ./internal/diagnostics
go build ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
```

## Rollback Plan

- Remove the directives from the supported set and revert the runtime class
  toggles. Existing generated output without these directives is unaffected.

## Risks

- Class names could collide if unprefixed; the implementation uses `gowdk-*`.
- CSS transitions without an end event need a deterministic fallback cleanup.
- Reordering must not disturb keyed row reuse or event listener rebinding.
