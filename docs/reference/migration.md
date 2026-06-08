# Migration Reference

GOWDK migration should make ownership boundaries explicit. `.gwdk` files declare
pages, routes, build-time data, SSR selection, actions, APIs, fragments, and
markup. Go packages keep auth, validation, persistence, business rules, and
handler behavior.

## Existing Go HTTP Apps

1. Keep the current Go handlers.
2. Add `.gwdk` pages for static or SSR output.
3. Declare action/API routes that call exported Go handlers.
4. Build the generated app as either a single binary or split frontend/backend.
5. Move route ownership only when the generated route is verified.

Do not import generated app output from feature packages. Feature packages
import public `runtime/` or addon packages only.

## Go Templates

Move one template at a time:

- Static template data becomes `build {}` when it is known at build time.
- Request-time template data becomes `load {}` and requires `ssr.Addon()`.
- POST handlers become `act Name POST "/path"` plus exported Go functions.
- JSON endpoints become `api Name METHOD "/path"` plus exported Go functions.
- Shared layout decisions stay in `.gwdk`; domain decisions stay in Go.

## htmx-Style Apps

Map htmx-style partial updates to server fragments:

- Keep full-page POST fallback working first.
- Add fragment responses for enhanced requests.
- Use explicit `g:target` and swap policy.
- Keep validation in generated request-shape checks or user Go handlers.

## JavaScript Framework Apps

Do not port framework concepts one-for-one. GOWDK does not make generated
JavaScript own routing, auth, load policy, action behavior, or global state.
Keep client behavior bounded to explicit islands and partial-update runtime.

## Previous GOWDK Slices

Older `act name { ... }` and `api name { ... }` block forms are rejected with
migration diagnostics. Use declaration forms:

```gwdk
act Submit POST "/signup"
api Session GET "/api/session"
```

Missing or unsupported Go handlers may generate temporary `501` stubs only when
the build explicitly allows missing backends. Production builds should bind to
real exported Go handlers.
