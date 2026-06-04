# Feature Spec: Module Config

## Problem

Projects can outgrow a single source glob quickly. A GOWDK app may have more
than one frontend surface plus backend or service-oriented `.gwdk` files, and
the config needs a simple way to name those source groups without changing the
compile-first model.

## Goals

- Let `gowdk.Config` declare named modules such as `frontend`, `frontend2`,
  `backend`, and `backendmicroservice`.
- Let `gowdk build` discover `.gwdk` files from each configured module when no
  explicit file list is passed.
- Let users select one or more modules at build time for their own deployment
  code.
- Keep module config statically parseable from `gowdk.config.go`.
- Keep root `Source` support working for single-module projects.

## Non-Goals

- Generating separate binaries or separate output directories per module.
- Enforcing backend/frontend behavior from module type in this slice.
- Generating Kubernetes manifests or owning deployment configuration.
- Executing user config code.
- Inferring routes from module folders.

## Users And Permissions

- Primary users: Go developers organizing a GOWDK app into multiple source
  groups.
- Roles or permissions: local read access to configured source folders and write
  access to the selected build output.
- Data visibility rules: module parse errors must not print full config source.

## User Flow

1. A user declares modules in `gowdk.config.go`.
2. The user runs `gowdk build` without explicit files.
3. The CLI discovers root source patterns plus module source patterns and emits
   one static build output.

## Requirements

### Functional

- `Config.Modules` declares named module entries.
- `ModuleConfig.Name` identifies the source group.
- `ModuleConfig.Type` can describe any user-defined intent such as `frontend`,
  `backendmicroservice`, `worker`, or another project-specific module role.
- `ModuleConfig.Source.Include` and `ModuleConfig.Source.Exclude` use the same
  glob semantics as root `Source`.
- If a module has a name but no include patterns, discovery uses
  `<module-name>/**/*.gwdk`.
- If any root or module include is configured, default `**/*.gwdk` discovery is
  not added.
- Root source excludes and module source excludes are both honored.
- `gowdk build --module <name>` limits discovery to selected configured modules.
- `--module` may be repeated or comma-separated.
- Explicit file arguments still bypass discovery.

### Non-Functional

- Performance: module config loading remains a single Go parser pass.
- Reliability: config loading remains side-effect-free.
- Accessibility: no UI impact.
- Security/privacy: no user Go code is executed while reading modules.
- Observability: docs must state that module type is metadata-only today.

## Acceptance Criteria

- [x] `gowdk.config.go` can parse literal `Modules` entries.
- [x] `gowdk build` discovers files from a name-only module source root.
- [x] `gowdk build` discovers files from explicit module include globs.
- [x] Module excludes prevent matching `.gwdk` files from being built.
- [x] Unconfigured folders are not discovered when modules define the source set.
- [x] `gowdk build --module <name>` builds only selected configured modules.

## Edge Cases

- Empty module entries are ignored by the static config parser.
- Non-literal module values are ignored in the current config subset.
- Module type is parsed for future codegen but does not change build behavior
  yet.
- Module selection is a build/discovery feature; deployment remains user-owned
  code.

## Dependencies

- Internal: `cmd/gowdk`, `internal/project`, `internal/discover`.
- External: Go standard library only.

## Open Questions

- Should future builds support per-module outputs or a module selection flag?
- Should backend and service modules eventually generate different handler
  packages or binaries?
