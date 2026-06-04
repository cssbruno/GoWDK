# Feature Spec: Build Discovery And Static Route Manifest

## Problem

Early `gowdk build` only works when every page and component file is passed on
the command line. That is enough for smoke tests, but it does not match the
portable file model where a project can be compiled from discovered `.gwdk`
sources. Duplicate page and component identities are also easy to miss until
later output overwrites or component resolution becomes ambiguous.

## Goals

- Let `gowdk build --out <dir>` discover `.gwdk` files from the current project
  directory when no explicit files are supplied.
- Use the existing `internal/discover` include/exclude behavior instead of a
  separate CLI-only scanner.
- Fail fast for duplicate page IDs and duplicate component names.
- Emit a stable JSON route manifest for generated static page artifacts.

## Non-Goals

- Loading `gowdk.config.go` from disk.
- Executing `paths {}` or `build {}` bodies.
- Generating action, API, partial fragment, SSR, CSS, asset, embed, or serving
  binary output.
- Inferring routes from file paths.

## Users And Permissions

- Primary users: Go developers compiling a local GOWDK project.
- Roles or permissions: local filesystem read access to source files and write
  access to the selected output directory.
- Data visibility rules: route manifests contain page IDs, routes, and output
  paths only; they must not include source bodies or secrets.

## User Flow

1. A user runs `gowdk build --out dist` in a directory containing `.gwdk` files.
2. The CLI discovers source files using default include/exclude patterns.
3. The compiler parses pages and components, validates duplicate identities and
   render rules, and emits static HTML plus a route manifest.

## Requirements

### Functional

- With explicit paths, `gowdk build --out <dir> <files...>` keeps using those
  paths and does not run default discovery.
- Without explicit paths, `gowdk build --out <dir>` discovers `**/*.gwdk` under
  the current working directory.
- Default discovery excludes `.git`, `vendor`, `node_modules`, and the selected
  output directory when it is under the current working directory.
- Duplicate page IDs produce compiler diagnostics.
- Duplicate component names produce compiler diagnostics.
- Static output includes `gowdk-routes.json` with schema version, page ID, route,
  and generated relative path for each emitted static page.

### Non-Functional

- Performance: discovery remains a single sorted filesystem walk.
- Reliability: duplicate IDs and render-mode errors prevent output generation.
- Accessibility: no UI impact.
- Security/privacy: route manifest data is limited to public routing metadata.
- Observability: generated route manifest makes static output inspectable.

## Acceptance Criteria

- [x] `gowdk build --out <dir>` succeeds in a temp project with a page and
  component file and no explicit file arguments.
- [x] Discovered build input ignores `.gwdk` files inside the output directory.
- [x] Duplicate page IDs fail validation.
- [x] Duplicate component names fail validation.
- [x] Static builds write `gowdk-routes.json` with stable relative output paths.

## Edge Cases

- No discovered files should fail with a clear error.
- Output directories outside the project root are not added to default excludes.
- Component-only input can produce an empty route manifest until page validation
  grows stricter.

## Dependencies

- Internal: `internal/discover`, `internal/lang`, `internal/compiler`,
  `internal/staticgen`.
- External: none.

## Open Questions

- What final project config loading model should populate `gowdk.Config.Source`?
- Should route manifest naming move under a reserved generated-output directory
  once asset and one-binary output are implemented?
