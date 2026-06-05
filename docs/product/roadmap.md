# Product Roadmap

## Philosophy

Not: SSR framework with static support.

Better: static/compiler-first app compiler with optional SSR.

Core rule:

```text
Core GOWDK renders at build time by default.
SSR addon renders at request time only where enabled.
```

## Mental Model

```text
GOWDK Core:
  Ship apps.

SSR Addon:
  Render this page at request time.

Actions Addon:
  Run backend mutations.

Partial Addon:
  Swap small HTML fragments.

Embed Addon:
  Ship frontend and backend in one binary.

Build Selection:
  Declare which configured modules are compiled into each generated binary.

Watch Redeploy:
  Rebuild, incrementally refresh page-local static edits, and restart generated
  binaries during local development without retriggering on no-op writes.
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
9. Static build targets define what each generated app or binary embeds.
10. Local watch can rebuild and restart generated binaries without Air, using
    content changes and page-local incremental output instead of timestamp-only
    churn.

## Phase Roadmap

Current implementation status is summarized in `README.md` and
`docs/product/requirements.md`.

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
| 11 | Generated JS islands by default; explicit WASM islands. |

## Version Roadmap

The version roadmap is a target sequence, not a statement that those versions
are shippable today. The repository currently reports CLI version `0.1.0`, but
the implementation is still pre-release and does not satisfy every v0.1 target.
Do not treat v0.2+ bullets as completed unless `docs/product/requirements.md`
marks the corresponding requirement implemented.

### v0.1

- Movable `.gwdk` files.
- `@page`, `@route`, `@layout`.
- `.cmp.gwdk` components.
- Static render.
- CSS/plugin extension points.
- Embedded assets.
- One-binary static serving.
- Generated Go WASM deploy artifacts.
- Static module-selected build targets for generated apps and binaries.
- Dependency-free watch rebuild, page-local incremental static output,
  generated-binary restart, and no-op output write skipping.
- Honest release/version readiness docs.

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
- Hybrid/cache/revalidation policy docs.

### v0.6

- Generated JavaScript islands for typed local component state.
- Explicit `g:island="wasm"` artifacts.
- Production WASM island ABI and browser-side Go logic.
