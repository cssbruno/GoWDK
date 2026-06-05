# Compiler Pipeline

## Current Pipeline

```text
project config plus explicit file paths or configured discovery
  -> parse source files
  -> build manifest
  -> validate render rules
  -> emit diagnostics, manifest JSON, site-map JSON, route-binding plans, simple app-shell HTML files, browser runtime assets, or generated app output
```

Project-level compiler commands require `gowdk.config.go` or `--config <file>`.
The current CLI accepts explicit `.gwdk` files, but explicit paths still require
a loaded config. `gowdk build` can also discover source files from literal
`Source.Include` and `Source.Exclude` settings plus configured module sources
when no explicit files are supplied. Configured `Build.Targets` can declare
selected modules, output dirs, generated app dirs, and binary paths; `gowdk
build` runs all configured targets and `gowdk build --target <name>` runs
selected targets. `gowdk build --module <name>` remains available for ad hoc
builds, and the flag may be repeated or comma-separated. Discovery uses
`**/*.gwdk` defaults when no root/module source is configured in the loaded
config.

`gowdk build [--config <file>] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]` currently
emits app-shell HTML, `gowdk-routes.json`, `gowdk-assets.json`, generated embedded
app source, and an optional binary for the selected
source set. The current rendered page subset covers simple `spa` and
`action` pages with non-dynamic routes or literal `paths {}` dynamic routes,
literal `build {}` data, imported Go build data functions, lowercase HTML
markup in `view {}`, and `.cmp.gwdk` component files.

`internal/parser.ParseSyntax` exposes a typed AST for the current source subset:
annotations, supported top-level blocks, parsed `view {}` markup nodes, literal
`paths {}`/`build {}` records, action statements, API route statements, and
source spans. Existing manifest parsing still drives the CLI while compiler
passes migrate toward that AST.

Browser-facing output is generated only when the source requires it. Partial
form metadata can emit `assets/gowdk/gowdk.js`; stateful components can emit
generated JavaScript island assets; explicit `g:island="wasm"` component calls
can emit WASM island loader assets. See `browser-compiler.md`.

## Target Pipeline

```text
project config
  -> discover sources
  -> lex/parse full AST
  -> semantic analysis and type checks
  -> manifest
  -> app/component/action/API/fragment/SSR codegen
  -> app assets and generated Go app
  -> optional embedded one-binary output
```

Future build work should expand from the current simple app artifact to full component, asset, handler, and one-binary output.
