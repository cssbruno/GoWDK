# Blocks

## Current Support

The parser records whether these top-level blocks are present:

- `paths {}`: declares dynamic SPA paths. Presence and raw body
  text are recorded. SPA builds support the first literal subset:
  `=> { slug: "hello-gowdk" }`.
- `build {}`: build-time data block. Presence and raw body text are recorded.
  SPA builds support the first literal subset, `=> { title: "Hello" }`, and
  the first imported Go function subset, `=> interop.FeaturedCopyForBuild()`.
- `load {}`: request-time data block. Presence and raw body text are recorded,
  then rejected on SPA/action pages.
- `view {}`: markup render block. Presence and body text are recorded; a
  minimal app-shell HTML subset is parsed for `gowdk build`. A direct nested
  `style {}` block can declare CSS for that page, component, or layout.

Actions and APIs are endpoint declarations, not blocks:

```gwdk
act Submit POST "/submit"
api Health GET "/api/health"
```

## Time Boundaries

- `paths {}` and `build {}` are build-time concepts.
- Page-level Go imports used by `build {}` run at build time with the local Go
  toolchain.
- Build-time Go function calls must use an explicit imported alias such as
  `interop.FeaturedCopyForBuild()`. Same-package Go functions are not resolved
  by bare name in the current slice; importing the package keeps build-time
  execution dependencies visible and avoids implicit same-package execution.
- `load {}` is request-time behavior and must not make SPA pages implicitly SSR.
- `act` and `api` endpoint declarations describe request handlers that should
  work without full-page SSR. Normal Go handlers own behavior and return
  `runtime/response.Response`.
- `view {}` renders markup for spa, action, partial, and SSR output.

## View Styles

Declare CSS close to markup with a direct nested `style {}` block:

```gwdk
view {
  <main class="hero">
    <h1>GOWDK</h1>
  </main>

  style {
    .hero {
      color: red;
    }
  }
}
```

The nested style block is not rendered as markup. GOWDK emits it through the
generated CSS asset pipeline:

- Page styles are appended to the page CSS asset.
- Component styles are emitted as scoped component CSS.
- Layout styles are linked only by pages that declare the layout.
