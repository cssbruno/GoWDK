# Feature Spec: Fast Watch Redeploy

## Problem

Developers need a tight local development loop similar to Vite, SvelteKit, or
Air: edit source, rebuild quickly, and see the running generated app refresh
without manually killing and restarting a binary. GOWDK should own this behavior
instead of depending on Air.

## Goals

- Extend the existing dependency-free `gowdk watch` loop to restart one
  generated binary after successful rebuilds.
- Keep the current polling watcher so the implementation remains portable and
  dependency-free.
- Support ad hoc `--bin` builds and configured `Build.Targets` with a `Binary`.
- Keep a previously running process alive when a rebuild fails.
- Avoid false rebuild/restart churn from timestamp-only touches or generated
  files whose bytes did not change.
- Regenerate only changed static page outputs for the first safe incremental
  watch slice.

## Non-Goals

- Do not add browser HMR, websocket live reload, or partial DOM patching in this
  slice.
- Do not add a filesystem notification dependency.
- Do not restart multiple binaries from one watch command yet.

## Users And Permissions

- Primary users: developers running GOWDK locally.
- Roles or permissions: local shell user able to build and execute generated
  binaries.
- Data visibility rules: no new data exposure; child process inherits the local
  environment like a manually started binary.

## User Flow

1. The developer runs `gowdk watch --restart --target admin` for a configured
   target with `Binary`.
2. GOWDK performs an initial build.
3. After a successful build, GOWDK starts the generated binary.
4. On source changes, GOWDK rebuilds and restarts the binary only when the
   rebuild succeeds.

## Requirements

### Functional

- `watch --restart` restarts one binary after successful builds.
- `watch --restart` can infer the binary from ad hoc `--bin <file>`.
- `watch --restart` can infer the binary from exactly one configured build
  target with `Binary`.
- `watch --restart` rejects `--once`.
- `watch --restart` rejects ambiguous configured target sets.
- Watched input snapshots compare content hashes, not modification times.
- Static and generated app output writes are skipped when the existing file
  already contains the desired bytes.
- Plain `gowdk watch --out <dir>` uses incremental static rendering when every
  changed input is an existing page source file.
- Incremental static rendering refreshes route and asset manifests and removes
  stale route output for changed pages.
- Config, source-set, component, layout, CSS, generated app, binary, target, and
  restart builds fall back to the full build path.

### Non-Functional

- Performance: reuse existing input discovery, compare content hashes to avoid
  timestamp-only rebuilds, preserve unchanged output file modification times,
  and render only changed static pages when the change is page-local.
- Reliability: failed rebuilds must not stop the currently running binary.
- Accessibility: no direct impact.
- Security/privacy: do not execute arbitrary shell strings; execute a concrete
  binary path directly.
- Observability: print rebuild and restart events.

## Acceptance Criteria

- [x] `gowdk watch --restart --out <dir> --app <dir> --bin <file> <files...>`
  accepts ad hoc binary builds.
- [x] `gowdk watch --restart --target <name>` accepts one configured target
  with a binary.
- [x] `gowdk watch --restart` rejects multiple configured binary targets unless
  a single target is selected.
- [x] Existing `watch --once` and plain rebuild behavior remain supported.
- [x] Touching or rewriting a watched file with identical content does not
  change the input snapshot.
- [x] Re-running static/app generation with identical output preserves
  generated file modification times.
- [x] Plain static watch can handle changed page sources through the
  incremental static renderer.
- [x] Component changes and other structural changes fall back to full builds.

## Edge Cases

- If the generated binary exits on its own, the next successful rebuild starts a
  new process.
- If stopping the old process takes too long, GOWDK kills it before starting the
  new one.
- If the binary path is missing after build, the restart step reports the error.
- If a static source file is removed from output, generated app sync removes the
  stale embedded static file even while preserving unchanged files.
- If a changed static page route moves, incremental static rendering removes
  the old route output using the previous route manifest.

## Dependencies

- Internal: `cmd/gowdk` watch/build pipeline.
- External: local Go toolchain only when the build command compiles a binary.

## Open Questions

- Should a future `gowdk dev` command combine static serving, generated app
  serving, and browser reload into one UI-focused command?
