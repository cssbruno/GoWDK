# Guards And Default-Deny Page Access

This is the single source of truth for how `guard` and page access control
work in GOWDK. Other docs (spec, routing, ssr, hooks) describe their own
concerns and link here for the access contract.

## The Contract

A page is **not public by default**. Access is never granted by omission.

- `guard` is **optional** on a page source. A page that declares no `guard`
  still builds — the build succeeds — but it is denied (403) at request time
  until access is stated.
- A guardless page emits a `missing_page_guard` **warning** so the omission is
  visible to authors and editors.
- Use `guard public` to serve a page on purpose.
- Use custom guard IDs, or native RBAC IDs such as `role:admin` and
  `permission:posts.write`, when the page is protected.
- `guard public` must stand alone — it cannot be combined with other guard IDs
  (`public_guard_exclusive`).

```gwdk
route "/"
guard public          # intentionally public

route "/dashboard"
guard auth.required   # protected

route "/draft"
# no guard -> builds with a warning, route returns 403 until a guard is added
```

## How Denial Is Enforced

The default-deny is enforced differently per render mode, but the observable
result is the same: a guardless page route returns **403**.

| Page kind | Enforcement |
| --- | --- |
| Static / build-time (SPA) | The generated app carries a deny registry. The route returns 403 before serving any static artifact. |
| Dynamic build-time (`paths {}`) | The page **route pattern** (e.g. `/blog/{slug}`) is denied, so every concrete artifact expanded from `paths {}` returns 403 — not just the pattern string. |
| Request-time (SSR / `server {}`) | The generated SSR handler returns 403 before running any context, load, or HTML statements. |

The deny check normalizes the request path first, so a page emitted as
`<route>/index.html` is denied when fetched directly by its file path
(`/dashboard/index.html`) and by its trailing-slash directory form, not only by
its canonical route.

### Backend Endpoints Cannot Be Public By Omission

A page that declares `act`, `api`, or `fragment` blocks derives request-time
endpoints that inherit the page's guards. If such a page declared no `guard`,
those endpoints would be publicly callable even though the page's own GET route
is denied. That contradicts the contract, so it is a **build error** (not a
warning): a guardless page with backend endpoints fails the build with
`missing_page_guard` until a guard is declared.

## Static Export Caveat

The 403 is enforced by the generated Go server. A pure static export served
without that server cannot enforce denial — the build warning is the backstop.
Do not rely on static hosting alone to protect a guardless page.

## Status

`guard` validation currently records and checks metadata and enforces the
default-deny described above. Guard functions return `nil` to allow a request
or an `error` to stop it. Ordinary errors fail closed with 403; explicit
`runtime/guard.RedirectTo`, `runtime/guard.Redirect`, and
`runtime/guard.Respond` errors write no-store redirects or custom responses.
Full authorization and richer request-local state are still planned — see
[docs/engineering/security.md](../engineering/security.md).

## Related

- [spec.md](spec.md) — full page keyword and metadata declaration contract.
- [docs/reference/routing.md](../reference/routing.md) — route validation and plans.
- [ssr.md](ssr.md) — request-time render mode and `server {}`.
- [diagnostics.md](diagnostics.md) — `missing_page_guard`, `public_guard_exclusive`.
