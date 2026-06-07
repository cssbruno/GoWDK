# Implementation Plan: `go {}` Inline Go Authoring

## Context

Relevant spec: `.llm/features/go-block-inline-go-authoring.md`

Relevant ADR: `docs/engineering/decisions/0009-optional-inline-go-authoring.md`

User direction:

- Go code should be importable and writable inside `.gwdk` when the user wants
  colocated authoring.
- `go {}` should organize that code.
- SPA, SSR, and addons need room for their own go block behavior.

## Product Rule

`go {}` is an authoring container for real Go. It must not become a custom
Go dialect or a second backend language.

The compiler may extract, format, type-check, and wire inline Go, but the final
code must behave like normal package Go:

- importable from generated adapters;
- visible to `go test` and `go build`;
- type-checked by the standard Go toolchain;
- source-mapped back to `.gwdk` for diagnostics;
- not generated user behavior.

## Proposed Syntax

Plain package go block:

```gwdk
go {
func HomePageForBuild() PageCopy {
	return PageCopy{Title: "Home"}
}
}
```

Lane-targeted go blocks:

```gwdk
go spa {
func ClientSeed() ClientState {
	return ClientState{}
}
}

go ssr {
func LoadDashboard(ctx ssr.LoadContext) (DashboardData, error) {
	return DashboardData{}, nil
}
}
```

Addon-targeted go blocks:

```gwdk
go addon.contracts {
func RegisterContracts(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, Result](r, HandleCreatePatient)
}
}
```

Target names are preserved as strings by the parser. Semantic validation
decides which targets are legal for a configured render mode or registered
addon.

## Target Semantics

| Target | Owner | Planned Meaning |
| --- | --- | --- |
| empty `go {}` | Compiler core | General package Go extracted into the declaring package. |
| `go spa {}` | Compiler core or SPA addon | Build-time/static-first helpers, SPA-only generated wiring, and client seed data. Must not imply request-time page rendering. |
| `go ssr {}` | SSR addon | Request-time helpers such as load functions, guards, SSR-only response helpers, and route-local logic. Requires SSR/hybrid request-time capability before execution. |
| `go addon.<name> {}` | Named addon | Addon-owned extracted Go and validation hooks. The addon decides required imports, signatures, generated adapter wiring, and diagnostics. |

## Addon Model

Addons should be able to consume go block targets without the compiler hardcoding
every feature-specific rule.

Planned addon contract:

```go
type GoBlockTarget struct {
	Target string
	OwnerPackage string
	SourcePath string
	Body string
	Span manifest.SourceSpan
}

type GoBlockConsumer interface {
	GoBlockTargets() []string
	ValidateGoBlock(target GoBlockTarget, ctx GoBlockContext) []Diagnostic
	GeneratedGo(target GoBlockTarget, ctx GoBlockContext) ([]GoFile, error)
}
```

Rules:

- Core owns empty, `spa`, and generic extraction policy.
- SSR owns `ssr` go block validation and request-time wiring.
- Addon targets must use `addon.<name>` to avoid conflicts with core lanes.
- An addon must be configured before its target can execute. Unknown addon
  targets remain parseable but should be diagnostics during `check`/`build`.
- Addons must emit normal Go files or adapter metadata, not hidden runtime
  behavior.

## SPA Behavior

SPA pages stay build-time by default.

Allowed future use:

- build-time helper functions;
- static data shaping;
- page/component-local Go declarations used by generated build output;
- optional client seed state that becomes serialized enhancement data.

Disallowed:

- request-time `load` behavior;
- auth/session/database access at browser runtime;
- hidden full-page SSR.

`go spa {}` must never make a page request-rendered. If it needs server
data at request time, the page must use `@render ssr` or a future explicit
hybrid branch.

## SSR Behavior

`go ssr {}` is request-time Go. It can be valid only when the page or
component participates in a request-time lane.

Allowed future use:

- route-local load helpers;
- guard helpers;
- request-aware layout helpers;
- response/error helpers;
- typed route-param helpers.

Validation:

- `go ssr {}` on a pure SPA page should parse but produce a diagnostic
  unless SSR or hybrid request-time behavior is enabled.
- SSR go blocks must not be compiled into static-only output.
- Generated SSR adapters must call exported Go symbols through normal Go
  package imports or extracted package files.

## Extraction Design

Extraction for default build-time `go {}` helpers now happens in the
build-data runner path. Broader generated-file extraction should happen after
parsing/analyzer normalization and before generated adapter source is emitted.

Pipeline:

```text
.gwdk source
  -> parser captures go blocks
  -> analyzer assigns package/source/target metadata
  -> go block extractor emits virtual or generated .go files
  -> go/parser + go/format validate extracted files
  -> go/types validates package with normal .go files
  -> generated adapters import/call exported symbols
```

