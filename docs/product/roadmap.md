# Product Roadmap

## Philosophy

Not: SSR framework with static support.

Better: static/compiler-first web kit with optional SSR.

Core rule:

```text
Core GOWDK renders at build time by default.
SSR addon renders at request time only where enabled.
```

## Mental Model

```text
GOWDK Core:
  Build my web app.

SSR Addon:
  Render this page at request time.

Actions Addon:
  Run backend mutations.

Partial Addon:
  Swap small HTML fragments.

Embed Addon:
  Ship frontend and backend in one binary.
```

## Final Rule Set

1. Files are portable.
2. Routes are declared inside files.
3. Layouts are declared by ID, not folder nesting.
4. Static render is default.
5. Actions can exist without SSR.
6. Partial updates use server fragments, not full page SSR.
7. SSR is an addon.
8. One-binary deploy works with or without SSR.

## Phase Roadmap

Current implementation status and remaining gaps are tracked in `docs/product/missing-implementation-checklist.md`.

| Phase | Scope |
| --- | --- |
| 1 | Portable file manifest. |
| 2 | Component compiler. |
| 3 | Static/prerender output. |
| 4 | CSS/plugin extension points. |
| 5 | One-binary static server. |
| 6 | Typed actions and forms. |
| 7 | Partial/server fragments. |
| 8 | SSR addon. |
| 9 | Hybrid render modes. |
| 10 | Component library. |
| 11 | WASM islands. |

## Version Roadmap

### v0.1

- Movable `.gwdk` files.
- `@page`, `@route`, `@layout`.
- `.cmp.gwdk` components.
- Static render.
- CSS/plugin extension points.
- Embedded assets.
- One-binary static serving.

### v0.2

- Typed actions.
- Form decoding.
- Validation.
- Redirects.
- Static `g:post` form lowering.
- CSRF.
- Server fragments.

### v0.3

- Partial updates.
- Enhanced `g:post` submissions.
- `g:target`.
- `g:swap`.
- Loading states.
- Focus restoration.

### v0.4

- SSR addon.
- `@render ssr`.
- Request-time `load {}`.
- Guards.
- Request layouts.
- Error boundaries.

### v0.5

- Hybrid mode.
- Per-route static, SSR, action, and client choices.
- Better caching.
- Revalidation.

### v0.6

- WASM islands.
