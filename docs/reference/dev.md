# Dev Loop

`gowdk dev` is a dependency-free development loop for GOWDK projects.

## Current Contract

The command:

- forwards build flags to `gowdk build`;
- writes to `gowdk_cache` unless `--out` or one selected build target supplies
  or infers an output directory;
- serves build output for SPA/static development;
- polls explicit or discovered `.gwdk`, CSS, and config inputs;
- compares watched input content hashes and skips no-op rebuild ticks;
- prints changed, added, and removed input paths when a rebuild starts;
- injects a small server-sent-events live-reload script into served HTML;
- shows a browser overlay for rebuild compiler/build failures with diagnostic
  code, source range, last-good build time, and changed-file context when
  available;
- keeps serving the last successful output when rebuilds fail.

When `--app <dir>` or a selected target has `App`, `dev` also builds the
generated app, compiles a local binary, starts it with `GOWDK_ADDR`, and
restarts that process after successful rebuilds. Runtime stdout and stderr stay
attached to the terminal.

## Rebuild Scope

For plain SPA `--out` builds, page, component, and layout edits use the
incremental SPA renderer when the changed files are already in the source set.
The dev loop validates the full compiler IR, derives page/component/layout
reverse dependencies, refreshes manifests, writes affected page output, and
removes stale route output for changed pages.

These changes use the full build path:

- CSS files;
- config files;
- source-set changes;
- target changes;
- generated app, binary, backend, or WASM output.

When build flags include `--timings`, incremental rebuilds update the timings
sidecar with counters for input changes, affected pages, component/layout/page
changes, files written, and identical writes skipped.

The dev loop stores a watched-input snapshot in the output directory. A later
poll tick can reuse that snapshot when the source set and output are still
present, which avoids reloading config and rewalking the tree on no-op ticks.

## HMR

Component-level HMR is not part of the current contract. The P0 baseline is
full-page live reload with last-good-output serving.

Local island state preservation is also not a current contract. Add it only
after GOWDK has a stable component/client dependency graph.

Generated-app runtime overlay delivery, dev-only runtime panic surfacing, and
component-aware HMR are tracked in
[#424](https://github.com/cssbruno/GoWDK/issues/424).

## Browser Overlay

For plain SPA/static dev serving, rebuild compiler/build failures are printed
to the terminal and sent to the browser over the existing live-reload event
stream. The injected script shows a fixed overlay while the last successful
output continues to serve.

The overlay includes, when available:

- diagnostic code, severity, message, source file, and source range;
- last successful build time;
- files that triggered the failed rebuild;
- generated route/endpoint attribution when the failing build report carries
  that metadata.

The overlay is removed on the next successful rebuild and page reload.

Generated app runtime mode keeps runtime stdout/stderr attached to the terminal.
Browser overlay delivery there is limited by the generated app process serving
the HTTP traffic, so generated-app runtime errors remain terminal-first until a
runtime browser bridge exists. See
[#424](https://github.com/cssbruno/GoWDK/issues/424).

## File Watching

The current watcher is portable polling controlled by `--interval`.

Native filesystem watching is deferred until there is a small cross-platform
implementation that does not add fragile dependencies or platform-specific
runtime assumptions.

## Doctor

Use `gowdk doctor` for setup and project-health checks that do not write build
output. It verifies the local Go/GOWDK toolchain, config loading, source
discovery, language validation, route metadata construction, and relevant
optional tools. Use `gowdk doctor --json` for CI or editor integrations.
