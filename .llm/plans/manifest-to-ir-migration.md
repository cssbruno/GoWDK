# Implementation Plan: Manifest → IR Migration

## Status (progress)

Done:

- Centralized the IR→manifest converter as `compiler.ManifestFromIR` and added
  IR-first validation entrypoints `compiler.ValidateProgram` /
  `compiler.ValidateBackendBindingPolicyIR`.
- Extracted shared leaf value types into the neutral `internal/source` package
  (manifest re-exports them as aliases). `internal/gwdkir`, `internal/gwdkast`,
  and `internal/contractscan` are now manifest-free (verified via
  `go list -deps`).
- Added IR-native `Page` behavior methods (`CachePolicy`, `RenderMode`,
  `HasGoBlock`, `DynamicParams`, `TypedRouteParams`) so render helpers consume IR.
- Migrated `internal/buildgen` render helpers to consume `gwdkir` models; only a
  `gotypes` bridge and public-API compat types remain.
- Swapped leaf-type references to `internal/source` across `appgen`, `compiler`,
  `parser`, `gwdkanalysis`, `lsp`.

Issue #145 progress (compiler IR-native validation):

- (done) Enriched the IR so it is no longer lossy for standalone Go endpoints:
  `gwdkir.GoEndpoint` preserves the raw declaration (kind/method/spans), and
  `ManifestFromIR` reconstructs `manifest.Endpoints` from it. This closes the
  silent-skip gap — `validateStandaloneEndpoints` / `validateRouteMethodConflicts`
  now run on the IR path.
- (done) Differential proof: `ValidateProgram(BuildIR(app))` is byte-identical to
  `ValidateManifest(app)` across a valid/invalid corpus (kept as a regression
  guard in `validate_differential_test.go`).
- (done) The CLI build path validates the IR directly via `ValidateProgram(ir)`,
  no longer threading a manifest into validation.
- (done) All validator bodies read `gwdkir` types directly; the `ManifestFromIR`
  adapter and `ValidateManifest` are deleted. The exhaustive validator test
  corpus kept its manifest fixtures and lowers them through the production
  `BuildIR` path, so diagnostic expectations were pinned byte-for-byte across
  the rewrite.
- (done) Standalone Go endpoint discovery (`compiler.DiscoverGoEndpoints`) and
  backend handler binding (`compiler.BindBackendHandlers`) are IR-native: they
  enrich `*gwdkir.Program` after `BuildIR` (via
  `gwdkanalysis.AddStandaloneEndpoints` / `AttachBackendBindings`). The binder
  returns the binding record list (now `source.BackendBinding`; manifest
  aliases it) so `lang` mirrors discovery/binding results back onto the parsed
  manifest — public manifest JSON output verified byte-identical.
- (done) `internal/gotypes` and `internal/goblockgen` take `gwdkir` types,
  deleting the IR→manifest conversion glue in buildgen and appgen.
- (done) The `lang` report commands (`check`/`sitemap`/`manifest`) validate the
  IR; manifest-typed `ValidateManifest`/`BuildRouteMetadata`/
  `ValidateBackendBindingPolicy` entrypoints are removed.
- (done) Residual build-path double validation removed: the CLI calls
  `buildgen.BuildFromValidatedIR`; `BuildFromIR` keeps the defensive validation
  for other callers.
- (done) `internal/compiler` no longer imports `internal/manifest` (non-test),
  verified via `go list` (Step 3 exit criterion).

Pending (largest remaining work):

- Step 4 residue: `internal/appgen` and `internal/lang`/`internal/lsp` still
  carry manifest-typed helpers around parsing and reports.
- Step 5: collapse the `AST → manifest → IR` path to `AST → IR` in
  `gwdkanalysis`; the parser seam and `BuildIR(manifest)` remain manifest-typed
  until then.
- Keep public manifest JSON until a release plan deprecates it.

## Context