Output options to decide in the feature spec:

- hidden generated directory such as `.gowdk/gen/scripts/<pkg>/...`;
- generated app source directory only;
- in-memory virtual files for `check`, materialized files for `build`;
- source map comments linking generated Go back to `.gwdk` line ranges.

The first implementation should avoid writing extracted files next to user
source unless explicitly requested.

## Diagnostics

Required diagnostics:

- duplicate `go` target in one source file;
- invalid target syntax;
- unknown addon target when addon is not configured;
- `go ssr` used without request-time rendering;
- extracted Go parse error with `.gwdk` line mapping;
- extracted Go format/type error with `.gwdk` line mapping where possible;
- symbol conflict between inline Go and normal `.go` files;
- generated file/import cycle from extracted scripts.

## Implementation Phases

### Phase 1: Metadata Slice

- Parse `go {}` and `go <target> {}`.
- Preserve target, body, and span in AST, manifest compatibility records, and
  IR.
- Reject duplicate targets in the same source file.
- Add docs marking execution/extraction as planned.
- Status: implemented.

### Phase 2: Core Extraction

- Parse default `go {}` blocks as Go during validation.
- Extract default `go {}` build helpers into a temporary Go runner for
  `build { => LocalFunc() }`.
- Format and parse extracted Go.
- Include extracted files in `check` and `build` package validation.
- Preserve source maps for diagnostics.
- Keep generated adapters unchanged except for seeing symbols from extracted
  Go.
- Status: partial. Build-time helper execution and validation-time type
  checking with normal `.go` files exist for saved default and `go spa {}`
  blocks. Generated app source materializes default, `go spa {}`, and
  `go ssr {}` blocks under `gowdk_go/`. Source-adjacent extracted
  files remain planned.

### Phase 3: Build-Time Integration

- Allow `build { => LocalFunc() }` to resolve functions from extracted inline
  Go in the same package.
- Keep imported build functions working.
- Add tests for slug/data shaping authored in `go {}`.
- Status: implemented for default and `go spa {}` build-data functions.

### Phase 4: SSR Integration

- Enable `go ssr {}` only for SSR or explicit hybrid request-time lanes.
- Let generated SSR adapters bind load/guard/helper symbols from extracted Go.
- Add SSR diagnostics for pure SPA misuse.
- Status: implemented for generated SSR load handlers. Guard/helper symbols
  remain planned.

### Phase 5: SPA/Addons

- Define `go spa {}` behavior for static-first helpers and client seed
  state.
- Compile page-level `go spa {}` browser mounts to Go WASM when the block
  exports `GOWDKMount<PageID>` with `//go:wasmexport`.
- Add addon go block consumer registration.
- Let addons validate and emit adapter Go for `go addon.<name> {}`.
- Add docs for addon authors.
- Status: implemented for configured addons that implement
  `gowdk.GoBlockConsumer`, for `go spa {}` static build-data helpers, and
  for page-level browser `go spa {}` WASM mounts. SPA client seed state and
  broader addon adapter semantics remain planned.

## Files Expected To Change

- Parser/AST/IR:
  - `internal/gwdkast/ast.go`
  - `internal/manifest/manifest.go`
  - `internal/gwdkir/ir.go`
  - `internal/parser/syntax.go`
  - `internal/parser/page.go`
  - `internal/parser/page_lower.go`
  - `internal/gwdkanalysis/analyzer.go`
- Extraction/build:
  - new `internal/goblockgen` or `internal/inlinego`
  - `internal/compiler`
  - `internal/buildgen`
  - `internal/appgen`
  - `internal/gotypes`
- Docs:
  - `README.md`
  - `docs/language/blocks.md`
  - `docs/language/syntax.md`
  - `docs/compiler/pipeline.md`
  - addon author docs

## Tests

- Parser tests for plain, targeted, nested-brace, and duplicate go blocks.
- Analyzer/IR tests preserving targets and spans.
- Extraction tests for generated Go formatting and source maps.
- Build-data tests using a `go {}` function.
- SSR tests for `go ssr {}` accepted/rejected by render mode.
- Addon tests for `go addon.<name> {}` validation.
- Full `go test ./...`.

## Verification Commands

```sh
go test ./internal/parser ./internal/gwdkanalysis
go test ./...
```

## Rollback Plan

- Remove go block metadata from AST/manifest/IR.
- Remove parser recognition and diagnostics.
- Remove extraction pipeline and addon go block hooks.
- Revert docs to separate `.go` files only.

## Risks

- Users may assume `go {}` executes before extraction exists.
- Addon targets can become fragmented if naming is not controlled.
- Inline Go can make `.gwdk` files too large without style guidance.
- Source maps are required for usable diagnostics; without them, errors in
  extracted Go will feel disconnected from authoring.
