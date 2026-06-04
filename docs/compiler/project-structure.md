# Project Structure

## Current Inputs

Most current CLI commands take explicit `.gwdk` file paths:

```sh
go run ./cmd/gowdk check examples/basic/*.gwdk
```

`gowdk init [dir]` scaffolds a conventional buildable project:

```text
gowdk.config.go
src/pages/home.page.gwdk
src/components/hero.cmp.gwdk
styles/global.css
```

The generated config uses `src/**/*.gwdk` as the source include, `styles/**/*.css`
for CSS inputs, and `dist/site` for build output.

`gowdk build` can also discover `.gwdk` files when no explicit files are
supplied. It reads literal `Source.Include`, `Source.Exclude`, and
`Modules`, and `Build.Output` fields from `gowdk.config.go` when present. Root
source patterns and module source patterns are additive; a name-only module
defaults to `<module-name>/**/*.gwdk`. `gowdk build --module <name>` limits
discovery to selected configured modules for user-owned deployment workflows.
When no root or module include is configured, discovery falls back to
`**/*.gwdk` and an explicit `--out` directory.

Current file-kind classification treats files ending in `.cmp.gwdk` or
containing `@component` as components, files ending in `.layout.gwdk` as layout
files, files ending in `.asset.gwdk` as asset-adjacent planning files, files
ending in `.plugin.gwdk` as plugin-adjacent planning files, and other `.gwdk`
inputs as pages. Layout, asset, and plugin-adjacent files are classified so
project discovery can accept future conventions, but their parsing, resolution,
and rendering remain planned.

## Planned Source Layout Decisions

Future compiler work must define:

- Final default source directories.
- How island files are classified.
- Where user Go code lives.
- How full app config is discovered or passed to every compiler command.
- Whether module types map to separate generated packages, binaries, or output
  directories.
- How examples and fixture apps are kept runnable.

Routes and layouts must remain declared inside files, not inferred from folder location.