`docs/engineering/architecture.md` ("Compatibility Records" section) tracks
`internal/manifest` compatibility records as known, well-documented debt. The
`gwdkir.Program` IR is meant to be the single stable compiler handoff, but
`internal/manifest` types are still threaded through six packages. Reference
counts of `manifest.` (non-test) today:

| Package | `manifest.` refs | Role |
| --- | ---: | --- |
| `internal/compiler` | 1058 | validation, backend binding, route metadata — gates everyone |
| `internal/buildgen` | 731 | render helpers + an **IR→manifest reconstruction shim** |
| `internal/appgen` | 214 | route/adapter planning (already IR-first at the boundary) |
| `internal/gwdkanalysis` | 176 | AST→manifest lowering, then manifest→IR build |
| `internal/parser` | 169 | emits manifest records beside typed AST |
| `internal/gwdkir` | 80 | re-exports a few shared manifest value types |
| `internal/lang` | 41 | CLI parse/report glue |
| `internal/gwdkast` | 35 | shares span/value types with manifest |

The single most important fact from the data-flow investigation: **manifest is
a mandatory intermediate, not a parallel output.** The current pipeline is
`AST → manifest → IR`, and `gwdkir` already models essentially every record
manifest does (Page, Component, Layout, Action, API, Fragment, StateContract,
WASMContract, Prop, Export, Emit, spans, BackendBinding-as-`Endpoint.Binding`).
So this migration is mostly **re-pointing consumers and deleting converters**,
not designing new models.

## Assumptions

- `gwdkir.Program` stays the stable handoff; new fields are added there first.
- Public manifest **JSON** output (`internal/manifest/json.go`) is a separate
  concern and is explicitly OUT of scope here (architecture doc step 5 keeps it
  until a release plan deprecates it). We migrate the in-memory model coupling,
  not the wire format.
- Behavior must not change; each step is independently shippable and gated by
  the existing test suite.
- No new third-party dependencies.

## The Two Seams That Matter

The data flow has exactly two conversion seams to delete, in this order:

1. **`buildgen.buildModelFromIR(ir)` — `internal/buildgen/ir.go:8`** (the
   IR→manifest reconstruction shim). buildgen's public entrypoint
   `BuildFromIR(config, ir, outputDir)` **already takes IR**, then immediately
   rebuilds a `manifest.Manifest` from it (`buildPageFromIR`,
   `buildComponentFromIR`, `buildLayoutFromIR`, `buildBackendBindingFromIR`)
   solely to call `compiler.ValidateManifest` and
   `compiler.ValidateBackendBindingPolicy` (see `build.go:37,43`). This is the
   exact round-trip the architecture doc warns about and the **highest-leverage
   first kill** — it is lossy, fragile, and serves only validation.

2. **`gwdkanalysis.BuildIR(config, app manifest.Manifest)` —
   `internal/gwdkanalysis/ir_builder.go:14`** (the manifest→IR build). This is
   the backbone of the pipeline. Removing manifest as the *input* to IR is the
   final, largest step and depends on parser emitting AST-first.

## Proposed Changes (sequenced — each step ships independently)

### Step 0 — Guardrails (no behavior change)

- Add a focused golden test that runs the full `cmd/gowdk build` orchestration
  on a representative fixture and snapshots the resulting `gwdkir.Program`
  (JSON-serialized) plus generated output file list. This snapshot is the
  safety net every later step must keep green. Land this BEFORE touching code.

### Step 1 — Make compiler validation IR-native (the keystone)

`compiler` gates every other package, and validation is what forces the
buildgen IR→manifest shim to exist. Provide IR-typed validation entrypoints
that read `gwdkir.Program`:

- `compiler.ValidateProgram(config, ir gwdkir.Program) error` — IR twin of
  `ValidateManifest` (`validate.go:29`).
- `compiler.ValidateBackendBindingPolicyIR(config, ir gwdkir.Program) error` —
  IR twin of `ValidateBackendBindingPolicy` (`backend_binding_policy.go:12`).

