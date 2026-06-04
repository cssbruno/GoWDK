# Feature Spec: Static Asset Manifest

## Problem

`gowdk build` can now write CSS assets returned by compile-time CSS processors,
but generated output has no asset manifest that future embedded/static servers
can load. Route output is already described by `gowdk-routes.json`; CSS assets
need the same kind of stable machine-readable build output.

## Goals

- Emit a generated asset manifest during static builds.
- Record CSS assets emitted by CSS processors.
- Keep asset paths relative to the selected output directory and slash-separated.
- Reuse the runtime asset manifest shape so generated servers can resolve assets
  without knowing compiler internals.

## Non-Goals

- Discover arbitrary user asset files.
- Hash, fingerprint, minify, or rewrite CSS asset names.
- Generate an embedded filesystem or serving binary.
- Add Tailwind or any concrete CSS tool to core.

## Users And Permissions

- Primary users: Go developers running `gowdk build`.
- Roles or permissions: no special permissions beyond write access to the output
  directory.
- Data visibility rules: generated manifests must not expose source file paths or
  private local paths.

## User Flow

1. A CSS processor returns a CSS asset such as `assets/app.css`.
2. `gowdk build` writes the CSS asset under the output directory.
3. `gowdk build` writes `gowdk-assets.json` with a logical-to-emitted path entry
   for that CSS file.
4. Future generated servers can load the manifest and resolve the asset path.

## Requirements

### Functional

- Static builds write `gowdk-assets.json` at the output root.
- The manifest includes `version: 1`.
- The manifest includes a `files` map from logical asset name to emitted path.
- CSS processor assets are recorded with relative slash-separated paths.
- The manifest is deterministic regardless of processor return order.

### Non-Functional

- Performance: manifest generation is linear in emitted CSS asset count.
- Reliability: unsafe CSS asset paths remain rejected before any files are
  written.
- Accessibility: no impact.
- Security/privacy: manifest entries must not include absolute paths or source
  paths.
- Observability: CLI output prints the generated asset manifest path.

## Acceptance Criteria

- [x] A build with a CSS processor writes the CSS file and records it in
  `gowdk-assets.json`.
- [x] The generated asset manifest uses relative slash-separated paths.
- [x] The build result exposes the generated asset manifest path.
- [x] Builds with invalid CSS asset paths still produce no partial output.
- [x] Docs describe the generated asset manifest.

## Edge Cases

- A build without CSS assets still writes an empty `files` map so downstream
  tooling can rely on the manifest path.
- Duplicate CSS asset output paths remain invalid before manifest generation.

## Dependencies

- Internal: `internal/staticgen`, `runtime/asset`, `cmd/gowdk`.
- External: none.

## Open Questions

- Should future non-CSS assets use logical names from config, source paths, or
  emitted output paths?
- Should hashing be core behavior or delegated to asset/CSS plugins?
