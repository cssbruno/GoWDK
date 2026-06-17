# Auth Guard Example

This example shows the experimental 0.x auth addon around generated guards and
CSRF-protected actions.

Run from this directory:

```sh
make check
make routes
make build
GOWDK_AUTH_SESSION_SECRET=development-auth-session-secret-32bytes GOWDK_CSRF_SECRET=development-auth-csrf-secret-32bytes GOWDK_ADDR=127.0.0.1:8094 bin/auth-guard
```

Open `http://127.0.0.1:8094/`.

The example sets `auth.Options.Insecure` so localhost HTTP works. Real apps
should leave that option false when serving behind HTTPS.

Use:

```text
email: demo@example.com
password: demo-password
```

## Files

- `gowdk.config.go`: enables `auth.Addon(auth.Options{...})` and `ssr.Addon()`,
  declares the required session and CSRF secrets, and builds one generated
  binary.
- `src/authguard/auth.go`: owns demo credentials, password verification,
  login/logout handlers, and SSR load data in normal Go. Generated auth startup
  owns the default session manager and `auth.required` guard.
- `src/authguard/login.page.gwdk`: public login route and CSRF-protected login
  action.
- `src/authguard/dashboard.page.gwdk`: protected SSR dashboard with
  `guard auth.required, role:user` and a CSRF-protected logout action.

## Ownership

GOWDK owns generated route dispatch, guard execution order, CSRF token
injection/validation, signed session cookie helpers, default `auth.required`,
and native RBAC guard checks. The app owns users, credential policy, durable
storage, custom guard decisions, and backend resource authorization.

Runtime secrets are separate: `GOWDK_AUTH_SESSION_SECRET` signs sessions and
`GOWDK_CSRF_SECRET` signs generated action tokens. Both must be stable
environment values of at least 32 bytes. Secret values are not stored in config.
