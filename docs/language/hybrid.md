# Hybrid Rendering

`@render hybrid` is an explicit request-time page lane. It uses the same
generated route/render path as SSR, but reports route behavior as `hybrid` so
future hybrid-specific capabilities can stay distinct from full SSR.

## Stable Modes

| Page shape | Output | Notes |
| --- | --- | --- |
| `@render hybrid` without `load {}` | request-time page route | Requires SSR enabled and renders build-time data through the generated request-time handler. |
| `@render hybrid` with `load {}` | request-time page route | Requires SSR enabled and uses the same generated load/render path as SSR. |

## Rules

- Hybrid pages are not prerendered as static HTML for that route; they run
  through generated request-time handlers.
- Hybrid pages without `load {}` render route params and build-time data at
  request time.
- Hybrid pages with `load {}` also run request-time `Load<PageID>` data.
- `@cache` and `@revalidate` remain HTTP response cache policy. They do not add
  background regeneration or data-dependency revalidation.
- Hybrid pages do not stream today.
- Hybrid pages do not partially refresh server data today. Use explicit
  fragments for partial updates.
- Generated JavaScript may enhance static SPA navigation and forms, but it does
  not own hybrid route existence, server data, cache, auth, or validation.

## Diagnostics

- A SPA page with `load {}` is rejected. Use `@render ssr` or
  `@render hybrid`.
- Any hybrid page requires the SSR feature gate. Use the SSR addon in config or
  pass `--ssr` for CLI checks/builds.

## Examples

Hybrid page without request-time load data:

```gwdk
package pages

@page marketing
@route "/"
@render hybrid

view {
  <main>Request-time hybrid shell</main>
}
```

Request-time hybrid page:

```gwdk
package pages

@page dashboard
@route "/dashboard"
@render hybrid

load {
  => { user.name }
}

view {
  <main>Hello {user.name}</main>
}
```

Local development:

```sh
gowdk dev --out dist/site
gowdk dev --ssr --out dist/site --app .gowdk/app
```

Use `--app` with SSR enabled for hybrid pages so the generated request-time
route handlers can run.