Implement these by reading IR fields directly. Where a sub-validator
(`validate_page.go`, `validate_component_*.go`, etc.) reads a manifest field,
add an IR-reading variant or a tiny accessor; keep the manifest versions intact
for now (parallel, not replaced). Internally the manifest validators can be
re-expressed in terms of the IR ones via a thin local adapter to avoid
duplicating logic, but the **public** new surface is IR-typed.

### Step 2 — Delete the buildgen IR→manifest shim

- Switch `buildgen/build.go:37,43` to call the new `ValidateProgram` /
  `ValidateBackendBindingPolicyIR`.
- Delete `buildModelFromIR` and its helpers in `internal/buildgen/ir.go`
  (`buildPageFromIR`, `buildComponentFromIR`, `buildLayoutFromIR`,
  `buildBackendBindingFromIR`, and the ~12 sub-helpers).
- Migrate the remaining buildgen render helpers that still take
  `manifest.Page/Component/Layout` to take the `gwdkir` equivalents. These are
  mechanical field-rename changes since the IR types mirror the manifest ones.
  This is the bulk of buildgen's 731 refs and can be done file-by-file
  (`css.go`, `components.go`, `render.go`, `routes.go`, …), each compiling and
  testing green on its own.

**Exit criterion for Step 2:** `internal/buildgen` no longer imports
`internal/manifest`.

### Step 3 — Make backend binding + endpoint discovery IR-native

- `BindBackendHandlers(app manifest.Manifest) manifest.Manifest`
  (`backend_bindings.go:38`) currently mutates manifest. Move binding discovery
  to populate `gwdkir.Endpoint.Binding` directly (the IR already has the field;
  `attachBackendBindings` in `gwdkanalysis/ir_bindings.go:10` already copies it
  across). Provide `BindBackendHandlersIR(ir *gwdkir.Program)`.
- `DiscoverGoEndpointComments(app) (app, error)` (`go_endpoints.go:18`) — same
  treatment: discover into IR endpoints.
- Update `BuildRouteMetadata` (`route_bindings.go:105`) to take IR (it already
  builds IR internally and discards the manifest afterward, so this mostly
  deletes the manifest plumbing).
- Update the `cmd/gowdk/build.go:89-116` orchestration to call the IR-native
  validate/bind path. After this, the CLI build path no longer needs manifest
  between BuildIR and codegen.

**Exit criterion for Step 3:** `internal/compiler` no longer imports
`internal/manifest` (public JSON aside — confirm `json.go` is the only
remaining manifest consumer module-wide besides the parser seam).

### Step 4 — appgen + lang/lsp cleanup

- `appgen` is already IR-first at its boundary (`Options.IR`,
  `actionEndpointsFromIR`, `apiEndpointsFromIR`). Convert the residual
  manifest-typed helpers (`routes.go`, `types.go`, `source_*.go`,
  `validate_*.go`) to IR types — mechanical, mirrors Step 2.
- `lang` (`sitemap.go`, `accessibility.go`, `tools.go`) and `lsp`
  (`completion_hover.go`, `components.go`, `diagnostics.go`): derive reports and
  symbols from the IR snapshot. LSP open-document source spans stay as-is
  (architecture doc keeps those); only project-wide symbol/report derivation
  moves to IR.

### Step 5 — Collapse the AST→manifest→IR path to AST→IR (the backbone)

Only after Steps 1–4 leave manifest used by just the parser seam + public JSON:

- Add `gwdkanalysis.BuildIRFromAST(config, files []gwdkast.File) gwdkir.Program`
  that lowers typed AST directly to IR (reusing the existing `lowerIRPage` /
  `lowerIRComponent` logic, but sourcing from AST rather than manifest records).
- Repoint the parser/`lang.ParseBuildFiles` path to feed AST into the new
  builder.
