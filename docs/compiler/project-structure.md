# Project Structure

## Current Inputs

Project-level CLI commands require `gowdk.config.go` in the current directory,
or an explicit config passed with `--config <file>`, before they compile,
validate, or inspect `.gwdk` code. Explicit `.gwdk` file paths narrow the input
set, but they do not bypass the config requirement:

```sh
go run ./cmd/gowdk check examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

`gowdk init [dir]` scaffolds a conventional buildable project:

```text
gowdk.config.go
.gitignore
src/pages/home.page.gwdk
src/components/hero.cmp.gwdk
styles/global.css
```

The generated config uses `src/**/*.gwdk` as the source include,
`styles/**/*.css` for CSS inputs, a `site` build target, `.gowdk/site` for
generated app source, and `bin/site` for the generated binary. The `site`
target's output is inferred as `.gowdk/output/site`. The scaffolded
`.gitignore` ignores generated output directories.

`gowdk build` can also discover `.gwdk` files when no explicit files are
supplied. It reads literal `Source.Include`, `Source.Exclude`, and
`Modules`, and `Build.Output` fields from the required config. Root source
patterns and module source patterns are additive when no module is selected; a
name-only module defaults to `<module-name>/**/*.gwdk`.
`Build.Targets` can declare selected modules, optional output dirs, generated
app dirs, and binary paths for user-owned deployment workflows. Target `Output`
defaults to `.gowdk/output/<target-name>` when omitted. With targets configured,
`gowdk build` runs all targets and `gowdk build --target <name>` runs selected
targets. `gowdk build --module <name>` remains available for ad hoc builds, and
the flag may be repeated or comma-separated. The selected modules define what
gets emitted to `--out`, copied into `--app`, and embedded into `--bin`. When
the loaded config has no root or module include, discovery falls back to
`**/*.gwdk` and an explicit `--out` directory.

`.gwdk` files are selected by source discovery, explicit CLI paths, or selected
modules. Go `import` declarations inside `.gwdk` files import normal Go
packages for typed contracts and build functions; they do not import `.gwdk`
components, layouts, or pages. Same-package `.gwdk` and `.go` files are peers,
except `gowdk.config.go`, which is project configuration rather than
application package code.

Cross-package GOWDK source imports use `use`, not Go `import`:

```gwdk
package pages

use ui "components"

view {
  <main><ui.Hero title="GOWDK" /></main>
}
```

The quoted `use` target is a discovered `.gwdk` package name. Pages and
components can use qualified component calls through their own scoped aliases.
Pages use qualified layout references with `@layout alias.id`, components use
qualified stores from client blocks with `use alias.store`, and pages select
cross-package CSS assets with `@css alias.name`. Bare store and asset names are
same-package selections or built-in selections; cross-package lookup is never
implicit.

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
- Whether build targets need per-target addons, render settings, or package
  layout controls.
- How examples and fixture apps are kept runnable.
- Component-level scoped CSS and asset files.

Routes and layouts must remain declared inside files, not inferred from folder location.
