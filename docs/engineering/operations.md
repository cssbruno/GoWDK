# Operations

## Current Status

The current repository provides compiler/runtime tooling, SPA output, local
generated-output serving, generated embedded app source, and local generated
binary builds.

The target deployment model is one Go binary that can serve embedded SPA
pages and backend routes. Today, that binary serves embedded build output plus
supported action, API, fragment, guard, rate-limit, SSR, and explicit
hybrid-with-load request-time slices. SSR remains optional.

## Runtime

- Application processes: current local development can use the `gowdk` CLI;
  generated applications can run as a Go binary with supported action, API,
  fragment, guard, and SSR handlers.
- Background workers: not part of initial MVP.
- Datastores: user application choice; usable from imported build-time Go data
  functions and from user-owned action, API, fragment, and `load {}` handlers.
- Queues: user application choice.
- External services: user application choice.

## Environments

- Local: run CLI commands such as `gowdk check`, `gowdk manifest`,
  `gowdk sitemap`, `gowdk build`, `gowdk dev`, `gowdk serve`, and `gowdk lsp`.
- Development: current flow compiles build output, generated app source, and
  supported request-time handlers.
- Staging: target flow verifies one-binary serving and addon behavior.
- Production: not a supported readiness claim yet. Current generated binaries
  can serve embedded app assets, supported action/API/fragment handlers,
  generated guards, SSR pages with declared `load {}` identifier or dotted
  paths, and hybrid pages with or without declared `load {}` data. Hybrid
  streaming, data refresh, and non-HTTP revalidation remain planned.

## Observability

- Logs: compiler diagnostics and current generated runtime request logs.
- Metrics: route counts and render mode counts are available through manifests.
  `runtime/app.Metrics` records dependency-free request counters for generated
  app dispatch paths, and generated apps can expose snapshots through
  `/_gowdk/health` when a metrics collector is attached.
- Traces: request-time SSR, actions, APIs, and fragments are future production
  observability concerns.
- Alerts: action failures, API failures, SSR errors, and asset serving errors
  are future production concerns.
- Dashboards: generated manifest and route behavior should be inspectable.

## Deployment

Build-output deployment target:

```text
gowdk build --out dist --app .gowdk/app --bin dist/<app> <files>
gowdk build --out dist --app .gowdk/app --wasm dist/<app>.wasm <files>
```

The current generated binary serves embedded prerendered HTML, CSS, SPA
assets, supported POST action handlers, partial action fragment responses,
standalone fragment routes, API handlers, guarded request-time lanes, concrete
or dynamic SSR pages with declared `load {}` paths, and hybrid pages that
explicitly declare `load {}` from the selected output directory. `--wasm`
compiles the same generated app with `GOOS=js GOARCH=wasm` for hosts that can
run Go WebAssembly; it is not browser WASM islands. Future generated artifacts
should improve production operations guidance, richer validation, broader
hybrid streaming/data refresh and broader generated-client reactivity.

Current local development can serve generated build output with:

```sh
gowdk serve --dir dist
```

This is development tooling and does not replace generated app output when a
deployable binary is needed.

Current local development can rebuild generated build output on changes, serve
it, and live reload browsers with:

```sh
gowdk dev --out dist
```

`dev` uses polling so it stays dependency-free and portable. It compares
input content hashes, so touching a file without changing its bytes does not
trigger another rebuild. For plain SPA `--out` builds, edits to existing
page source files use incremental SPA rendering: GOWDK still parses and
validates the full compiler IR, but writes only the changed page output and
refreshes manifests. Component, layout, CSS, config, source-set, target, app,
binary, and WASM changes use the full build path. Generated build output files,
manifests, generated app source, and embedded build output files are skipped
when their bytes are unchanged, which reduces churn in the local dev loop. This
is live reload, not browser HMR.

GOWDK does not currently generate Kubernetes manifests or own deployment
configuration. Users can drive their own container or Kubernetes deployment code
by declaring SPA `Build.Targets` or by building selected configured modules
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

Generated servers must have conservative defaults before any production
readiness claim:

- Set `http.Server` read, write, idle, and header timeouts.
- Set `MaxHeaderBytes`.
- Cap action/API request body size before form or JSON decoding; generated
  apps default to 1 MiB and can override action/API caps with
  `Build.BodyLimits`.
- Do not set `Build.CSRF.Disabled` for production generated action or command
  handlers. Provide a stable `GOWDK_CSRF_SECRET` or configured
  `Build.CSRF.SecretEnv` value in each runtime environment.
- Return explicit method-not-allowed responses for unsupported methods.
- Serve app assets with deterministic cache headers.
- Avoid public debug endpoints by default.
- Exclude local env files, private source files, and temporary build artifacts from embedded output.
- Keep logs useful for route/action/API/SSR failures without logging secrets or
  sensitive form values. Generated panic-boundary logs and diagnostics redact
  common secret fields (`password`, `secret`, `_gowdk_csrf`, cookies,
  authorization headers, session IDs, and access/refresh/id tokens), but
  app-owned logs remain the application's responsibility.

## Maintenance

- Backup and restore: user application responsibility.
- Data retention: user application responsibility.
- Dependency updates: keep compiler/runtime dependencies minimal and documented.
- Incident process: user application responsibility, but generated routes should expose enough logs and diagnostics to debug failures.
