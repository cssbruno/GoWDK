# Compiler Pipeline

## Current Pipeline

```text
explicit file paths or default build discovery
  -> parse source files
  -> build manifest
  -> validate render rules
  -> emit diagnostics, manifest JSON, site-map JSON, route-binding plans, simple static HTML files, or generated static app output
```

The current CLI accepts explicit `.gwdk` files. `gowdk build` can also discover
source files from literal `gowdk.config.go` `Source.Include` and
`Source.Exclude` settings plus configured module sources when no explicit files
are supplied. `gowdk build --module <name>` limits discovery to selected
configured modules, or discovery uses `**/*.gwdk` defaults when no root/module
source is configured.

`gowdk build [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]` currently
emits static HTML, `gowdk-routes.json`, `gowdk-assets.json`, generated embedded
static app source, and an optional static-serving binary for simple `static` and
`action` pages with non-dynamic routes or literal `paths {}` dynamic routes,
literal `build {}` data, lowercase HTML markup in `view {}`, and `.cmp.gwdk`
component files.

`internal/parser.ParseSyntax` exposes a typed AST for the current source subset:
annotations, supported top-level blocks, parsed `view {}` markup nodes, literal
`paths {}`/`build {}` records, action statements, API route statements, and
source spans. Existing manifest parsing still drives the CLI while compiler
passes migrate toward that AST.

## Target Pipeline

```text
project config
  -> discover sources
  -> lex/parse full AST
  -> semantic analysis and type checks
  -> manifest
  -> static/component/action/API/fragment/SSR codegen
  -> static assets and generated Go app
  -> optional embedded one-binary output
```

Future build work should expand from the current simple static artifact to full component, asset, handler, and one-binary output.