- Keep `BuildIR(manifest.Manifest)` and the AST→manifest lowering in
  `manifest_lowering.go` ALIVE but used **only** to produce public manifest
  JSON, isolated behind `internal/manifest/json.go`. This satisfies architecture
  doc step 5 ("keep public manifest JSON compatibility until a release plan
  explicitly deprecates it").

## Files Expected To Change

- New: `internal/compiler/validate_program.go`, `.../backend_bindings_ir.go`,
  `internal/gwdkanalysis/ir_from_ast.go`, plus the Step 0 golden test.
- Heavy edits: `internal/buildgen/*` (delete `ir.go` shim; field renames across
  render helpers), `internal/compiler/route_bindings.go`,
  `backend_bindings.go`, `go_endpoints.go`, `validate*.go`,
  `internal/appgen/*`, `cmd/gowdk/build.go`, `route_report.go`, `dev_loop.go`.
- Light edits: `internal/lang/*`, `internal/lsp/*`.
- Untouched on purpose: `internal/manifest/json.go` (public wire format),
  parser's AST output, `gwdkir/ir.go` types (add fields only if a gap is found).

## Data And API Impact

- No change to public manifest JSON, generated HTML, generated Go, route
  manifests, or build reports. The golden snapshot in Step 0 enforces this.
- Internal-only API surface change: new IR-typed validation/binding entrypoints;
  removal of the buildgen IR→manifest shim. `internal/` packages have no
  external API stability guarantee.

## Tests

- Unit: IR-twin validators must reproduce the exact `ValidationError` set the
  manifest validators produce. Add table tests that run both on the same fixture
  IR/manifest and assert identical error slices during the parallel-existence
  window (Steps 1–4).
- Integration: the Step 0 build-orchestration golden test (IR snapshot +
  output file list) gates every step.
- End-to-end: `scripts/test-go-modules.sh` (root + nested adapter modules).
- Manual: `gowdk build` on `examples/` that have buildable targets; diff
  `dist/` output byte-for-byte against pre-migration output.

## Verification Commands

```sh
# Per-package, after each step:
go build ./... && go vet ./internal/buildgen ./internal/compiler ./internal/appgen
gofmt -l internal cmd

# Confirm a package has shed its manifest dependency (run after its step):
grep -rl 'gowdk/internal/manifest' internal/buildgen --include='*.go' | grep -v _test.go   # expect empty after Step 2
grep -rl 'gowdk/internal/manifest' internal/compiler --include='*.go' | grep -v _test.go   # expect empty after Step 3

# Full gate:
scripts/test-go-modules.sh
```

## Rollback Plan

- Each step is a separate PR that compiles and passes tests on its own. Rolling
  back is reverting that PR; later steps depend on earlier ones but no step
  leaves the tree in a non-building state.
- The parallel-existence strategy (Steps 1, 3 add IR entrypoints beside the
  manifest ones rather than replacing in place) means a regression in an IR
  validator can be reverted to the manifest path without unwinding consumer
  edits.

## Risks

- **Validator drift:** the IR twins must reproduce manifest validation exactly.
  Mitigated by the dual-run table tests during the parallel window (Tests §).
- **Lossy shim removal:** `buildModelFromIR` reconstructs manifest from IR; if
  any buildgen render helper depended on a manifest field NOT present in IR,
  Step 2 surfaces it as a compile error → add the field to `gwdkir` first
  (architecture doc rule: "add fields to IR first"). Audit for this in Step 2.
- **Span/value type sharing:** `gwdkir`, `gwdkast`, and `manifest` share span
  and value types (80 / 35 refs). Decide ownership early — likely move the
  shared value types into a neutral package or `gwdkir` to avoid a residual
  manifest import just for `SourceSpan`.
- **Scope creep into public JSON:** explicitly fenced off. If a reviewer pushes
  to also migrate `json.go`, that is a separate release-gated decision.

## Suggested PR Sequence

1. Step 0 golden test (safety net).
2. Step 1 IR-native compiler validation (parallel, additive).
3. Step 2 delete buildgen shim + buildgen field migration. **Highest payoff.**
4. Step 3 IR-native backend binding/discovery + CLI orchestration.
5. Step 4 appgen + lang/lsp.
6. Step 5 AST→IR backbone; isolate manifest to public JSON only.
