# Flagship Full-Stack Example

This example keeps a small vertical slice in one native GOWDK app. It builds a
single generated Go binary with static pages, request-time endpoints, SSR load
data, server fragments, local island state, a WASM island placeholder, CSS,
component assets, contracts, guards, and an optional rate limiter hook.

Run from this directory:

```sh
make check
make routes
make build
GOWDK_CSRF_SECRET=development-flagship-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8092 bin/flagship
```

Expected build outputs:

- `dist/` contains generated app output, route metadata, asset metadata, and the
  build report.
- `.gowdk/app/` contains the generated Go app source.
- `bin/flagship` is the one-binary server.

## Surfaces

| Surface | File |
| --- | --- |
| Static home with imported build-time Go data | `src/app/home.page.gwdk`, `src/data/copy.go` |
| Local component state and WASM island call site | `src/app/counter.cmp.gwdk`, `src/ui/counter.go` |
| Action login form with generated field validation | `src/app/home.page.gwdk`, `src/app/app.go` |
| API status endpoint | `src/app/home.page.gwdk`, `src/app/app.go` |
| Server fragment and partial form update | `src/app/home.page.gwdk`, `src/app/app.go` |
| Protected SSR dashboard with `load {}` | `src/app/dashboard.page.gwdk`, `apphooks/flagship_hooks.go.txt` |
| Hybrid request-time page | `src/app/hybrid.page.gwdk` |
| Component asset and configured CSS | `src/app/asset-badge.cmp.gwdk`, `src/app/badge.svg`, `styles/flagship.css` |
| Command/query contracts | `src/contracts/contracts.go` |
| Optional rate limiting | `apphooks/flagship_hooks.go.txt` |

## Routes

The main generated routes are:

- `GET /` static shell with the login action, status API link, fragment form,
  contract form/query markers, counter island, and component asset.
- `POST /login` validates direct form fields, creates a signed demo session,
  and redirects to `/dashboard`.
- `GET /api/status` returns generated JSON from app-owned Go.
- `POST /summary` returns a fragment targeting `#summary`.
- `GET /fragments/summary` serves the standalone declared fragment.
- `GET /dashboard` runs guard `auth.required`, then `LoadDashboard`.
- `POST /logout` clears the signed demo session.
- `GET /hybrid` demonstrates a request-time hybrid page with build-time copy.
- `POST /workflow/start` and the page-route query adapter are generated from
  `g:command` and `g:query`.

## Ownership Boundaries

- `.gwdk` files declare routes, render lanes, forms, fragments, contracts,
  CSS, assets, and island metadata.
- Go packages under `src/` own credentials, session state, endpoint behavior,
  build-time data, SSR load data, contracts, and island state shapes.
- `apphooks/flagship_hooks.go.txt` is copied into the generated app package before
  binary compilation so custom guards and the optional rate limiter can be wired
  through the generated app hook surface.
- Generated output in `.gowdk/`, `dist/`, and `bin/` is intentionally ignored.

## Demo Credentials

Use `demo@example.com` and `demo-password`. Override them with
`GOWDK_FLAGSHIP_EMAIL`, `GOWDK_FLAGSHIP_PASSWORD`, and
`GOWDK_FLAGSHIP_SECRET`.

## Current Limitations

- Custom guards and rate limiter registration are generated-app hooks today, so
  `make build` prepares `apphooks/flagship_hooks.go.txt` before compiling the
  binary. Running `gowdk build --target flagship` directly from a clean tree will
  miss those hooks.
- The WASM island uses the current call-site placeholder path. A real browser Go
  WASM package can replace it when the example needs browser-owned Go logic.
- Contract command/query adapters are local in-process web adapters; realtime
  transport and split worker wiring remain outside this example.
- Session storage is in memory for the demo. Production apps should use normal
  app-owned durable session storage.
