# Login Example

This is a feature-bound auth example. The `.gwdk` files declare the login
actions and session API, and GOWDK discovers matching Go handlers in the same
feature package.

- One generated binary is the default.
- `src/features/auth/auth.go` implements `Login`, `Logout`, and `Session`.
- Split frontend/backend binaries are optional and generated from the same
  declarations.
- No hand-written JavaScript is used.
- No HTML lives in Go code.

## Files

- `gowdk.config.go`: configures one-binary and split generated targets.
- `src/features/auth/login.page.gwdk`: root login page with `act login` and
  `api session`.
- `src/features/auth/dashboard.page.gwdk`: dashboard page with `act logout`.
- `src/features/auth/auth.go`: same-package feature-bound handlers.
- `src/features/auth/login-app.cmp.gwdk`: reusable login panel component.
- `src/features/auth/login-error.page.gwdk`: failed-login route.
- `styles/auth.css`: CSS input selected by pages with `@css auth`.
- `.gowdk/output/login`: inferred generated output for the one-binary target.
- `.gowdk/output/split`: inferred generated output for the split target.
- `.gowdk/login`: generated one-binary Go app source.
- `.gowdk/login-frontend` and `.gowdk/login-backend`: generated split app sources.
- `bin/login`: generated one-binary app.
- `bin/login-frontend` and `bin/login-backend`: generated split binaries.

## Run

From this directory:

```sh
cd examples/login
make serve
```

Open `http://127.0.0.1:8090/`.

Use:

```text
email: demo@example.com
password: demo-password
```

The generated app calls `auth.Login`, sets a signed HttpOnly SameSite session
cookie, and redirects to `/dashboard`. `GET /api/session` calls `auth.Session`.

## Split Mode

Split mode keeps the same `.gwdk` declarations but generates separate frontend
and backend binaries. The frontend serves app output and proxies action/API
routes to `GOWDK_BACKEND_ORIGIN`.

```sh
cd examples/login
make serve-split
```

Open:

```text
frontend: http://127.0.0.1:8090/
backend:  http://127.0.0.1:8091/
```

## Equivalent Commands

```sh
cd examples/login
go run ../../cmd/gowdk build --target login
GOWDK_ADDR=127.0.0.1:8090 bin/login
```

For split mode:

```sh
go run ../../cmd/gowdk build --target split
GOWDK_ADDR=127.0.0.1:8091 bin/login-backend
GOWDK_ADDR=127.0.0.1:8090 GOWDK_BACKEND_ORIGIN=http://127.0.0.1:8091 bin/login-frontend
```

## Backend Routes

- `POST /`: bound from `act login` to `auth.Login`.
- `POST /dashboard`: bound from `act logout` to `auth.Logout`.
- `GET /api/session`: bound from `api session` to `auth.Session`.
