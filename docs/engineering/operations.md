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
  `gowdk sitemap`, `gowdk build`, `gowdk serve`, and `gowdk lsp`.
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
```

The current generated binary serves embedded prerendered HTML, CSS, static
assets, and first-slice POST redirect action handlers from the selected output
directory. Future generated binaries should also serve real typed action logic,
API routes, partial fragment handlers, and optional SSR routes.

Current local development can serve generated static output with:

```sh
gowdk serve --dir dist
```

This is development tooling and does not replace generated app output when a
deployable binary is needed.

GOWDK does not currently generate Kubernetes manifests or own deployment
configuration. Users can drive their own container or Kubernetes deployment code
by building selected configured modules with `gowdk build --module <name>`.
Generated apps identify replicas through `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose that data through `/_gowdk/health`, and include
it in `X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the
generated app creates one at process start; deployment code should set it when a
stable ID is needed across restarts.

## Generated Server Baseline

Generated servers must have conservative defaults before production release:

- Set `http.Server` read, write, idle, and header timeouts.
- Set `MaxHeaderBytes`.
- Cap action/API request body size before form or JSON decoding.
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
