# Feature Spec: Module Binary Packaging

## Problem

GOWDK projects can define multiple modules, but deployers need a clear way to
decide which modules are packaged into each generated Go binary. A project may
want one binary that embeds only an admin module, another binary that embeds a
public site plus API module, or a single binary that embeds all selected
modules.

## Goals

- Let SPA build target config define the exact `.gwdk` source set compiled
  into SPA output, generated app source, and generated binaries.
- Support one module per binary, multiple modules per binary, and multiple
  separate binaries from different module selections.
- Keep deployment target orchestration user-owned for now.
- Document config and commands that make binary composition explicit and
  repeatable.

## Non-Goals

- Do not add a deployment orchestrator in this slice.
- Do not infer infrastructure behavior from `ModuleConfig.Type`.
- Do not split one generated binary into independently enabled runtime modules.

## Users And Permissions

- Primary users: Go developers building modular GOWDK apps.
- Roles or permissions: local build users with access to configured source
  modules.
- Data visibility rules: a generated binary embeds only the selected source
  modules and generated assets for that build.

## User Flow

1. The developer declares modules in `gowdk.config.go`.
2. The developer declares `Build.Targets` with names, modules, output dirs,
   generated app dirs, and binary paths.
3. The developer runs `gowdk build` to build all configured targets.
4. The developer runs `gowdk build --target <name>` to build selected targets.

## Requirements

### Functional

- `Build.Targets` may declare one or more SPA build targets.
- `--target` may be repeated or comma-separated.
- Ad hoc `--module` may be repeated or comma-separated.
- When modules are selected, root source discovery is skipped and only selected
  configured module sources are discovered.
- `--app` embeds the selected build output.
- `--bin` compiles a binary from the generated app that embeds the selected
  output.
- Unknown module names fail before generating output.

### Non-Functional

- Performance: module filtering should reuse existing discovery logic.
- Reliability: generated binaries must be runnable and serve only selected
  module routes.
- Accessibility: no direct impact.
- Security/privacy: generated binaries must not accidentally embed unselected
  module pages.
- Observability: docs must describe the module-selection contract clearly.

## Acceptance Criteria

- [x] A build with one selected module produces a binary serving that module and
  not unselected module routes.
- [x] A build with multiple selected modules produces one binary serving all
  selected module routes.
- [x] `gowdk build` runs all configured `Build.Targets`.
- [x] `gowdk build --target <name>` runs only selected configured targets.
- [x] Docs show how to with config configure different binaries from different
  module sets.

## Edge Cases

- Selected modules can define conflicting routes; existing route-conflict
  validation should fail the build.
- Explicit file paths bypass discovery and therefore bypass module selection.
- Separate build targets must use separate output/app directories to avoid stale
  artifacts from previous module selections.

## Dependencies

- Internal: module discovery in `cmd/gowdk`, SPA generation, generated app
  generation, and binary build support.
- External: Go toolchain for `--bin`.

## Open Questions

- Should build targets eventually support per-target addons and render-mode
  overrides?
