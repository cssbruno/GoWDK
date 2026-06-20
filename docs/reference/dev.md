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
restarts that process after successful rebuilds. The generated app runs on an
internal loopback port behind a dev-only proxy at the requested `--addr`, so
HTML responses get the same live-reload and build-error overlay bridge as
plain SPA/static dev serving. Runtime stdout and stderr stay attached to the
terminal.

Terminal startup output uses stable wording for the serving mode:

```text
Static dev server: serving <output-dir> at http://<addr>
Generated app runtime: proxy http://<addr> -> http://<internal-addr> (binary <path>)
```

`<addr>` is the public dev address. `<internal-addr>` is the loopback address
assigned to the generated app process through `GOWDK_ADDR`; it is an
implementation detail for local dev proxying and should not be used in deploy
docs.

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

On each detected change, terminal output starts with stable change-summary
wording:

```text
Change detected at <RFC3339 timestamp>: <n> changed, <n> added, <n> removed
  changed: <path>
  added: <path>
  removed: <path>
```

After a successful rebuild, the loop prints one of:

```text
Dev rebuild complete: static output refreshed at <output-dir>
Dev rebuild complete: generated app restarted: proxy http://<addr> -> http://<internal-addr> (binary <path>)
```

## HMR

`gowdk dev` supports conservative component-aware HMR for generated JavaScript
islands in plain SPA/static serving. When a changed component source maps to the
current page and the browser can find matching `<gowdk-island>` roots, the dev
bridge fetches the fresh document, swaps those island roots, remounts islands,
and emits `gowdk:component-hmr`.

The dev bridge falls back to full-page reload for page changes, layout changes,
source-set changes, generated app/runtime mode, WASM islands, and component
changes that do not have a matching island boundary on the current page. Local
island state preservation is not a current contract.

Generated-app rebuild and runtime 5xx overlay delivery use the dev-only proxy
bridge. Broader state-preserving component HMR remains tracked in
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

Generated app runtime mode keeps runtime stdout/stderr attached to the terminal
and serves browser traffic through the dev-only proxy bridge. Rebuild failures
from generated app compilation are sent to the same browser overlay. Request-time
5xx HTML responses from the generated app trigger a generic runtime overlay with
the HTTP status only; the payload does not include request paths, query strings,
cookies, submitted form values, response bodies, panic values, or stack traces.
The terminal remains the source for redacted runtime panic logs.

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
