# Feature Spec: Fast Dev Reload

## Problem

Developers need a tight local development loop similar to Vite, SvelteKit, or
Svelte: edit source, rebuild quickly, and see the served app refresh without
manually running separate build and serve commands. GOWDK should own this local
loop.

## Goals

- Keep the dependency-free `gowdk dev` polling loop portable.
- Run an initial build, serve the output directory, and reload the browser after
  successful rebuilds.
- Support ad hoc `--out` builds and configured `Build.Targets` with `Output`.
- Keep the previous served output available when a rebuild fails.
- Avoid false rebuild/reload churn from timestamp-only touches or generated
  files whose bytes did not change.
- Regenerate only changed SPA page outputs for the first safe incremental
  dev slice.

## Non-Goals

- Do not add browser HMR, websocket live reload, or partial DOM patching in this
  slice.
- Do not add a filesystem notification dependency.
- Do not manage generated binary processes from this command.

## Users And Permissions

- Primary users: developers running GOWDK locally.
- Roles or permissions: local shell user able to build and execute generated
  binaries.
- Data visibility rules: no new data exposure; child process inherits the local
  environment like a manually started binary.

## User Flow

1. The developer runs `gowdk dev --target admin` for a configured target with
   `Output`, or `gowdk dev --out gowdk_cache <files...>` for ad hoc inputs.
2. GOWDK performs an initial build.
3. GOWDK serves the output directory.
4. On source changes, GOWDK rebuilds and sends a browser reload only when the
   rebuild succeeds.

## Requirements

### Functional

- `dev` runs an initial build, serves the output directory, and polls inputs.
- `dev` defaults ad hoc output to `gowdk_cache` when no output is provided.
- `dev --target <name>` uses the selected target `Output`.
- `dev --target` rejects ambiguous or output-less target selections.
- Dev input snapshots compare content hashes, not modification times.
- SPA and generated app output writes are skipped when the existing file
  already contains the desired bytes.
- Plain `gowdk dev --out <dir>` uses incremental SPA rendering when every
  changed input is an existing page source file.
- Incremental SPA rendering refreshes route and asset manifests and removes
  stale route output for changed pages.
- Config, source-set, component, layout, CSS, generated app, binary, and target
  builds fall back to the full build path.

### Non-Functional

- Performance: reuse existing input discovery, compare content hashes to avoid
  timestamp-only rebuilds, preserve unchanged output file modification times,
  and render only changed SPA pages when the change is page-local.
- Reliability: failed rebuilds must keep serving the previous output.
- Accessibility: no direct impact.
- Security/privacy: do not execute arbitrary shell strings.
- Observability: print rebuild and browser reload events.

## Acceptance Criteria

- [x] `gowdk dev --out <dir> <files...>` accepts ad hoc builds.
- [x] `gowdk dev --target <name>` accepts one configured target with output.
- [x] `gowdk dev` defaults output to `gowdk_cache` for ad hoc inputs.
- [x] `gowdk dev --target` rejects multiple configured output targets unless a
  single target is selected.
- [x] Touching or rewriting a dev input file with identical content does not
  change the input snapshot.
- [x] Re-running SPA/app generation with identical output preserves
  generated file modification times.
- [x] Plain SPA dev can handle changed page sources through the
  incremental SPA renderer.
- [x] Component changes and other structural changes fall back to full builds.

## Edge Cases

- If the dev address is already in use, startup reports the server error.
- If the output directory is missing, the initial build creates it before the
  server starts.
- If a SPA source file is removed from output, generated app sync removes the
  stale embedded SPA file even while preserving unchanged files.
- If a changed SPA page route moves, incremental SPA rendering removes
  the old route output using the previous route manifest.

## Dependencies

- Internal: `cmd/gowdk` dev/build pipeline.
- External: local Go toolchain only when the forwarded build command compiles a
  binary.

## Open Questions

- Should a future `gowdk dev` command combine SPA serving, generated app
  serving, and browser reload into one UI-focused command?
