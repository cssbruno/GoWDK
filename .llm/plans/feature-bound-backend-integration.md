# Implementation Plan: Feature-Bound Backend Integration

## Context

GOWDK is positioned as a compile-first Go web compiler where frontend markup,
backend actions, API routes, partials, and optional SSR are part of one product
model. The current implementation still leaves a practical separation between
frontend `.gwdk` files and user-owned backend Go code:

- `.gwdk` files can import Go for build-time data and component state.
- `act {}` and `api {}` blocks are parsed and appear in route plans.
- Generated apps can serve first-slice action redirects and fragments.
- Generated apps do not discover or call real user Go action/API handlers.

The result is that examples must either hardcode backend URLs in views or place
a separate backend binary next to the frontend. This does not provide enough
GOWDK-owned glue to make frontend and backend feel congruent.

This plan adds feature-bound backend integration: a `.gwdk` file can declare
actions and APIs, and GOWDK automatically discovers matching Go handlers in the
same feature package, wires them into generated apps, and reports binding status.

## Assumptions

- Scope is core GOWDK plus the login example.
- Runtime support must cover both one generated binary and optional split
  frontend/backend binaries.
- One generated GOWDK binary is the preferred product direction.
- Split binaries remain useful for local development and service-style deploys.
- Backend discovery uses the same feature package as the `.gwdk` file.
- Handler names come from block names:
  - `act login` maps to exported Go function `Login`.
  - `api session` maps to exported Go function `Session`.
- Missing handlers are not build errors. Generated runtime returns HTTP `501 Not
  Implemented` with a clear message.
- Unsupported handler signatures are not build errors in the first slice.
  Generated runtime returns HTTP `501 Not Implemented` and reports the issue in
  binding metadata.
- Routes stay declared in `.gwdk` files. Folder location selects feature package
  ownership, not route identity.

## Proposed Changes

- Add a feature-binding discovery pass after manifest validation.
  - For each page, identify the same-directory Go package for the `.gwdk` file.
  - Resolve the Go import path using existing `go list`-style infrastructure.
  - Inspect exported functions in that package.
  - Create binding records for every `act` and `api` block.
  - Mark each binding as `bound`, `missing`, or `unsupported_signature`.

- Support minimal first-slice handler signatures.
  - Action handler:
    `func Login(context.Context, form.Values) (response.Response, error)`.
  - API handler:
    `func Session(context.Context, *http.Request) (response.Response, error)`.
  - Reuse existing `runtime/form.Values` and `runtime/response.Response`.
  - Do not introduce a separate action/API response envelope.

- Wire feature-bound handlers into generated one-binary apps.
  - Generated app code imports only packages that have at least one bound handler.
  - Generated action routes decode and validate forms as they do today, then call
    the discovered Go function when present.
  - Generated API routes call the discovered Go function when present.
  - Missing or unsupported bindings generate `501` handlers without referencing
    missing symbols.
  - Handler errors use `runtime/response.HandlerStatus` and no-store error
    responses for request-time routes.

- Add split frontend/backend binary support.
  - Add `BuildTargetConfig.BackendApp` and `BuildTargetConfig.BackendBinary`.
  - Add CLI flags `--backend-app <dir>` and `--backend-bin <file>`.
  - In split mode, frontend binary serves SPA output and proxies generated
    backend routes to `GOWDK_BACKEND_ORIGIN`.
  - Backend binary is generated from the same feature binding records and imports
    the same user feature packages.
  - If split backend options are omitted, behavior remains one-binary by default.

- Improve reporting and diagnostics.
  - Add binding status to `gowdk routes`.
  - Add binding status to public manifest JSON or build report, whichever is
    already the best home for generated behavior metadata.
  - Include source path, block kind, block name, route, feature import path,
    expected function name, and status.
  - Keep missing handlers non-fatal.
  - Use clear runtime messages, for example:
    `GOWDK action handler auth.Login is not implemented`.

