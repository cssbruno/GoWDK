# Feature Spec: Embedded Static App

## Problem

GOWDK can emit static HTML and serve it locally, but v0.1 still needs a
deployable Go binary path. Users should be able to compile simple static GOWDK
output into a generated Go app that embeds the generated files and serves them
without request-time full-page rendering.

## Goals

- Add a `gowdk build` option that writes generated Go app source for the current
  static output directory.
- Add an optional `gowdk build` option that compiles the generated app into one
  local binary.
- Embed only files from the selected static output directory.
- Serve root and extensionless static routes from embedded `index.html` files.
- Keep the generated server conservative: GET/HEAD only, no directory listings,
  HTTP timeouts, and bounded headers.

## Non-Goals

- Generate action, API, partial fragment, SSR, or hybrid request-time handlers.
- Generate route registration from `internal/codegen.RouteBinding`.
- Add cache-control, custom error pages, TLS, metrics, or graceful shutdown.
- Replace `gowdk serve`; the local server remains useful during development.

## Users And Permissions

- Primary users: Go developers building simple static-first GOWDK apps.
- Roles or permissions: anyone who can run the compiler and Go toolchain.
- Data visibility rules: generated binaries embed only the selected build output
  directory, not source files outside that directory.

## User Flow

1. User runs `gowdk build --out dist --app .gowdk/app --bin dist/site <files>`.
2. GOWDK emits static files into `dist`.
3. GOWDK writes a generated Go module under `.gowdk/app` with the static output
   copied under `.gowdk/app/static`.
4. GOWDK builds `dist/site`.
5. User runs `GOWDK_ADDR=127.0.0.1:8080 dist/site`.

## Requirements

### Functional

- `gowdk build --app <dir>` writes `go.mod`, `main.go`, and copied static files
  under `<dir>/static`.
- `gowdk build --bin <file>` requires `--app <dir>` and runs `go build` for the
  generated app.
- The generated app serves `/` from embedded `static/index.html`.
- The generated app serves `/blog/post` from embedded
  `static/blog/post/index.html`.
- The generated app returns 405 for non-GET/HEAD methods.
- The generated app does not list embedded directories.
- The generated app reads `GOWDK_ADDR` and defaults to `127.0.0.1:8080`.
- The generated app reads `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and
  `GOWDK_INSTANCE_ID` for deploy-time identity.
- The generated app auto-generates an instance ID at process start when
  `GOWDK_INSTANCE_ID` is omitted.
- The generated app exposes `/_gowdk/health` with status, app, module, and
  instance ID.
- The generated app adds instance identity headers to responses.

### Non-Functional

- Performance: static pages are served from embedded files without request-time
  page rendering.
- Reliability: generation fails before app source is written when the app
  directory is inside the static output directory.
- Accessibility: generated serving does not change emitted HTML.
- Security/privacy: generated app input is restricted to the chosen static output
  directory; server defaults set timeouts and header limits.
- Observability: generated app logs its listen address and server failures, and
  exposes deploy-time instance identity through headers and health output.

## Acceptance Criteria

- [x] `gowdk build --out <dir> --app <app-dir> <files>` writes a generated app
  source tree.
- [x] `gowdk build --out <dir> --app <app-dir> --bin <binary> <files>` builds a
  runnable binary.
- [x] A binary built from the generated app serves the home page over HTTP.
- [x] A binary built from the generated app serves extensionless nested routes.
- [x] A binary built from the generated app exposes module and instance identity.
- [x] Unit tests cover generated app files and option validation.
- [x] Docs describe the generated app command, binary command, and current
  limitations.

## Edge Cases

- `--bin` without `--app` fails with usage guidance.
- Empty `--app` or `--bin` values fail.
- App directories inside the static output directory fail to avoid embedding
  generated source into itself.
- Directory requests without an `index.html` return 404.

## Dependencies

- Internal: current `internal/staticgen` output and route-derived file layout.
- External: Go toolchain when `--bin` is requested.

## Open Questions

- Where should generated action/API/fragment packages live once those features
  exist?
- Should generated binaries support configurable cache-control policies in v0.1?
