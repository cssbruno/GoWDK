# Hybrid Rendering

`@render hybrid` is SPA-first. It does not mean full-page SSR by default.

## Stable Modes

| Page shape | Output | Notes |
| --- | --- | --- |
| `@render hybrid` without `load {}` | build-time SPA output | Works like SPA output and appears as `spa` route behavior with an `ssr_disabled` info note. |
| `@render hybrid` with `load {}` | request-time page route | Requires SSR enabled and uses the same generated load/render path as SSR. |

## Rules

- Bare hybrid pages are prerendered as build-time SPA output.
- Hybrid pages with `load {}` are not prerendered as static HTML for that route;
  they run request-time `Load<PageID>` data.
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
- A hybrid page with `load {}` requires the SSR feature gate. Use the SSR addon
  in config or pass `--ssr` for CLI checks/builds.
- A bare hybrid route reports request-time rendering as disabled in route info.

## Examples

Build-time hybrid page:

```gwdk
package pages

@page marketing
@route "/"
@render hybrid

view {
  <main>Static page output</main>
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

Use plain `dev` output for bare hybrid SPA pages. Use `--app` with SSR enabled
when a hybrid page declares `load {}`.
