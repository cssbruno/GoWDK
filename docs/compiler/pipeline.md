# Compiler Pipeline

## Current Pipeline

```text
project config plus explicit file paths or configured discovery
  -> parse source files into typed GOWDK AST
  -> lower parsed records into internal/gwdkir.Program
  -> enrich IR with Go endpoint discovery and backend bindings
  -> validate IR invariants, render rules, routes, endpoints, packages, and assets
     into a compiler-owned ValidatedProgram phase token
  -> emit diagnostics, public manifest JSON, site-map JSON, route/endpoint metadata, build-output artifacts, browser runtime assets, or generated app output
```

Project-level compiler commands require `gowdk.config.go` or `--config <file>`.
The current CLI accepts explicit `.gwdk` files, but explicit paths still require
a loaded config. `gowdk build` can also discover source files from
`Source.Include` and `Source.Exclude` settings plus configured module sources
when no explicit files are supplied. Configured `Build.Targets` can declare
selected modules, output dirs, generated app dirs, binary paths, backend-only
outputs, and contract worker/cron role outputs; `gowdk build` runs all
configured targets and `gowdk build --target <name>` runs selected targets.
`gowdk build --module <name>` remains available for ad hoc
builds, and the flag may be repeated or comma-separated. Discovery uses
`**/*.gwdk` defaults when no root/module source is configured in the loaded
config.

`gowdk build [--config <file>] [--project-root <dir>] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--worker-app <dir>] [--worker-bin <file>] [--cron-app <dir>] [--cron-bin <file>] [files...]` currently
emits app-shell HTML, `gowdk-routes.json`, `gowdk-assets.json`, generated
embedded app source, optional web/backend binaries, and optional contract
worker/cron binaries for the selected source set. The current rendered page
subset covers simple `spa` and
`action` pages with non-dynamic routes or literal `paths {}` dynamic routes,
literal `build {}` data, imported or same-package Go build data functions with
optional `gowdk.BuildParams`, lowercase HTML markup in `view {}`, and
`.cmp.gwdk` component files.

`internal/syntax` owns the shared tokenizer. `internal/parser.ParseSyntax` uses
that token stream to recover a typed `internal/gwdkast.File` for the current
source subset: package declarations, typed page/component/layout/route/render/layout/guard/CSS
declarations, metadata declarations, Go imports, GOWDK uses, stores, typed
component contracts, supported top-level blocks, parsed `view {}` markup nodes,
literal `paths {}`/`build {}` records, action/API endpoint declarations, and
source spans. Parser diagnostics accumulate across declaration and block
boundaries so one syntax error does not hide later declarations in the same
file.

`internal/gwdkanalysis` assembles parsed page, component, and layout records
into `internal/gwdkir.Program`. `internal/compiler.AnalyzedProgram` wraps that
IR after enrichment, and `internal/compiler.ValidatedProgram` is the opaque
phase token returned only by compiler validation. Build-output fast paths accept
`ValidatedProgram`; raw `gwdkir.Program` entrypoints must run compiler
validation before emission. Static build output resolves page artifacts, CSS,
assets, manifests, and report metadata into `internal/buildgen.BuildPlan`
before writing files. Generated app output resolves auto routes, endpoint
projections, SSR artifacts, sitemap data, and generator-local validation into
`internal/appgen.ApplicationPlan` before writing files.

The IR models packages, source files, page routes, backend endpoints from
`.gwdk` declarations or explicit Go comments, templates, client behavior,
source-selected assets, asset scope/hash metadata, parsed view nodes, typed
literal records, and imported build-data call metadata.

`gwdkir.Blocks` retains source bodies for spans, formatting, inspection, and
compatibility only. Parser/analyzer lowering must populate parsed `view {}`
nodes, ordered literal `paths {}`/`build {}` records, and imported build-data
call metadata before validation. Downstream validation and generated-output
planning consume those typed fields; raw block bodies are not a semantic fallback
after lowering. `gwdkir.CheckInvariants` rejects non-empty supported `view {}`
bodies that reach the IR without parsed view nodes.

Build, memory build, incremental SPA build, SSR artifact, generated app
planning, route reports, LSP metadata, and the public `gowdk manifest` report
consume `internal/gwdkir.Program` and own their output planning separately.
Route and asset manifests are generated output artifacts, not compiler handoff
records.
Generated app Go, backend adapter Go, build-data helper Go, and starter config
Go are constructed as Go ASTs, printed, and formatted before use or write.

Browser-facing output is generated only when the source requires it. Partial
form metadata can emit `assets/gowdk/gowdk.js`; stateful components can emit
generated JavaScript island assets; component-level `wasm` declarations can
emit WASM island loader assets. See `browser-compiler.md`.

## Invalid IR Policy

`internal/gwdkir.CheckInvariants` is the compiler's internal sanity check for
IR defects that should be impossible after parsing and analysis. It checks
structural contracts such as sorted slices, closed enum values, and references
between IR slices. It does not replace user-facing validation for authoring
errors such as duplicate routes, missing guards, or unsupported source syntax.

Public compiler, CLI, buildgen, and appgen boundaries must return diagnostics or
ordinary errors for invalid IR. They must not panic on malformed IR, because a
bad handoff should be reported as an internal compiler error with any available
build report or source context.

Panicking helpers are allowed only in `_test.go` files, must include `ForTest`
or `must` in the helper name, and must not be imported by runtime, CLI, or
generated-output packages. Use those helpers only to keep focused tests small
after the non-panicking boundary behavior is already pinned.

## Target Pipeline

```text
project config
  -> discover sources
  -> lex/parse full AST
  -> semantic analysis and type checks
  -> stable internal IR
  -> app/component/action/API/fragment/SSR codegen
  -> go/format
  -> app assets and generated Go app
  -> optional embedded one-binary output
```

Future build work should expand from the current generated-output slice while
keeping downstream passes on `internal/gwdkir.Program`.

The `lex/parse full AST` front-end is the shared-token parser behind the stable
`internal/gwdkast` AST seam. The decision record for that cutover is
`docs/engineering/decisions/0010-tokenizer-recursive-descent-parser.md`.