- Update `examples/login`.
  - Keep auth UI and Go backend behavior under `src/features/auth/`.
  - Replace hardcoded backend URLs with `act` and `api` declarations.
  - Implement `Login`, `Session`, and `Logout` in the auth feature package using
    supported signatures.
  - Demonstrate one-binary build as the preferred path.
  - Keep split-runtime `make serve` as an optional flow that runs frontend and
    backend binaries together.

- Update docs.
  - Document feature-bound backend integration in routing/actions/API references.
  - Document same-package discovery in project-structure docs.
  - Update examples README to explain one-binary and split-binary flows.
  - Update current limitations to remove claims that generated apps cannot call
    user Go action/API handlers once this slice is implemented.

## Files Expected To Change

- `internal/manifest`: add backend binding metadata or supporting model types.
- `internal/compiler`: add feature-binding discovery and validation/reporting.
- `internal/appgen`: generate action/API calls, 501 stubs, and split backend app
  output.
- `internal/codegen`: reuse or adapt existing registry-backed action/API output
  for feature-bound direct calls.
- `cmd/gowdk`: add split backend flags and build target fields.
- `runtime/response` and `runtime/form`: reuse existing contracts; add helpers
  only if generated code becomes repetitive.
- `docs/reference/routing.md`, `docs/language/actions.md`,
  `docs/language/api.md`, and `docs/compiler/project-structure.md`.
- `examples/login`.

## Data And API Impact

- Public config adds optional fields:
  - `BuildTargetConfig.BackendApp string`
  - `BuildTargetConfig.BackendBinary string`
- CLI adds optional flags:
  - `--backend-app <dir>`
  - `--backend-bin <file>`
- Route/build metadata gains backend binding status.
- Existing `.gwdk` syntax remains compatible.
- Existing builds without feature-bound handlers continue to work.
- Missing handlers become visible at runtime as `501`, not as build failures.

## Tests

- Unit:
  - Discover same-package handler bindings for actions and APIs.
  - Map block names to exported Go function names.
  - Report `missing` when a function does not exist.
  - Report `unsupported_signature` when a function exists with the wrong shape.
  - Keep duplicate route and render-mode validation behavior unchanged.

- Integration:
  - Generated one-binary app calls a discovered action handler.
  - Generated one-binary app calls a discovered API handler.
  - Generated app returns `501` for missing action handlers.
  - Generated app returns `501` for missing API handlers.
  - Split backend app imports feature packages and serves backend routes.
  - Split frontend app proxies backend routes to `GOWDK_BACKEND_ORIGIN`.

- End-to-end:
  - Login example one-binary flow signs in and redirects to `/dashboard`.
  - Login example split-binary flow signs in and redirects to frontend
    `/dashboard`.
  - Existing action redirect, partial fragment, SPA, and SSR smoke tests keep
    passing.

- Manual:
  - Inspect `gowdk routes` output and confirm bound/missing/unsupported status.
  - Inspect generated build report and confirm binding metadata is present.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/compiler ./internal/appgen ./internal/codegen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
cd examples/login && make check
cd examples/login && make build
```

## Rollback Plan

- Keep current SPA/action/SSR generation paths intact while adding feature
  binding as an additional route execution path.
- If direct feature imports cause regressions, disable bound handler call
  generation and keep generated `501` stubs.
- If split backend generation causes regressions, retain one-binary support and
  remove `BackendApp`/`BackendBinary` CLI/config wiring.
- Revert docs and login example to first-slice generated action behavior if the
  binding model proves too broad for the current compiler slice.

## Risks

- Same-package Go discovery can be slow if every `.gwdk` file invokes `go list`
  independently. Cache package inspection by directory/import path.
- Feature packages must not import generated app packages, or import cycles will
  occur. Document this explicitly.
- Generated code must avoid referencing missing or unsupported functions.
- Split-runtime proxying can obscure deploy boundaries if not clearly documented
  as optional.
- The first supported handler signatures are intentionally narrow; expanding
  typed action/API bodies should happen in later slices.
