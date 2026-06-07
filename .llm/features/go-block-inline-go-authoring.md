# Feature Spec: Go Block Inline Go Authoring

## Problem

GOWDK should let users colocate page/component-specific Go with `.gwdk` markup
when they want that workflow, without turning `.gwdk` into a second backend
language.

## Goals

- Add `go {}` as the organizing block for optional inline Go authoring.
- Preserve optional go block targets so SPA, SSR, and addons can define their own
  future extraction behavior.
- Keep separate `.go` files as the default supported path.
- Keep inline Go planned as real Go that will extract to importable, formatted,
  testable package code.

## Non-Goals

- Define all addon-specific go block semantics.
- Add a custom Go dialect or arbitrary non-Go scripting.

## Users And Permissions

- Primary users: Go developers authoring GOWDK apps.
- Roles or permissions: no special permissions.
- Data visibility rules: go block bodies may contain application code and must be
  treated like source code, not generated output or public assets.

## User Flow

1. A user adds a `go {}` block near related `.gwdk` declarations.
2. The parser captures the go block body and target.
3. Analyzer/IR preserve the go block.
4. Default `go {}` build-data functions can be called by
   `build { => LocalFunc() }`.
5. `go ssr {}` load handlers can execute through generated SSR adapters.
6. Configured addons implementing `gowdk.GoBlockConsumer` can validate targeted
   addon go blocks and emit generated app Go files.

## Requirements

### Functional

- Parse `go {}` on pages, components, and layouts.
- Parse targeted go blocks such as `go ssr {}` and
  `go addon.contracts {}`.
- Reject duplicate go block targets on the same source file.
- Preserve go block body, target, and source span in manifest compatibility
  records and IR.
- Parse go block bodies as Go during validation.
- Type-check saved default `go {}` blocks with sibling Go files in the same
  package during validation.
- Execute default `go {}` no-argument build-data functions when referenced by
  `build { => LocalFunc() }`.
- Bind same-page action, API, and fragment handlers from default `go {}` when
  no same-package `.go` handler exists.
- Compile page-level `go client {}` mounts to Go WASM when the block
  exports `GOWDKMount<PageID>` with `//go:wasmexport`.
- Execute `go ssr {}` load handlers through generated SSR adapters.
- Diagnose `go ssr {}` on pure SPA pages.
- Diagnose `go addon.<name> {}` when the named addon is not configured.
- Let configured addons implementing `gowdk.GoBlockConsumer` validate and emit
  generated app Go files for `go addon.<name> {}`.

### Non-Functional

- Performance: parsing remains line-oriented and cheap.
- Reliability: unsupported targets are preserved, not interpreted.
- Accessibility: no direct impact.
- Security/privacy: Go code is source, not asset output.
- Observability: no runtime impact in this slice.

## Acceptance Criteria

- [x] `ParseSyntax` returns go blocks with body and target.
- [x] `ParsePage`, `ParseComponent`, and `ParseLayout` preserve go blocks.
- [x] Analyzer lowers go blocks into compiler IR.
- [x] Docs describe current build-time, SSR-load, and addon-consumer
  `go {}` behavior.
- [x] Saved default `go {}` blocks are type-checked with sibling Go files
  during validation.
- [x] Default `go {}` build-data functions can feed `build {}`.
- [x] Default `go {}` action, API, and fragment handlers can bind generated
  backend endpoints.
- [x] Generated app source materializes default `go {}` and `go ssr {}`
  blocks under `gowdk_go/`.
- [x] Page-level `go client {}` mounts emit WASM and loader assets.
- [x] `go ssr {}` load handlers execute in generated app binaries.
- [x] Addon go block targets produce semantic diagnostics.
- [x] Addon go block consumers can emit generated app Go files.
- [x] `go test ./...` passes.

## Edge Cases

- Nested braces inside `go {}` should not close the go block early.
- Duplicate `go {}` and duplicate targeted go blocks should be rejected.
- `go {}` inside `view {}` remains invalid because it is markup text, not a
  top-level authoring block.

## Dependencies

- Internal: ADR 0009.
- External: standard Go parser/type-checker for the future extraction slice.

## Open Questions

- Exact extraction output location and source-map strategy.
- Whether target names should be limited to known lanes/addons at parse time or
  validated by registered addons later.
