# Dev Loop

`gowdk dev` is a dependency-free development loop for GOWDK projects.

## Current Contract

The command:

- forwards build flags to `gowdk build`;
- writes to `gowdk_cache` unless `--out` or one selected build target supplies
  an output directory;
- serves build output for SPA/static development;
- polls explicit or discovered `.gwdk`, CSS, and config inputs;
- prints changed, added, and removed input paths when a rebuild starts;
- injects a small server-sent-events live-reload script into served HTML;
- keeps serving the last successful output when rebuilds fail.

When `--app <dir>` or a selected target has `App`, `dev` also builds the
generated app, compiles a local binary, starts it with `GOWDK_ADDR`, and
restarts that process after successful rebuilds. Runtime stdout and stderr stay
attached to the terminal.

## Rebuild Scope

For plain SPA `--out` builds, page-only edits use the incremental SPA renderer:
the dev loop validates the full manifest, refreshes manifests, writes changed
page output, and removes stale route output for changed pages.

These changes use the full build path:

- component files;
- layout files;
- CSS files;
- config files;
- source-set changes;
- target changes;
- generated app, binary, backend, or WASM output.

## HMR

Component-level HMR is not part of the current contract. The P0 baseline is
full-page live reload with last-good-output serving.

Local island state preservation is also not a current contract. Add it only
after GOWDK has a stable component/client dependency graph and browser overlay.

## Browser Overlay

Browser error overlay is planned and not implemented today. Compiler and build
failures are printed to the terminal. The last successful output continues to
serve while the error is fixed.

## File Watching

The current watcher is portable polling controlled by `--interval`.

Native filesystem watching is deferred until there is a small cross-platform
implementation that does not add fragile dependencies or platform-specific
runtime assumptions.

## Doctor

There is no `gowdk doctor` command today. Add it only when repeated setup
failures show a stable checklist worth automating.
