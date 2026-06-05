# Login Example

This is an integrated auth-feature GOWDK login example. The auth feature keeps
its `.gwdk` UI, typed render state, backend routes, and backend entrypoint
together. At runtime, the generated GOWDK frontend binary and the feature-owned
backend binary run at the same time on different ports.

- GOWDK owns the login UI, app dashboard page, and generated frontend binary.
- `src/features/auth/auth.go` is the shared feature package. GOWDK imports it
  for login form state, and the backend binary imports it for API/session
  routes.
- Go owns the backend entrypoint until generated GOWDK actions can own this
  flow directly.
- No hand-written JavaScript is used.
- No HTML lives in Go code.

## Files

- `gowdk.config.go`: configures source discovery, CSS discovery, and build output.
- `src/features/auth/login.page.gwdk`: root page that renders `LoginApp`.
- `src/features/auth/login-app.cmp.gwdk`: login form component that posts to the feature-owned backend binary.
- `src/features/auth/auth.go`: shared auth feature state and backend route implementation.
- `src/features/auth/dashboard.page.gwdk`: app dashboard page served by the frontend binary.
- `src/features/auth/login-error.page.gwdk`: frontend fallback error route.
- `styles/auth.css`: CSS input selected by the GWDK pages with `@css auth`.
- `src/features/auth/backend/main.go`: small backend binary entrypoint that imports the auth feature package.
- `dist/site`: generated app frontend output.
- `.gowdk/frontend`: generated frontend Go app source.
- `bin/login-frontend`: generated GOWDK frontend binary.
- `bin/login-backend`: user-owned Go backend binary.

## Structure

```text
examples/login/
  gowdk.config.go
  src/features/auth/
    login.page.gwdk
    login-app.cmp.gwdk
    dashboard.page.gwdk
    login-error.page.gwdk
    auth.go
    backend/main.go
  styles/auth.css
  dist/site/
  .gowdk/frontend/
  bin/
```

## Run

From this directory:

```sh
cd examples/login
make serve
```

Open:

```text
frontend: http://127.0.0.1:8090/
backend:  http://127.0.0.1:8091/_backend/health
```

Use:

```text
email: demo@example.com
password: demo-password
```

The visible login form is rendered by the GOWDK frontend binary using state
from `src/features/auth/auth.go`. Submitting the form posts to the
feature-owned backend binary, which imports the same package, sets a signed
session cookie, and redirects back to the frontend dashboard.

## Equivalent Commands

```sh
cd examples/login
go run ../../cmd/gowdk build
go build -o bin/login-backend ./src/features/auth/backend
GOWDK_BACKEND_ADDR=127.0.0.1:8091 GOWDK_FRONTEND_ORIGIN=http://127.0.0.1:8090 bin/login-backend
GOWDK_ADDR=127.0.0.1:8090 bin/login-frontend
```

## Development Loop

Use two terminals:

Terminal 1:

```sh
cd examples/login
go run ../../cmd/gowdk dev --target frontend
```

Terminal 2:

```sh
cd examples/login
go run ./src/features/auth/backend
```

Open `http://127.0.0.1:8090/`.

## Backend Reference Routes

- `GET /_backend/health`: backend health check.
- `POST /api/login`: validates origin, checks credentials, creates a signed
  HttpOnly SameSite session cookie, and redirects to the frontend dashboard or
  returns JSON.
- `GET /api/session`: returns the current backend session as JSON.
- `POST /api/logout`: deletes the backend session, clears the cookie, and
  redirects to the frontend root.

## Current GOWDK Limitation

The backend reference API stays inside the auth feature as Go code because
GOWDK does not yet support user-owned login actions with sessions, CSRF, and
guard enforcement in generated handlers. When that lands, the backend boundary
should move into `act {}` and guard-aware `.gwdk` declarations.
