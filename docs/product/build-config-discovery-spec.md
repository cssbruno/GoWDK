# Feature Spec: Build Config Discovery

## Problem

`gowdk build` can discover files with default include and exclude patterns, but
real projects need a repeatable way to choose source globs and output directory.
The public config type already models `Source.Include`, `Source.Exclude`, and
`Build.Output`, and architecture examples show `gowdk.config.go`, but the CLI
does not read that file yet.

## Goals

- Let `gowdk build` load a local `gowdk.config.go` when it exists.
- Support static extraction of literal `gowdk.Config` fields needed by the build
  command: `Source.Include`, `Source.Exclude`, and `Build.Output`.
- Keep config loading side-effect-free by parsing Go source instead of executing
  user code.
- Let explicit CLI flags and explicit file arguments keep precedence over config
  defaults.

## Non-Goals

- Compiling or executing `gowdk.config.go`.
- Loading addons, render defaults, asset modes, or arbitrary Go expressions.
- Adding config-aware `check`, `manifest`, `sitemap`, or LSP diagnostics in this
  slice.
- Discovering config files outside the current working directory.

## Users And Permissions

- Primary users: Go developers running `gowdk build` in a project root.
- Roles or permissions: local read access to `gowdk.config.go` and configured
  source files, plus write access to the output directory.
- Data visibility rules: config parse errors must not print source file contents.

## User Flow

1. A user creates `gowdk.config.go` with literal source include/exclude globs and
   build output.
2. The user runs `gowdk build` or `gowdk build --out dist`.
3. The CLI loads config, discovers source files from configured globs when no
   explicit file list is passed, and emits static output.

## Requirements

### Functional

- `gowdk build` loads `gowdk.config.go` from the current working directory when
  present.
- `gowdk build --config <path>` loads the requested config path.
- Missing default config is ignored; missing explicit config fails.
- Config `Source.Include` overrides the default include patterns when non-empty.
- Config `Source.Exclude` is appended to default excludes.
- `--out` overrides `Build.Output`.
- If neither `--out` nor `Build.Output` is set, `gowdk build` still fails with a
  usage error.
- Literal string list values in `[]string{...}` and `[]string{}` are supported.

### Non-Functional

- Performance: config loading is a single Go parser pass.
- Reliability: no user code is executed while reading config.
- Accessibility: no UI impact.
- Security/privacy: diagnostics name fields and files but do not echo full config
  source.
- Observability: docs must state the supported subset.

## Acceptance Criteria

- [x] A temp project with `gowdk.config.go` can run `gowdk build` without
  explicit files or `--out`.
- [x] `--out` overrides `Build.Output`.
- [x] `--config <path>` can load a non-default config file.
- [x] Configured excludes prevent matching `.gwdk` files from being built.
- [x] Invalid config syntax returns a clear build error.

## Edge Cases

- Non-literal config values are ignored for this slice and should not be
  mistaken for executed config.
- Multiple `gowdk.Config` variables use the first literal assigned to `Config`.
- Output directories under the project root are still excluded from discovery.

## Dependencies

- Internal: `cmd/gowdk`, `internal/project`, `internal/discover`.
- External: Go standard library only.

## Open Questions

- Should config loading eventually compile a user Go package or stay static and
  declarative?
- How should addons and render defaults be represented safely for CLI tools?
