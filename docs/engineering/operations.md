# Operations

## Current Status

The current repository provides compiler scaffolding, language tooling, static
output, local static serving, and an initial generated embedded static app
binary path.

The target deployment model is one Go binary that can serve embedded static pages and backend routes. SSR remains optional.

## Runtime

- Application processes: current local development can use the `gowdk` CLI;
  static applications can run as a generated Go binary; future dynamic
  applications will add generated action, API, fragment, and SSR handlers.
- Background workers: not part of initial MVP.
- Datastores: user application choice; used by generated actions, APIs, `build {}`, or `load {}`.
- Queues: user application choice.
- External services: user application choice.

## Environments

- Local: run CLI commands such as `gowdk check`, `gowdk manifest`,
  `gowdk sitemap`, `gowdk build`, `gowdk watch`, `gowdk serve`, and `gowdk lsp`.
- Development: target flow compiles static output and generated handlers.
- Staging: target flow verifies one-binary serving and addon behavior.
- Production: target flow serves static assets, actions, APIs, fragments, and optional SSR from the generated binary.

## Observability

- Logs: compiler diagnostics and generated runtime request logs.
- Metrics: route counts, render mode counts, action failures, SSR latency when enabled.
- Traces: request-time SSR, actions, APIs, and fragments.
- Alerts: action failures, API failures, SSR errors, asset serving errors.
- Dashboards: generated manifest and route behavior should be inspectable.

## Deployment

Static deployment target:

```text
gowdk build --out dist --app .gowdk/app --bin dist/<app> <files>
gowdk build --out dist --app .gowdk/app --wasm dist/<app>.wasm <files>
```

The current generated binary serves embedded prerendered HTML, CSS, static
assets, and first-slice POST redirect action handlers from the selected output
directory. `--wasm` compiles the same generated app with `GOOS=js GOARCH=wasm`
for hosts that can run Go WebAssembly; it is not browser WASM islands. Future
generated artifacts should also serve real typed action logic, API routes,
partial fragment handlers, and optional SSR routes.

Current local development can serve generated static output with:

```sh
gowdk serve --dir dist
```

This is development tooling and does not replace generated app output when a
deployable binary is needed.

Current local development can rebuild generated static output on changes with:

```sh
gowdk watch --out dist
```

`watch` uses polling so it stays dependency-free and portable. It compares
input content hashes, so touching a file without changing its bytes does not
trigger another rebuild. For plain static `--out` builds, edits to existing
page source files use incremental static rendering: GOWDK still parses and
validates the full manifest, but writes only the changed page output and
refreshes manifests. Component, layout, CSS, config, source-set, target, app,
binary, WASM, and restart changes use the full build path. For generated app
development, `watch --restart` can rebuild and restart one generated binary
after each successful build:

```sh
gowdk watch --restart --target admin
gowdk watch --restart --out dist/app --app .gowdk/app --bin bin/app
```

Failed rebuilds leave the current process running. Generated static files,
manifests, generated app source, and embedded static files are skipped when
their bytes are unchanged, which reduces restart churn in the local redeploy
loop. This is not browser HMR.

GOWDK does not currently generate Kubernetes manifests or own deployment
configuration. Users can drive their own container or Kubernetes deployment code
by declaring static `Build.Targets` or by building selected configured modules
with repeated or comma-separated `gowdk build --module <name>` flags. The
selected modules define what is emitted to `--out`, copied into `--app`, and
embedded into `--bin` or `--wasm`; use separate output/app/bin/wasm paths when
separate artifacts need different module sets.
Generated apps identify replicas through `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose that data through `/_gowdk/health`, and include
it in `X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the
generated app creates one at process start; deployment code should set it when a
stable ID is needed across restarts.

## Generated Server Baseline

Generated servers must have conservative defaults before production release:

- Set `http.Server` read, write, idle, and header timeouts.
- Set `MaxHeaderBytes`.
- Cap action/API request body size before form or JSON decoding; generated
  action handlers currently use a fixed 1 MiB cap.
- Use signed CSRF tokens with HttpOnly, Secure, SameSite=Lax cookies when
  generated action handlers wire CSRF.
- Return explicit method-not-allowed responses for unsupported methods.
- Serve static assets with deterministic cache headers.
- Avoid public debug endpoints by default.
- Exclude local env files, private source files, and temporary build artifacts from embedded output.
- Keep logs useful for route/action/API/SSR failures without logging secrets or sensitive form values.

## Maintenance

- Backup and restore: user application responsibility.
- Data retention: user application responsibility.
- Dependency updates: keep compiler/runtime dependencies minimal and documented.
- Incident process: user application responsibility, but generated routes should expose enough logs and diagnostics to debug failures.
